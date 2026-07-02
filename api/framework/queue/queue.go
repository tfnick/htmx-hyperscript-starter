package queue

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"
	"github.com/tfnick/go-svelte-starter/api/db"
	"github.com/tfnick/go-svelte-starter/api/framework/logging"
	"maragu.dev/goqite"
	"maragu.dev/goqite/jobs"
)

const (
	QueueScheduledTasks        = "scheduled-tasks"
	QueueDomainEvents          = "domain-events"
	QueueIntegrationWebhooks   = "integration-webhooks"
	QueueExternalNotifications = "external-notifications"
	QueueHeavyTasks            = "heavy-tasks"

	DefaultMaxReceive       = 3
	defaultGoqiteMaxReceive = DefaultMaxReceive
	goqiteTimeFormat        = "2006-01-02T15:04:05.000Z07:00"
)

var (
	ErrMessageNotFound     = errors.New("queue message not found")
	ErrMessageNotExhausted = errors.New("queue message is not exhausted")
)

type Manager struct {
	queues map[string]*goqite.Queue
	wakes  map[string]*queueWake
	sqlDB  *sql.DB
	driver db.Driver
	log    *zerolog.Logger
}

type JobFunc func(context.Context, []byte) error

type JSONHandler[T any] func(context.Context, T) error
type LimitProvider func(context.Context) (int, error)

type Runner struct {
	queue        *goqite.Queue
	queueName    string
	wake         <-chan struct{}
	sqlDB        *sql.DB
	driver       db.Driver
	maxReceive   int
	limiter      runnerLimitController
	pollInterval time.Duration
	extend       time.Duration
	log          *zerolog.Logger
	handlers     map[string]JobFunc
}

type JSONRunner struct {
	queue        *goqite.Queue
	queueName    string
	wake         <-chan struct{}
	sqlDB        *sql.DB
	driver       db.Driver
	maxReceive   int
	limiter      runnerLimitController
	pollInterval time.Duration
	extend       time.Duration
	log          *zerolog.Logger
	handler      JobFunc
}

type SendOptions struct {
	Queue    string
	Body     []byte
	Delay    time.Duration
	Priority int
}

type jobMessage struct {
	Name    string
	Message []byte
}

type queueWake struct {
	ch chan struct{}
}

type runnerLimitController struct {
	current         int
	provider        LimitProvider
	refreshInterval time.Duration
	nextRefresh     time.Time
}

func newQueueWake() *queueWake {
	return &queueWake{ch: make(chan struct{}, 1)}
}

func (w *queueWake) notify() {
	select {
	case w.ch <- struct{}{}:
	default:
	}
}

func NewManager() (*Manager, error) {
	sqlDB, err := db.SQLDBFor("app")
	if err != nil {
		return nil, fmt.Errorf("get app sql db for queue manager: %w", err)
	}
	driver, err := db.DriverFor("app")
	if err != nil {
		return nil, fmt.Errorf("get app db driver for queue manager: %w", err)
	}
	flavor, err := goqiteFlavor(driver)
	if err != nil {
		return nil, err
	}

	logger := logging.For("queue")
	wakes := newQueueWakes()
	return &Manager{
		queues: map[string]*goqite.Queue{
			QueueScheduledTasks: goqite.New(goqite.NewOpts{
				DB:        sqlDB,
				Name:      QueueScheduledTasks,
				SQLFlavor: flavor,
			}),
			QueueDomainEvents: goqite.New(goqite.NewOpts{
				DB:        sqlDB,
				Name:      QueueDomainEvents,
				SQLFlavor: flavor,
			}),
			QueueIntegrationWebhooks: goqite.New(goqite.NewOpts{
				DB:        sqlDB,
				Name:      QueueIntegrationWebhooks,
				SQLFlavor: flavor,
			}),
			QueueExternalNotifications: goqite.New(goqite.NewOpts{
				DB:        sqlDB,
				Name:      QueueExternalNotifications,
				SQLFlavor: flavor,
			}),
			QueueHeavyTasks: goqite.New(goqite.NewOpts{
				DB:        sqlDB,
				Name:      QueueHeavyTasks,
				SQLFlavor: flavor,
			}),
		},
		wakes:  wakes,
		sqlDB:  sqlDB,
		driver: driver,
		log:    &logger,
	}, nil
}

func newQueueWakes() map[string]*queueWake {
	return map[string]*queueWake{
		QueueScheduledTasks:        newQueueWake(),
		QueueDomainEvents:          newQueueWake(),
		QueueIntegrationWebhooks:   newQueueWake(),
		QueueExternalNotifications: newQueueWake(),
		QueueHeavyTasks:            newQueueWake(),
	}
}

func goqiteFlavor(driver db.Driver) (goqite.SQLFlavor, error) {
	switch driver {
	case db.DriverSQLite:
		return goqite.SQLFlavorSQLite, nil
	case db.DriverPostgres:
		return goqite.SQLFlavorPostgreSQL, nil
	default:
		return 0, fmt.Errorf("unsupported queue database driver: %s", driver)
	}
}

func newRunnerLimitController(limit int) runnerLimitController {
	if limit <= 0 {
		limit = 1
	}
	return runnerLimitController{
		current: limit,
	}
}

func (c *runnerLimitController) SetProvider(provider LimitProvider, refreshInterval time.Duration) {
	c.provider = provider
	if refreshInterval <= 0 {
		refreshInterval = time.Second
	}
	c.refreshInterval = refreshInterval
	c.nextRefresh = time.Time{}
}

func (c *runnerLimitController) Current(ctx context.Context, log *zerolog.Logger, queueName string) int {
	if c.current <= 0 {
		c.current = 1
	}
	if c.provider == nil {
		return c.current
	}

	now := time.Now()
	if !c.nextRefresh.IsZero() && now.Before(c.nextRefresh) {
		return c.current
	}
	c.nextRefresh = now.Add(c.refreshInterval)

	limit, err := c.provider(ctx)
	if err != nil {
		logQueueLimit(log, queueName, err, c.current, "failed to refresh queue runner limit")
		return c.current
	}
	if limit <= 0 {
		logQueueLimit(log, queueName, nil, c.current, "queue runner limit provider returned invalid limit")
		return c.current
	}
	if limit != c.current && log != nil {
		log.Info().Str("queue", queueName).Int("old_limit", c.current).Int("limit", limit).Msg("queue runner limit refreshed")
	}
	c.current = limit
	return c.current
}

func (c *runnerLimitController) NextRefreshDelay() (time.Duration, bool) {
	if c.provider == nil {
		return 0, false
	}
	if c.nextRefresh.IsZero() {
		return 0, true
	}
	delay := time.Until(c.nextRefresh)
	if delay < 0 {
		delay = 0
	}
	return delay, true
}

func logQueueLimit(log *zerolog.Logger, queueName string, err error, current int, message string) {
	if log == nil {
		return
	}
	event := log.Info().Str("queue", queueName).Int("current_limit", current)
	if err != nil {
		event = event.Err(err)
	}
	event.Msg(message)
}

func (m *Manager) Send(ctx context.Context, opts SendOptions) (string, error) {
	q, err := m.queue(opts.Queue)
	if err != nil {
		return "", err
	}

	message := goqite.Message{
		Body:     opts.Body,
		Delay:    opts.Delay,
		Priority: opts.Priority,
	}

	if tx, ok := db.SQLTxFor(ctx, "app"); ok {
		id, err := q.SendAndGetIDTx(ctx, tx, message)
		if err == nil {
			m.notifyAfterCommit(ctx, opts.Queue)
		}
		return string(id), err
	}

	id, err := q.SendAndGetID(ctx, message)
	if err == nil {
		m.notify(opts.Queue)
	}
	return string(id), err
}

func (m *Manager) SendJSON(ctx context.Context, opts SendOptions, payload any) (string, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	opts.Body = body
	return m.Send(ctx, opts)
}

func (m *Manager) CreateJSONJob(ctx context.Context, queueName string, name string, payload any, delay time.Duration, priority int) (string, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return m.CreateJob(ctx, queueName, name, body, delay, priority)
}

func (m *Manager) NewRunner(queueName string, limit int, pollInterval time.Duration) (*Runner, error) {
	q, err := m.queue(queueName)
	if err != nil {
		return nil, err
	}
	wake, err := m.wake(queueName)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 1
	}
	if pollInterval <= 0 {
		pollInterval = 100 * time.Millisecond
	}

	return &Runner{
		queue:        q,
		queueName:    queueName,
		wake:         wake.ch,
		sqlDB:        m.sqlDB,
		driver:       m.driver,
		maxReceive:   defaultGoqiteMaxReceive,
		limiter:      newRunnerLimitController(limit),
		pollInterval: pollInterval,
		extend:       5 * time.Second,
		log:          m.log,
		handlers:     map[string]JobFunc{},
	}, nil
}

func (m *Manager) NewJSONRunner(queueName string, limit int, pollInterval time.Duration) (*JSONRunner, error) {
	q, err := m.queue(queueName)
	if err != nil {
		return nil, err
	}
	wake, err := m.wake(queueName)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 1
	}
	if pollInterval <= 0 {
		pollInterval = 100 * time.Millisecond
	}

	return &JSONRunner{
		queue:        q,
		queueName:    queueName,
		wake:         wake.ch,
		sqlDB:        m.sqlDB,
		driver:       m.driver,
		maxReceive:   defaultGoqiteMaxReceive,
		limiter:      newRunnerLimitController(limit),
		pollInterval: pollInterval,
		extend:       5 * time.Second,
		log:          m.log,
	}, nil
}

func (r *Runner) SetLimitProvider(provider LimitProvider, refreshInterval time.Duration) {
	r.limiter.SetProvider(provider, refreshInterval)
}

func (r *JSONRunner) SetLimitProvider(provider LimitProvider, refreshInterval time.Duration) {
	r.limiter.SetProvider(provider, refreshInterval)
}

func (r *Runner) Register(name string, job JobFunc) {
	if _, ok := r.handlers[name]; ok {
		panic(fmt.Sprintf(`job "%v" already registered`, name))
	}
	r.handlers[name] = job
}

func RegisterJobJSON[T any](runner *Runner, name string, handler JSONHandler[T]) {
	runner.Register(name, HandleJSON(handler))
}

func (r *Runner) Start(ctx context.Context) {
	if len(r.handlers) == 0 {
		r.log.Info().Msg("job queue runner has no handlers")
		return
	}

	r.log.Info().Int("limit", r.limiter.Current(ctx, r.log, r.queueName)).Msg("starting job queue runner")
	var wg sync.WaitGroup
	var running atomic.Int32
	done := make(chan struct{}, 1)

	for {
		if !waitForRunnerCapacity(ctx, r.log, r.queueName, &r.limiter, &running, done) {
			r.log.Info().Msg("stopping job queue runner")
			wg.Wait()
			r.log.Info().Msg("stopped job queue runner")
			return
		}

		message, err := r.queue.Receive(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				continue
			}
			r.log.Info().Err(err).Str("queue", r.queueName).Msg("error receiving job queue message")
			time.Sleep(time.Second)
			continue
		}
		if message == nil {
			if !r.waitForWakeOrPoll(ctx) {
				continue
			}
			continue
		}

		running.Add(1)
		wg.Add(1)
		go func(message *goqite.Message) {
			defer wg.Done()
			defer func() {
				running.Add(-1)
				signalRunnerCapacity(done)
			}()
			defer func() {
				if rec := recover(); rec != nil {
					r.log.Info().Str("id", string(message.ID)).Interface("error", rec).Msg("recovered from panic in job queue message")
				}
			}()

			jm, err := decodeJobMessage(message.Body)
			if err != nil {
				r.log.Info().Str("id", string(message.ID)).Err(err).Msg("error decoding job queue message")
				return
			}

			handler, ok := r.handlers[jm.Name]
			if !ok {
				r.log.Info().Str("id", string(message.ID)).Str("name", jm.Name).Msg("job queue message has unregistered job")
				return
			}

			jobCtx, cancel := context.WithCancel(ctx)
			defer cancel()

			go r.extendWhileRunning(jobCtx, message.ID)

			r.log.Info().Str("id", string(message.ID)).Str("name", jm.Name).Msg("running job queue message")
			before := time.Now()
			if err := handler(jobCtx, jm.Message); err != nil {
				r.log.Info().Str("id", string(message.ID)).Str("name", jm.Name).Err(err).Msg("job queue message failed")
				return
			}
			r.log.Info().Str("id", string(message.ID)).Str("name", jm.Name).Dur("duration", time.Since(before)).Msg("ran job queue message")

			deleteCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
			defer cancel()
			if err := r.queue.Delete(deleteCtx, message.ID); err != nil {
				r.log.Info().Str("id", string(message.ID)).Str("name", jm.Name).Err(err).Msg("error deleting job queue message")
			}
		}(message)
	}
}

func (r *JSONRunner) Register(job JobFunc) {
	r.handler = job
}

func RegisterJSON[T any](runner *JSONRunner, handler JSONHandler[T]) {
	runner.Register(HandleJSON(handler))
}

func HandleJSON[T any](handler JSONHandler[T]) JobFunc {
	return func(ctx context.Context, body []byte) error {
		var payload T
		if err := json.Unmarshal(body, &payload); err != nil {
			return fmt.Errorf("decode queue JSON payload: %w", err)
		}
		return handler(ctx, payload)
	}
}

func (r *JSONRunner) Start(ctx context.Context) {
	if r.handler == nil {
		r.log.Info().Msg("json queue runner has no handler")
		return
	}

	r.log.Info().Int("limit", r.limiter.Current(ctx, r.log, r.queueName)).Msg("starting json queue runner")
	var wg sync.WaitGroup
	var running atomic.Int32
	done := make(chan struct{}, 1)

	for {
		if !waitForRunnerCapacity(ctx, r.log, r.queueName, &r.limiter, &running, done) {
			r.log.Info().Msg("stopping json queue runner")
			wg.Wait()
			r.log.Info().Msg("stopped json queue runner")
			return
		}

		message, err := r.queue.Receive(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				continue
			}
			r.log.Info().Err(err).Msg("error receiving json queue message")
			time.Sleep(time.Second)
			continue
		}
		if message == nil {
			if !r.waitForWakeOrPoll(ctx) {
				continue
			}
			continue
		}

		running.Add(1)
		wg.Add(1)
		go func(message *goqite.Message) {
			defer wg.Done()
			defer func() {
				running.Add(-1)
				signalRunnerCapacity(done)
			}()
			defer func() {
				if rec := recover(); rec != nil {
					r.log.Info().Str("id", string(message.ID)).Interface("error", rec).Msg("recovered from panic in json queue message")
				}
			}()

			jobCtx, cancel := context.WithCancel(ctx)
			defer cancel()

			go r.extendWhileRunning(jobCtx, message.ID)

			r.log.Info().Str("id", string(message.ID)).Msg("running json queue message")
			before := time.Now()
			if err := r.handler(jobCtx, message.Body); err != nil {
				r.log.Info().Str("id", string(message.ID)).Err(err).Msg("json queue message failed")
				return
			}
			r.log.Info().Str("id", string(message.ID)).Dur("duration", time.Since(before)).Msg("ran json queue message")

			deleteCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
			defer cancel()
			if err := r.queue.Delete(deleteCtx, message.ID); err != nil {
				r.log.Info().Str("id", string(message.ID)).Err(err).Msg("error deleting json queue message")
			}
		}(message)
	}
}

func (r *JSONRunner) waitForWakeOrPoll(ctx context.Context) bool {
	return waitForQueueWakeOrPoll(ctx, r.pollInterval, r.wake, r.nextClaimDelay)
}

func (r *JSONRunner) nextClaimDelay(ctx context.Context) (time.Duration, bool) {
	return nextQueueClaimDelay(ctx, r.log, r.queueName, r.nextClaimAt)
}

func (r *JSONRunner) nextClaimAt(ctx context.Context) (time.Time, bool, error) {
	return nextQueueClaimAt(ctx, r.driver, r.sqlDB, r.queueName, r.maxReceive)
}

func (r *JSONRunner) drainWake() {
	drainQueueWake(r.wake)
}

func (r *JSONRunner) extendWhileRunning(ctx context.Context, id goqite.ID) {
	delay := r.extend - r.extend/5
	if delay <= 0 {
		delay = time.Second
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			if err := r.queue.Extend(ctx, id, r.extend); err != nil {
				r.log.Info().Str("id", string(id)).Err(err).Msg("error extending json queue message timeout")
			}
			timer.Reset(delay)
		}
	}
}

func (r *Runner) waitForWakeOrPoll(ctx context.Context) bool {
	return waitForQueueWakeOrPoll(ctx, r.pollInterval, r.wake, r.nextClaimDelay)
}

func (r *Runner) nextClaimDelay(ctx context.Context) (time.Duration, bool) {
	return nextQueueClaimDelay(ctx, r.log, r.queueName, r.nextClaimAt)
}

func (r *Runner) nextClaimAt(ctx context.Context) (time.Time, bool, error) {
	return nextQueueClaimAt(ctx, r.driver, r.sqlDB, r.queueName, r.maxReceive)
}

func (r *Runner) extendWhileRunning(ctx context.Context, id goqite.ID) {
	delay := r.extend - r.extend/5
	if delay <= 0 {
		delay = time.Second
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			if err := r.queue.Extend(ctx, id, r.extend); err != nil {
				r.log.Info().Str("id", string(id)).Err(err).Msg("error extending job queue message timeout")
			}
			timer.Reset(delay)
		}
	}
}

func decodeJobMessage(body []byte) (jobMessage, error) {
	var jm jobMessage
	if err := gob.NewDecoder(bytes.NewReader(body)).Decode(&jm); err != nil {
		return jobMessage{}, err
	}
	if jm.Name == "" {
		return jobMessage{}, errors.New("job name is empty")
	}
	return jm, nil
}

func waitForQueueWakeOrPoll(ctx context.Context, pollInterval time.Duration, wake <-chan struct{}, nextClaimDelay func(context.Context) (time.Duration, bool)) bool {
	waitFor := pollInterval
	if delay, ok := nextClaimDelay(ctx); ok && (waitFor <= 0 || delay < waitFor) {
		waitFor = delay
	}
	if waitFor <= 0 {
		return true
	}

	timer := time.NewTimer(waitFor)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-wake:
		drainQueueWake(wake)
		return true
	case <-timer.C:
		return true
	}
}

func waitForRunnerCapacity(ctx context.Context, log *zerolog.Logger, queueName string, limiter *runnerLimitController, running *atomic.Int32, done <-chan struct{}) bool {
	for {
		select {
		case <-ctx.Done():
			return false
		default:
		}

		limit := limiter.Current(ctx, log, queueName)
		if int(running.Load()) < limit {
			return true
		}

		delay, ok := limiter.NextRefreshDelay()
		if ok {
			if delay <= 0 {
				continue
			}
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return false
			case <-done:
				timer.Stop()
			case <-timer.C:
			}
			continue
		}

		select {
		case <-ctx.Done():
			return false
		case <-done:
		}
	}
}

func signalRunnerCapacity(done chan<- struct{}) {
	select {
	case done <- struct{}{}:
	default:
	}
}

func nextQueueClaimDelay(ctx context.Context, log *zerolog.Logger, queueName string, nextClaimAt func(context.Context) (time.Time, bool, error)) (time.Duration, bool) {
	nextAt, ok, err := nextClaimAt(ctx)
	if err != nil {
		log.Info().Err(err).Str("queue", queueName).Msg("error finding next queue claim deadline")
		return 0, false
	}
	if !ok {
		return 0, false
	}
	delay := time.Until(nextAt)
	if delay < 0 {
		delay = 0
	}
	return delay, true
}

func nextQueueClaimAt(ctx context.Context, driver db.Driver, sqlDB *sql.DB, queueName string, maxReceive int) (time.Time, bool, error) {
	switch driver {
	case db.DriverSQLite:
		var value sql.NullString
		err := sqlDB.QueryRowContext(ctx, `SELECT MIN(timeout) FROM goqite WHERE queue = ? AND received < ?`, queueName, maxReceive).Scan(&value)
		if err != nil {
			return time.Time{}, false, err
		}
		if !value.Valid || value.String == "" {
			return time.Time{}, false, nil
		}
		parsed, err := time.Parse(goqiteTimeFormat, value.String)
		if err != nil {
			return time.Time{}, false, err
		}
		return parsed, true, nil
	case db.DriverPostgres:
		var value sql.NullTime
		err := sqlDB.QueryRowContext(ctx, `SELECT MIN(timeout) FROM goqite WHERE queue = $1 AND received < $2`, queueName, maxReceive).Scan(&value)
		if err != nil {
			return time.Time{}, false, err
		}
		if !value.Valid {
			return time.Time{}, false, nil
		}
		return value.Time, true, nil
	default:
		return time.Time{}, false, fmt.Errorf("unsupported queue database driver: %s", driver)
	}
}

func drainQueueWake(wake <-chan struct{}) {
	for {
		select {
		case <-wake:
		default:
			return
		}
	}
}

func (m *Manager) CreateJob(ctx context.Context, queueName string, name string, body []byte, delay time.Duration, priority int) (string, error) {
	q, err := m.queue(queueName)
	if err != nil {
		return "", err
	}

	message := goqite.Message{
		Body:     body,
		Delay:    delay,
		Priority: priority,
	}

	if tx, ok := db.SQLTxFor(ctx, "app"); ok {
		id, err := jobs.CreateTx(ctx, tx, q, name, message)
		if err == nil {
			m.notifyAfterCommit(ctx, queueName)
		}
		return string(id), err
	}

	id, err := jobs.Create(ctx, q, name, message)
	if err == nil {
		m.notify(queueName)
	}
	return string(id), err
}

func (m *Manager) RetryMessageNow(ctx context.Context, queueName string, messageID string) error {
	if _, err := m.queue(queueName); err != nil {
		return err
	}

	now := time.Now().UTC()
	var result sql.Result
	var err error
	switch m.driver {
	case db.DriverSQLite:
		result, err = m.sqlDB.ExecContext(ctx, `
			UPDATE goqite
			SET timeout = ?, received = 0
			WHERE queue = ? AND id = ? AND received >= ?
		`, now.Format(goqiteTimeFormat), queueName, messageID, defaultGoqiteMaxReceive)
	case db.DriverPostgres:
		result, err = m.sqlDB.ExecContext(ctx, `
			UPDATE goqite
			SET timeout = $1, received = 0
			WHERE queue = $2 AND id = $3 AND received >= $4
		`, now, queueName, messageID, defaultGoqiteMaxReceive)
	default:
		return fmt.Errorf("unsupported queue database driver: %s", m.driver)
	}
	if err != nil {
		return err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		exists, err := m.messageExists(ctx, queueName, messageID)
		if err != nil {
			return err
		}
		if !exists {
			return ErrMessageNotFound
		}
		return ErrMessageNotExhausted
	}

	m.notify(queueName)
	return nil
}

func (m *Manager) DeleteMessage(ctx context.Context, queueName string, messageID string) error {
	if _, err := m.queue(queueName); err != nil {
		return err
	}

	var result sql.Result
	var err error
	switch m.driver {
	case db.DriverSQLite:
		result, err = m.sqlDB.ExecContext(ctx, `DELETE FROM goqite WHERE queue = ? AND id = ?`, queueName, messageID)
	case db.DriverPostgres:
		result, err = m.sqlDB.ExecContext(ctx, `DELETE FROM goqite WHERE queue = $1 AND id = $2`, queueName, messageID)
	default:
		return fmt.Errorf("unsupported queue database driver: %s", m.driver)
	}
	if err != nil {
		return err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrMessageNotFound
	}
	return nil
}

func (m *Manager) messageExists(ctx context.Context, queueName string, messageID string) (bool, error) {
	var count int
	var err error
	switch m.driver {
	case db.DriverSQLite:
		err = m.sqlDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM goqite WHERE queue = ? AND id = ?`, queueName, messageID).Scan(&count)
	case db.DriverPostgres:
		err = m.sqlDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM goqite WHERE queue = $1 AND id = $2`, queueName, messageID).Scan(&count)
	default:
		return false, fmt.Errorf("unsupported queue database driver: %s", m.driver)
	}
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (m *Manager) queue(name string) (*goqite.Queue, error) {
	q := m.queues[name]
	if q == nil {
		return nil, fmt.Errorf("queue not registered: %s", name)
	}
	return q, nil
}

func (m *Manager) wake(name string) (*queueWake, error) {
	w := m.wakes[name]
	if w == nil {
		return nil, fmt.Errorf("queue wake not registered: %s", name)
	}
	return w, nil
}

func (m *Manager) notify(queueName string) {
	w, err := m.wake(queueName)
	if err != nil {
		m.log.Info().Err(err).Str("queue", queueName).Msg("skip queue wake")
		return
	}
	w.notify()
}

func (m *Manager) notifyAfterCommit(ctx context.Context, queueName string) {
	if err := db.RegisterAfterCommit(ctx, func(context.Context) {
		m.notify(queueName)
	}); err != nil {
		if errors.Is(err, db.ErrNoActiveAppTx) {
			m.notify(queueName)
			return
		}
		m.log.Info().Err(err).Str("queue", queueName).Msg("failed to register queue wake after commit")
	}
}

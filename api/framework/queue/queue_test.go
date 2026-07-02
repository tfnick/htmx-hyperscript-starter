package queue

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tfnick/go-svelte-starter/api/db"
	fwusecase "github.com/tfnick/go-svelte-starter/api/framework/usecase"
	"maragu.dev/goqite"
)

func TestGoqiteFlavorUsesDatabaseDriver(t *testing.T) {
	tests := []struct {
		name   string
		driver db.Driver
		want   goqite.SQLFlavor
	}{
		{name: "sqlite", driver: db.DriverSQLite, want: goqite.SQLFlavorSQLite},
		{name: "postgres", driver: db.DriverPostgres, want: goqite.SQLFlavorPostgreSQL},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := goqiteFlavor(tt.driver)
			if err != nil {
				t.Fatalf("goqiteFlavor() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("goqiteFlavor() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGoqiteFlavorRejectsUnsupportedDriver(t *testing.T) {
	if _, err := goqiteFlavor(db.Driver("mysql")); err == nil {
		t.Fatalf("expected unsupported driver error")
	}
}

func TestJSONRunnerWakesAfterSendWithoutWaitingForPoll(t *testing.T) {
	manager := setupQueueTestDB(t)
	queueManager, err := NewManager()
	if err != nil {
		t.Fatalf("new queue manager: %v", err)
	}
	_ = manager

	runner, err := queueManager.NewJSONRunner(QueueDomainEvents, 1, time.Hour)
	if err != nil {
		t.Fatalf("new json runner: %v", err)
	}

	type payload struct {
		Hello string `json:"hello"`
	}

	handled := make(chan payload, 1)
	RegisterJSON(runner, func(_ context.Context, message payload) error {
		handled <- message
		return nil
	})

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go runner.Start(ctx)

	if _, err := queueManager.SendJSON(t.Context(), SendOptions{Queue: QueueDomainEvents}, map[string]string{"hello": "world"}); err != nil {
		t.Fatalf("send json: %v", err)
	}

	got := requireValueSignal(t, handled, 500*time.Millisecond, "json runner did not wake after send")
	if got.Hello != "world" {
		t.Fatalf("expected typed json runner payload, got %#v", got)
	}
}

func TestRunnerHandlesCreateJobPayloadAfterWake(t *testing.T) {
	manager := setupQueueTestDB(t)
	queueManager, err := NewManager()
	if err != nil {
		t.Fatalf("new queue manager: %v", err)
	}
	_ = manager

	runner, err := queueManager.NewRunner(QueueScheduledTasks, 1, time.Hour)
	if err != nil {
		t.Fatalf("new job runner: %v", err)
	}

	handled := make(chan []byte, 1)
	runner.Register("scheduled.test", func(_ context.Context, message []byte) error {
		handled <- message
		return nil
	})

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go runner.Start(ctx)

	if _, err := queueManager.CreateJob(t.Context(), QueueScheduledTasks, "scheduled.test", []byte(`{"task":"ok"}`), 0, 0); err != nil {
		t.Fatalf("create job: %v", err)
	}

	got := requireBytesSignal(t, handled, 500*time.Millisecond, "job runner did not wake after create job")
	if string(got) != `{"task":"ok"}` {
		t.Fatalf("expected original job payload, got %q", string(got))
	}
}

func TestRunnerHandlesCreateJSONJobPayloadAfterWake(t *testing.T) {
	setupQueueTestDB(t)
	queueManager, err := NewManager()
	if err != nil {
		t.Fatalf("new queue manager: %v", err)
	}

	runner, err := queueManager.NewRunner(QueueScheduledTasks, 1, time.Hour)
	if err != nil {
		t.Fatalf("new job runner: %v", err)
	}

	type payload struct {
		Task string `json:"task"`
	}

	handled := make(chan payload, 1)
	RegisterJobJSON(runner, "scheduled.typed", func(_ context.Context, message payload) error {
		handled <- message
		return nil
	})

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go runner.Start(ctx)

	if _, err := queueManager.CreateJSONJob(t.Context(), QueueScheduledTasks, "scheduled.typed", payload{Task: "ok"}, 0, 0); err != nil {
		t.Fatalf("create json job: %v", err)
	}

	got := requireValueSignal(t, handled, 500*time.Millisecond, "job runner did not handle typed json job")
	if got.Task != "ok" {
		t.Fatalf("expected typed json job payload, got %#v", got)
	}
}

func TestHandleJSONDecodesTypedPayload(t *testing.T) {
	type payload struct {
		ID    string `json:"id"`
		Count int    `json:"count"`
	}

	var got payload
	handler := HandleJSON(func(_ context.Context, message payload) error {
		got = message
		return nil
	})

	if err := handler(t.Context(), []byte(`{"id":"msg-1","count":2}`)); err != nil {
		t.Fatalf("handle json: %v", err)
	}
	if got.ID != "msg-1" || got.Count != 2 {
		t.Fatalf("unexpected typed payload: %#v", got)
	}
}

func TestHandleJSONReturnsDecodeError(t *testing.T) {
	type payload struct {
		ID string `json:"id"`
	}

	handler := HandleJSON(func(context.Context, payload) error {
		t.Fatal("handler must not run for invalid json")
		return nil
	})

	if err := handler(t.Context(), []byte(`{"id":`)); err == nil {
		t.Fatal("expected invalid json error")
	}
}

func TestRunnerUsesNextClaimDeadlineBeforeLongPollFallback(t *testing.T) {
	setupQueueTestDB(t)
	queueManager, err := NewManager()
	if err != nil {
		t.Fatalf("new queue manager: %v", err)
	}

	runner, err := queueManager.NewRunner(QueueScheduledTasks, 1, time.Hour)
	if err != nil {
		t.Fatalf("new job runner: %v", err)
	}

	handled := make(chan struct{}, 1)
	runner.Register("scheduled.delayed", func(context.Context, []byte) error {
		handled <- struct{}{}
		return nil
	})

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go runner.Start(ctx)

	if _, err := queueManager.CreateJob(t.Context(), QueueScheduledTasks, "scheduled.delayed", []byte(`{}`), 80*time.Millisecond, 0); err != nil {
		t.Fatalf("create delayed job: %v", err)
	}

	requireSignal(t, handled, 600*time.Millisecond, "job runner did not use next claim deadline before long fallback")
}

func TestRunnerKeepsMessageWhenHandlerFails(t *testing.T) {
	manager := setupQueueTestDB(t)
	queueManager, err := NewManager()
	if err != nil {
		t.Fatalf("new queue manager: %v", err)
	}
	appDB, err := manager.GetEngine("app")
	if err != nil {
		t.Fatalf("get app engine: %v", err)
	}

	runner, err := queueManager.NewRunner(QueueScheduledTasks, 1, time.Hour)
	if err != nil {
		t.Fatalf("new job runner: %v", err)
	}

	handled := make(chan struct{}, 1)
	runner.Register("scheduled.fail", func(context.Context, []byte) error {
		handled <- struct{}{}
		return errStopQueueTest
	})

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go runner.Start(ctx)

	if _, err := queueManager.CreateJob(t.Context(), QueueScheduledTasks, "scheduled.fail", []byte(`{}`), 0, 0); err != nil {
		t.Fatalf("create job: %v", err)
	}
	requireSignal(t, handled, 500*time.Millisecond, "job runner did not run failing handler")

	if count := countQueueMessages(t, appDB, QueueScheduledTasks); count != 1 {
		t.Fatalf("expected failed job message to remain retryable, got %d", count)
	}
	if received := countQueueMessagesReceivedAtLeast(t, appDB, QueueScheduledTasks, 1); received != 1 {
		t.Fatalf("expected failed job message to be received once, got %d", received)
	}
}

func TestRunnerKeepsMessageForUnknownJob(t *testing.T) {
	manager := setupQueueTestDB(t)
	queueManager, err := NewManager()
	if err != nil {
		t.Fatalf("new queue manager: %v", err)
	}
	appDB, err := manager.GetEngine("app")
	if err != nil {
		t.Fatalf("get app engine: %v", err)
	}

	runner, err := queueManager.NewRunner(QueueScheduledTasks, 1, time.Hour)
	if err != nil {
		t.Fatalf("new job runner: %v", err)
	}
	runner.Register("scheduled.known", func(context.Context, []byte) error {
		t.Fatal("known handler must not run for unknown job")
		return nil
	})

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go runner.Start(ctx)

	if _, err := queueManager.CreateJob(t.Context(), QueueScheduledTasks, "scheduled.unknown", []byte(`{}`), 0, 0); err != nil {
		t.Fatalf("create job: %v", err)
	}

	requireEventually(t, 500*time.Millisecond, "unknown job message should be claimed and remain retryable", func() bool {
		return countQueueMessagesReceivedAtLeast(t, appDB, QueueScheduledTasks, 1) == 1
	})
	if count := countQueueMessages(t, appDB, QueueScheduledTasks); count != 1 {
		t.Fatalf("expected unknown job message to remain retryable, got %d", count)
	}
}

func TestRunnerKeepsMessageForInvalidJobBody(t *testing.T) {
	manager := setupQueueTestDB(t)
	queueManager, err := NewManager()
	if err != nil {
		t.Fatalf("new queue manager: %v", err)
	}
	appDB, err := manager.GetEngine("app")
	if err != nil {
		t.Fatalf("get app engine: %v", err)
	}

	runner, err := queueManager.NewRunner(QueueScheduledTasks, 1, time.Hour)
	if err != nil {
		t.Fatalf("new job runner: %v", err)
	}
	runner.Register("scheduled.test", func(context.Context, []byte) error {
		t.Fatal("handler must not run for invalid gob body")
		return nil
	})

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go runner.Start(ctx)

	if _, err := queueManager.Send(t.Context(), SendOptions{Queue: QueueScheduledTasks, Body: []byte("not-gob")}); err != nil {
		t.Fatalf("send invalid body: %v", err)
	}

	requireEventually(t, 500*time.Millisecond, "invalid job body should be claimed and remain retryable", func() bool {
		return countQueueMessagesReceivedAtLeast(t, appDB, QueueScheduledTasks, 1) == 1
	})
	if count := countQueueMessages(t, appDB, QueueScheduledTasks); count != 1 {
		t.Fatalf("expected invalid job body to remain retryable, got %d", count)
	}
}

func TestQueueWakeRunsAfterTransactionCommit(t *testing.T) {
	setupQueueTestDB(t)
	queueManager, err := NewManager()
	if err != nil {
		t.Fatalf("new queue manager: %v", err)
	}

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceSystem)
	if err := fwusecase.WithAppTx(ctx, func(txCtx fwusecase.Context) error {
		if _, err := queueManager.SendJSON(txCtx.Std(), SendOptions{Queue: QueueDomainEvents}, map[string]string{"tx": "commit"}); err != nil {
			return err
		}
		if queueWakePending(queueManager, QueueDomainEvents) {
			t.Fatalf("queue wake fired before transaction commit")
		}
		return nil
	}); err != nil {
		t.Fatalf("with app tx: %v", err)
	}
	if !queueWakePending(queueManager, QueueDomainEvents) {
		t.Fatalf("expected queue wake after transaction commit")
	}
}

func TestQueueWakeDiscardedOnTransactionRollback(t *testing.T) {
	setupQueueTestDB(t)
	queueManager, err := NewManager()
	if err != nil {
		t.Fatalf("new queue manager: %v", err)
	}

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceSystem)
	err = fwusecase.WithAppTx(ctx, func(txCtx fwusecase.Context) error {
		if _, err := queueManager.SendJSON(txCtx.Std(), SendOptions{Queue: QueueDomainEvents}, map[string]string{"tx": "rollback"}); err != nil {
			return err
		}
		return errStopQueueTest
	})
	if err != errStopQueueTest {
		t.Fatalf("expected rollback error, got %v", err)
	}
	if queueWakePending(queueManager, QueueDomainEvents) {
		t.Fatalf("queue wake must not fire after transaction rollback")
	}
}

func TestJSONRunnerDrainsAfterSingleWake(t *testing.T) {
	setupQueueTestDB(t)
	queueManager, err := NewManager()
	if err != nil {
		t.Fatalf("new queue manager: %v", err)
	}

	runner, err := queueManager.NewJSONRunner(QueueDomainEvents, 2, time.Hour)
	if err != nil {
		t.Fatalf("new json runner: %v", err)
	}

	handled := make(chan struct{}, 2)
	var running atomic.Int32
	release := make(chan struct{})
	runner.Register(func(context.Context, []byte) error {
		if running.Add(1) == 2 {
			handled <- struct{}{}
		}
		<-release
		handled <- struct{}{}
		return nil
	})

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go runner.Start(ctx)

	if _, err := queueManager.SendJSON(t.Context(), SendOptions{Queue: QueueDomainEvents}, map[string]string{"message": "one"}); err != nil {
		t.Fatalf("send first message: %v", err)
	}
	if _, err := queueManager.SendJSON(t.Context(), SendOptions{Queue: QueueDomainEvents}, map[string]string{"message": "two"}); err != nil {
		t.Fatalf("send second message: %v", err)
	}

	requireSignal(t, handled, 500*time.Millisecond, "json runner did not drain a second message after wake")
	close(release)
	requireSignal(t, handled, 500*time.Millisecond, "first handler did not finish")
	requireSignal(t, handled, 500*time.Millisecond, "second handler did not finish")
}

func TestJSONRunnerRefreshesLimitProviderToIncreaseConcurrency(t *testing.T) {
	setupQueueTestDB(t)
	queueManager, err := NewManager()
	if err != nil {
		t.Fatalf("new queue manager: %v", err)
	}

	runner, err := queueManager.NewJSONRunner(QueueHeavyTasks, 1, time.Hour)
	if err != nil {
		t.Fatalf("new json runner: %v", err)
	}
	var limit atomic.Int32
	limit.Store(1)
	runner.SetLimitProvider(func(context.Context) (int, error) {
		return int(limit.Load()), nil
	}, 10*time.Millisecond)

	started := make(chan struct{}, 2)
	release := make(chan struct{})
	runner.Register(func(context.Context, []byte) error {
		started <- struct{}{}
		<-release
		return nil
	})

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go runner.Start(ctx)

	if _, err := queueManager.SendJSON(t.Context(), SendOptions{Queue: QueueHeavyTasks}, map[string]string{"message": "one"}); err != nil {
		t.Fatalf("send first message: %v", err)
	}
	if _, err := queueManager.SendJSON(t.Context(), SendOptions{Queue: QueueHeavyTasks}, map[string]string{"message": "two"}); err != nil {
		t.Fatalf("send second message: %v", err)
	}

	requireSignal(t, started, 500*time.Millisecond, "first message did not start")
	requireNoSignal(t, started, 100*time.Millisecond, "second message started before dynamic limit increased")

	limit.Store(2)
	requireSignal(t, started, 500*time.Millisecond, "second message did not start after dynamic limit increased")
	close(release)
}

func TestJSONRunnerRefreshesLimitProviderToDecreaseConcurrency(t *testing.T) {
	setupQueueTestDB(t)
	queueManager, err := NewManager()
	if err != nil {
		t.Fatalf("new queue manager: %v", err)
	}

	runner, err := queueManager.NewJSONRunner(QueueHeavyTasks, 2, time.Hour)
	if err != nil {
		t.Fatalf("new json runner: %v", err)
	}
	var limit atomic.Int32
	limit.Store(2)
	runner.SetLimitProvider(func(context.Context) (int, error) {
		return int(limit.Load()), nil
	}, 10*time.Millisecond)

	started := make(chan struct{}, 2)
	release := make(chan struct{})
	runner.Register(func(context.Context, []byte) error {
		started <- struct{}{}
		<-release
		return nil
	})

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go runner.Start(ctx)

	if _, err := queueManager.SendJSON(t.Context(), SendOptions{Queue: QueueHeavyTasks}, map[string]string{"message": "one"}); err != nil {
		t.Fatalf("send first message: %v", err)
	}
	requireSignal(t, started, 500*time.Millisecond, "first message did not start")

	limit.Store(1)
	time.Sleep(20 * time.Millisecond)

	if _, err := queueManager.SendJSON(t.Context(), SendOptions{Queue: QueueHeavyTasks}, map[string]string{"message": "two"}); err != nil {
		t.Fatalf("send second message: %v", err)
	}
	requireNoSignal(t, started, 100*time.Millisecond, "second message started before running count dropped below decreased limit")

	close(release)
	requireSignal(t, started, 500*time.Millisecond, "second message did not start after running count dropped below decreased limit")
}

func TestRunnerLimitControllerRetainsLastLimitOnProviderFailure(t *testing.T) {
	limiter := newRunnerLimitController(2)
	limiter.SetProvider(func(context.Context) (int, error) {
		return 0, errors.New("load limit")
	}, time.Millisecond)
	if got := limiter.Current(t.Context(), nil, QueueHeavyTasks); got != 2 {
		t.Fatalf("expected failed provider to keep last limit 2, got %d", got)
	}

	limiter.SetProvider(func(context.Context) (int, error) {
		return 0, nil
	}, time.Millisecond)
	if got := limiter.Current(t.Context(), nil, QueueHeavyTasks); got != 2 {
		t.Fatalf("expected invalid provider limit to keep last limit 2, got %d", got)
	}

	limiter.SetProvider(func(context.Context) (int, error) {
		return 3, nil
	}, time.Millisecond)
	if got := limiter.Current(t.Context(), nil, QueueHeavyTasks); got != 3 {
		t.Fatalf("expected valid provider limit 3, got %d", got)
	}
}

func TestJSONRunnerUsesNextClaimDeadlineBeforeLongPollFallback(t *testing.T) {
	setupQueueTestDB(t)
	queueManager, err := NewManager()
	if err != nil {
		t.Fatalf("new queue manager: %v", err)
	}

	runner, err := queueManager.NewJSONRunner(QueueDomainEvents, 1, time.Hour)
	if err != nil {
		t.Fatalf("new json runner: %v", err)
	}

	handled := make(chan struct{}, 1)
	runner.Register(func(context.Context, []byte) error {
		handled <- struct{}{}
		return nil
	})

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go runner.Start(ctx)

	if _, err := queueManager.SendJSON(t.Context(), SendOptions{
		Queue: QueueDomainEvents,
		Delay: 80 * time.Millisecond,
	}, map[string]string{"message": "delayed"}); err != nil {
		t.Fatalf("send delayed message: %v", err)
	}

	requireSignal(t, handled, 600*time.Millisecond, "json runner did not use next claim deadline before long fallback")
}

func TestManagerRetryMessageNowMakesExhaustedMessageClaimable(t *testing.T) {
	manager := setupQueueTestDB(t)
	queueManager, err := NewManager()
	if err != nil {
		t.Fatalf("new queue manager: %v", err)
	}
	appDB, err := manager.GetEngine("app")
	if err != nil {
		t.Fatalf("get app engine: %v", err)
	}

	now := time.Now().UTC()
	seedQueueTestMessage(t, appDB, "retry-dead-message", QueueHeavyTasks, now.Add(-5*time.Minute), now.Add(-1*time.Minute), DefaultMaxReceive)

	if err := queueManager.RetryMessageNow(t.Context(), QueueHeavyTasks, "retry-dead-message"); err != nil {
		t.Fatalf("retry message now: %v", err)
	}
	if !queueWakePending(queueManager, QueueHeavyTasks) {
		t.Fatalf("expected retry-now to wake queue")
	}

	q, err := queueManager.queue(QueueHeavyTasks)
	if err != nil {
		t.Fatalf("get queue: %v", err)
	}
	message, err := q.Receive(t.Context())
	if err != nil {
		t.Fatalf("receive retried message: %v", err)
	}
	if message == nil || string(message.ID) != "retry-dead-message" {
		t.Fatalf("expected retried message to be claimable, got %#v", message)
	}
}

func TestManagerRetryMessageNowRejectsNonExhaustedMessage(t *testing.T) {
	manager := setupQueueTestDB(t)
	queueManager, err := NewManager()
	if err != nil {
		t.Fatalf("new queue manager: %v", err)
	}
	appDB, err := manager.GetEngine("app")
	if err != nil {
		t.Fatalf("get app engine: %v", err)
	}

	now := time.Now().UTC()
	seedQueueTestMessage(t, appDB, "retry-ready-message", QueueHeavyTasks, now.Add(-5*time.Minute), now.Add(-1*time.Minute), 0)

	if err := queueManager.RetryMessageNow(t.Context(), QueueHeavyTasks, "retry-ready-message"); !errors.Is(err, ErrMessageNotExhausted) {
		t.Fatalf("expected ErrMessageNotExhausted, got %v", err)
	}
	if queueWakePending(queueManager, QueueHeavyTasks) {
		t.Fatalf("retry-now should not wake queue when message is not exhausted")
	}
}

func TestManagerDeleteMessageRemovesQueueRow(t *testing.T) {
	manager := setupQueueTestDB(t)
	queueManager, err := NewManager()
	if err != nil {
		t.Fatalf("new queue manager: %v", err)
	}
	appDB, err := manager.GetEngine("app")
	if err != nil {
		t.Fatalf("get app engine: %v", err)
	}

	now := time.Now().UTC()
	seedQueueTestMessage(t, appDB, "delete-message", QueueHeavyTasks, now.Add(-5*time.Minute), now.Add(-1*time.Minute), DefaultMaxReceive)

	if err := queueManager.DeleteMessage(t.Context(), QueueHeavyTasks, "delete-message"); err != nil {
		t.Fatalf("delete message: %v", err)
	}
	if count := countQueueMessages(t, appDB, QueueHeavyTasks); count != 0 {
		t.Fatalf("expected deleted queue message, got %d", count)
	}
	if err := queueManager.DeleteMessage(t.Context(), QueueHeavyTasks, "delete-message"); !errors.Is(err, ErrMessageNotFound) {
		t.Fatalf("expected ErrMessageNotFound for missing message, got %v", err)
	}
}

var errStopQueueTest = errors.New("stop queue test")

func setupQueueTestDB(t *testing.T) *db.DBManager {
	t.Helper()

	previous := db.DefaultManager
	manager := db.NewDBManager()
	db.DefaultManager = manager
	dir := t.TempDir()
	t.Cleanup(func() {
		_ = manager.Close()
		db.DefaultManager = previous
	})

	if err := manager.Open("app", "sqlite", filepath.Join(dir, "app.db")); err != nil {
		t.Fatalf("open app db: %v", err)
	}
	if err := manager.AutoMigrate("app"); err != nil {
		t.Fatalf("migrate app db: %v", err)
	}
	return manager
}

func queueWakePending(manager *Manager, queueName string) bool {
	wake, err := manager.wake(queueName)
	if err != nil {
		return false
	}
	select {
	case <-wake.ch:
		wake.notify()
		return true
	default:
		return false
	}
}

func requireSignal(t *testing.T, ch <-chan struct{}, timeout time.Duration, message string) {
	t.Helper()

	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-ch:
	case <-timer.C:
		t.Fatal(message)
	}
}

func requireNoSignal(t *testing.T, ch <-chan struct{}, timeout time.Duration, message string) {
	t.Helper()

	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-ch:
		t.Fatal(message)
	case <-timer.C:
	}
}

func requireBytesSignal(t *testing.T, ch <-chan []byte, timeout time.Duration, message string) []byte {
	t.Helper()

	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case value := <-ch:
		return value
	case <-timer.C:
		t.Fatal(message)
		return nil
	}
}

func requireValueSignal[T any](t *testing.T, ch <-chan T, timeout time.Duration, message string) T {
	t.Helper()

	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case value := <-ch:
		return value
	case <-timer.C:
		t.Fatal(message)
		var zero T
		return zero
	}
}

func requireEventually(t *testing.T, timeout time.Duration, message string, condition func() bool) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal(message)
}

func countQueueMessages(t *testing.T, appDB interface {
	GetP(any, string, ...any) error
}, queueName string) int {
	t.Helper()

	var count int
	if err := appDB.GetP(&count, `SELECT COUNT(*) FROM goqite WHERE queue = ?`, queueName); err != nil {
		t.Fatalf("count queue messages: %v", err)
	}
	return count
}

func countQueueMessagesReceivedAtLeast(t *testing.T, appDB interface {
	GetP(any, string, ...any) error
}, queueName string, received int) int {
	t.Helper()

	var count int
	if err := appDB.GetP(&count, `SELECT COUNT(*) FROM goqite WHERE queue = ? AND received >= ?`, queueName, received); err != nil {
		t.Fatalf("count received queue messages: %v", err)
	}
	return count
}

func seedQueueTestMessage(t *testing.T, appDB interface {
	ExecP(string, ...any) (sql.Result, error)
}, id string, queueName string, created time.Time, timeoutAt time.Time, received int) {
	t.Helper()

	if _, err := appDB.ExecP(`
		INSERT INTO goqite (id, queue, body, created, updated, timeout, received, priority)
		VALUES (?, ?, CAST(? AS BLOB), ?, ?, ?, ?, 0)
	`, id, queueName, `{"id":"`+id+`"}`, formatQueueTestTime(created), formatQueueTestTime(created), formatQueueTestTime(timeoutAt), received); err != nil {
		t.Fatalf("insert queue message %s: %v", id, err)
	}
}

func formatQueueTestTime(value time.Time) string {
	return value.UTC().Format(goqiteTimeFormat)
}

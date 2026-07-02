package events_test

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/tfnick/go-svelte-starter/api/db"
	"github.com/tfnick/go-svelte-starter/api/framework/events"
	"github.com/tfnick/go-svelte-starter/api/framework/queue"
	fwusecase "github.com/tfnick/go-svelte-starter/api/framework/usecase"
	usecaseevents "github.com/tfnick/go-svelte-starter/api/usecase/events"
)

const testTopic = "order.created"

var errHandlerFailed = errors.New("handler failed")

type handlerFunc func(context.Context, events.Event) error

func (fn handlerFunc) Handle(ctx context.Context, event events.Event) error {
	return fn(ctx, event)
}

type fakeStore struct {
	events     map[string]events.Event
	deliveries []fakeDelivery
	status     map[string]string
}

type fakeDelivery struct {
	eventID    string
	subscriber string
	messageID  string
}

type fakeQueueSender struct {
	messages []events.Message
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		events: make(map[string]events.Event),
		status: make(map[string]string),
	}
}

func (s *fakeStore) InsertEvent(_ context.Context, event events.Event) (string, error) {
	s.events[event.ID] = event
	return event.ID, nil
}

func (s *fakeStore) InsertDelivery(_ context.Context, eventID string, subscriber string, messageID string) error {
	s.deliveries = append(s.deliveries, fakeDelivery{eventID: eventID, subscriber: subscriber, messageID: messageID})
	s.status[eventID+"/"+subscriber] = "queued"
	return nil
}

func (s *fakeStore) LoadEvent(_ context.Context, id string) (events.Event, error) {
	event, ok := s.events[id]
	if !ok {
		return events.Event{}, errors.New("event not found")
	}
	return event, nil
}

func (s *fakeStore) MarkDeliveryRunning(_ context.Context, eventID string, subscriber string) error {
	s.status[eventID+"/"+subscriber] = "running"
	return nil
}

func (s *fakeStore) MarkDeliverySucceeded(_ context.Context, eventID string, subscriber string) error {
	s.status[eventID+"/"+subscriber] = "succeeded"
	return nil
}

func (s *fakeStore) MarkDeliveryFailed(_ context.Context, eventID string, subscriber string, _ string) error {
	s.status[eventID+"/"+subscriber] = "failed"
	return nil
}

func (s *fakeQueueSender) SendJSON(_ context.Context, opts queue.SendOptions, payload any) (string, error) {
	if opts.Queue != queue.QueueDomainEvents {
		return "", errors.New("unexpected queue")
	}
	message := payload.(events.Message)
	s.messages = append(s.messages, message)
	return "msg-" + message.Subscriber, nil
}

func TestRegisterRejectsDuplicateSubscriber(t *testing.T) {
	bus := events.NewBus()
	sub := events.Subscription{
		Topic:      testTopic,
		Subscriber: "sms.order_created",
	}

	if err := bus.Register(sub, handlerFunc(func(context.Context, events.Event) error {
		return nil
	})); err != nil {
		t.Fatalf("register: %v", err)
	}

	err := bus.Register(sub, handlerFunc(func(context.Context, events.Event) error {
		return nil
	}))
	if !errors.Is(err, events.ErrDuplicateSubscription) {
		t.Fatalf("expected duplicate subscription error, got %v", err)
	}
}

func TestPublishWithoutSubscribersReturnsNil(t *testing.T) {
	bus := events.NewBus()
	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceSystem)

	if err := bus.Publish(ctx, testEvent()); err != nil {
		t.Fatalf("publish without subscribers: %v", err)
	}
}

func TestPublishRequiresDurableConfigurationWhenSubscribersExist(t *testing.T) {
	bus := events.NewBus()
	if err := bus.Register(events.Subscription{
		Topic:      testTopic,
		Subscriber: "sms.order_created",
	}, handlerFunc(func(context.Context, events.Event) error {
		return nil
	})); err != nil {
		t.Fatalf("register: %v", err)
	}

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceSystem)
	err := bus.Publish(ctx, testEvent())
	if !errors.Is(err, events.ErrDurableEventsNotReady) {
		t.Fatalf("expected durable configuration error, got %v", err)
	}
}

func TestPublishFansOutToIndependentSubscriberMessages(t *testing.T) {
	bus := configuredBus()
	store := newFakeStore()
	sender := &fakeQueueSender{}
	bus.Configure(store, sender)

	for _, subscriber := range []string{"billing.capture", "email.receipt"} {
		if err := bus.Register(events.Subscription{
			Topic:      testTopic,
			Subscriber: subscriber,
		}, handlerFunc(func(context.Context, events.Event) error {
			return nil
		})); err != nil {
			t.Fatalf("register %s: %v", subscriber, err)
		}
	}

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceSystem)
	if err := bus.Publish(ctx, testEvent()); err != nil {
		t.Fatalf("publish durable: %v", err)
	}

	if len(sender.messages) != 2 {
		t.Fatalf("expected 2 queue messages, got %d", len(sender.messages))
	}
	if len(store.deliveries) != 2 {
		t.Fatalf("expected 2 deliveries, got %d", len(store.deliveries))
	}
	if sender.messages[0].Subscriber == sender.messages[1].Subscriber {
		t.Fatalf("expected independent subscriber messages, got %+v", sender.messages)
	}
}

func TestHandlerFailureIsPerSubscriber(t *testing.T) {
	bus := configuredBus()
	store := newFakeStore()
	sender := &fakeQueueSender{}
	bus.Configure(store, sender)

	if err := bus.Register(events.Subscription{
		Topic:      testTopic,
		Subscriber: "ok.subscriber",
	}, handlerFunc(func(context.Context, events.Event) error {
		return nil
	})); err != nil {
		t.Fatalf("register ok: %v", err)
	}
	if err := bus.Register(events.Subscription{
		Topic:      testTopic,
		Subscriber: "failed.subscriber",
	}, handlerFunc(func(context.Context, events.Event) error {
		return errHandlerFailed
	})); err != nil {
		t.Fatalf("register failed: %v", err)
	}

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceSystem)
	if err := bus.Publish(ctx, testEvent()); err != nil {
		t.Fatalf("publish durable: %v", err)
	}

	for _, message := range sender.messages {
		err := bus.HandleMessage(t.Context(), mustJSONMessage(t, message))
		if message.Subscriber == "failed.subscriber" && !errors.Is(err, events.ErrDurableSubscriberFailed) {
			t.Fatalf("expected durable subscriber failure, got %v", err)
		}
		if message.Subscriber == "ok.subscriber" && err != nil {
			t.Fatalf("expected ok subscriber success, got %v", err)
		}
	}

	eventID := sender.messages[0].EventID
	if got := store.status[eventID+"/ok.subscriber"]; got != "succeeded" {
		t.Fatalf("expected ok subscriber succeeded, got %s", got)
	}
	if got := store.status[eventID+"/failed.subscriber"]; got != "failed" {
		t.Fatalf("expected failed subscriber failed, got %s", got)
	}
}

func TestNewPayloadEventAndDecodePayload(t *testing.T) {
	type payload struct {
		OrderID string `json:"order_id"`
		Points  int64  `json:"points"`
	}

	event, err := events.NewPayloadEvent(testTopic, "order", "order-1", payload{
		OrderID: "order-1",
		Points:  10,
	})
	if err != nil {
		t.Fatalf("new payload event: %v", err)
	}
	if event.Topic != testTopic {
		t.Fatalf("expected topic %q, got %q", testTopic, event.Topic)
	}
	if event.AggregateType != "order" {
		t.Fatalf("expected aggregate type order, got %q", event.AggregateType)
	}
	if event.AggregateID != "order-1" {
		t.Fatalf("expected aggregate id order-1, got %q", event.AggregateID)
	}

	got, err := events.DecodePayload[payload](event)
	if err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	want := payload{OrderID: "order-1", Points: 10}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected payload %+v, got %+v", want, got)
	}
}

func TestTransactionalHandlerDecodesPayloadAndOpensAppTx(t *testing.T) {
	manager := setupEventsTestDB(t)
	appDB, err := manager.GetDB("app")
	if err != nil {
		t.Fatalf("get app db: %v", err)
	}

	type payload struct {
		UserID string `json:"user_id"`
	}
	event, err := events.NewPayloadEvent(testTopic, "user", "u1", payload{UserID: "u1"})
	if err != nil {
		t.Fatalf("new payload event: %v", err)
	}

	handler := events.TransactionalHandler(func(txCtx fwusecase.Context, gotEvent events.Event, gotPayload payload) error {
		if gotEvent.Topic != testTopic {
			return fmt.Errorf("expected topic %q, got %q", testTopic, gotEvent.Topic)
		}
		if gotPayload.UserID != "u1" {
			return fmt.Errorf("expected payload user u1, got %q", gotPayload.UserID)
		}
		if _, ok := db.SQLTxFor(txCtx.Std(), "app"); !ok {
			return fmt.Errorf("expected app transaction in handler context")
		}
		eng, err := db.EngineFor(txCtx.Std(), "app")
		if err != nil {
			return err
		}
		_, err = eng.ExecP(`INSERT INTO users (id, name, email, password_hash, email_verified, is_active) VALUES (?, ?, ?, ?, ?, ?)`, "u1", "Ada", "ada@example.com", "", 1, 1)
		return err
	})

	if err := handler.Handle(t.Context(), event); err != nil {
		t.Fatalf("handle transactional event: %v", err)
	}
	if got := countEventTestRows(t, appDB, `SELECT COUNT(*) FROM users WHERE id = 'u1'`); got != 1 {
		t.Fatalf("expected inserted user, got %d", got)
	}
}

func TestTransactionalHandlerRollsBackOnError(t *testing.T) {
	manager := setupEventsTestDB(t)
	appDB, err := manager.GetDB("app")
	if err != nil {
		t.Fatalf("get app db: %v", err)
	}

	event, err := events.NewPayloadEvent(testTopic, "user", "u1", struct{}{})
	if err != nil {
		t.Fatalf("new payload event: %v", err)
	}

	handler := events.TransactionalHandler(func(txCtx fwusecase.Context, _ events.Event, _ struct{}) error {
		eng, err := db.EngineFor(txCtx.Std(), "app")
		if err != nil {
			return err
		}
		if _, err := eng.ExecP(`INSERT INTO users (id, name, email, password_hash, email_verified, is_active) VALUES (?, ?, ?, ?, ?, ?)`, "u1", "Ada", "ada@example.com", "", 1, 1); err != nil {
			return err
		}
		return errHandlerFailed
	})

	err = handler.Handle(t.Context(), event)
	if !errors.Is(err, errHandlerFailed) {
		t.Fatalf("expected handler failure, got %v", err)
	}
	if got := countEventTestRows(t, appDB, `SELECT COUNT(*) FROM users WHERE id = 'u1'`); got != 0 {
		t.Fatalf("expected rollback to leave no users, got %d", got)
	}
}

func TestPublishInsideRolledBackTxLeavesNoDurableRowsOrMessages(t *testing.T) {
	manager := setupEventsTestDB(t)
	appDB, err := manager.GetDB("app")
	if err != nil {
		t.Fatalf("get app db: %v", err)
	}
	queueManager, err := queue.NewManager()
	if err != nil {
		t.Fatalf("new queue manager: %v", err)
	}

	bus := events.NewBus()
	bus.Configure(usecaseevents.DurableStore{}, queueManager)
	if err := bus.Register(events.Subscription{
		Topic:      testTopic,
		Subscriber: "audit.order_created",
	}, handlerFunc(func(context.Context, events.Event) error {
		return nil
	})); err != nil {
		t.Fatalf("register: %v", err)
	}

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceSystem)
	err = fwusecase.WithAppTx(ctx, func(txCtx fwusecase.Context) error {
		if err := bus.Publish(txCtx, testEvent()); err != nil {
			return err
		}
		return errHandlerFailed
	})
	if !errors.Is(err, errHandlerFailed) {
		t.Fatalf("expected rollback error, got %v", err)
	}

	for name, query := range map[string]string{
		"domain_events":           `SELECT COUNT(*) FROM domain_events`,
		"domain_event_deliveries": `SELECT COUNT(*) FROM domain_event_deliveries`,
		"goqite":                  `SELECT COUNT(*) FROM goqite WHERE queue = 'domain-events'`,
	} {
		var count int
		if err := appDB.Get(&count, query); err != nil {
			t.Fatalf("count %s: %v", name, err)
		}
		if count != 0 {
			t.Fatalf("expected rollback to leave no %s rows, got %d", name, count)
		}
	}
}

func configuredBus() *events.Bus {
	return events.NewBus()
}

func setupEventsTestDB(t *testing.T) *db.DBManager {
	t.Helper()

	previous := db.DefaultManager
	manager := db.NewDBManager()
	db.DefaultManager = manager

	path := filepath.Join(t.TempDir(), "app.db")
	t.Cleanup(func() {
		_ = manager.Close()
		db.DefaultManager = previous
	})

	if err := manager.Open("app", "sqlite", path); err != nil {
		t.Fatalf("open app db: %v", err)
	}
	if err := manager.AutoMigrate("app"); err != nil {
		t.Fatalf("migrate app db: %v", err)
	}
	return manager
}

func testEvent() events.Event {
	return events.Event{
		Topic:         testTopic,
		AggregateType: "order",
		AggregateID:   "order-1",
		PayloadJSON:   []byte(`{"order_id":"order-1"}`),
	}
}

func mustJSONMessage(t *testing.T, message events.Message) []byte {
	t.Helper()

	return []byte(`{"event_id":"` + message.EventID + `","subscriber":"` + message.Subscriber + `","topic":"` + message.Topic + `"}`)
}

func countEventTestRows(t *testing.T, appDB interface {
	Get(dest interface{}, query string, args ...interface{}) error
}, query string, args ...interface{}) int {
	t.Helper()

	var count int
	if err := appDB.Get(&count, query, args...); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	return count
}

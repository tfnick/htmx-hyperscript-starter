package events

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/tfnick/go-svelte-starter/api/framework/queue"
	"github.com/tfnick/go-svelte-starter/api/framework/timefmt"
	fwusecase "github.com/tfnick/go-svelte-starter/api/framework/usecase"
)

var (
	ErrInvalidEvent            = errors.New("invalid event")
	ErrInvalidSubscription     = errors.New("invalid event subscription")
	ErrDuplicateSubscription   = errors.New("duplicate event subscription")
	ErrDurableEventsNotReady   = errors.New("durable events are not configured")
	ErrDurableSubscriberFailed = errors.New("durable event subscriber failed")
)

type Event struct {
	ID            string
	Topic         string
	AggregateType string
	AggregateID   string
	PayloadJSON   []byte
	MetadataJSON  []byte
	OccurredAt    time.Time
}

type Subscription struct {
	Topic      string
	Subscriber string
}

type Handler interface {
	Handle(context.Context, Event) error
}

type Registry interface {
	Register(Subscription, Handler) error
	Subscribers(string) []Subscription
}

type Publisher interface {
	Publish(fwusecase.Context, Event) error
}

type Bus struct {
	mu       sync.RWMutex
	subs     map[string]map[string]Subscription
	store    Store
	queue    QueueSender
	handlers map[string]Handler
}

var defaultBus = NewBus()

func NewBus() *Bus {
	return &Bus{
		subs:     make(map[string]map[string]Subscription),
		handlers: make(map[string]Handler),
	}
}

type Store interface {
	InsertEvent(context.Context, Event) (string, error)
	InsertDelivery(context.Context, string, string, string) error
	LoadEvent(context.Context, string) (Event, error)
	MarkDeliveryRunning(context.Context, string, string) error
	MarkDeliverySucceeded(context.Context, string, string) error
	MarkDeliveryFailed(context.Context, string, string, string) error
}

type QueueSender interface {
	SendJSON(context.Context, queue.SendOptions, any) (string, error)
}

type Message struct {
	EventID    string `json:"event_id"`
	Subscriber string `json:"subscriber"`
	Topic      string `json:"topic"`
}

type PayloadEventOptions struct {
	ID            string
	Topic         string
	AggregateType string
	AggregateID   string
	MetadataJSON  []byte
	OccurredAt    time.Time
}

type TransactionalHandlerFunc[T any] func(fwusecase.Context, Event, T) error

func Configure(store Store, sender QueueSender) {
	defaultBus.Configure(store, sender)
}

func Register(sub Subscription, handler Handler) error {
	return defaultBus.Register(sub, handler)
}

func RegisterTransactional[T any](sub Subscription, fn TransactionalHandlerFunc[T]) error {
	if fn == nil {
		return fmt.Errorf("%w: transactional handler function is nil", ErrInvalidSubscription)
	}
	return Register(sub, TransactionalHandler(fn))
}

func Subscribers(topic string) []Subscription {
	return defaultBus.Subscribers(topic)
}

func Publish(ctx fwusecase.Context, event Event) error {
	return defaultBus.Publish(ctx, event)
}

func HandleMessage(ctx context.Context, message []byte) error {
	return defaultBus.HandleMessage(ctx, message)
}

func NewPayloadEvent[T any](topic string, aggregateType string, aggregateID string, payload T) (Event, error) {
	return NewPayloadEventWithOptions(PayloadEventOptions{
		Topic:         topic,
		AggregateType: aggregateType,
		AggregateID:   aggregateID,
	}, payload)
}

func NewPayloadEventWithOptions[T any](opts PayloadEventOptions, payload T) (Event, error) {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return Event{}, fmt.Errorf("%w: encode payload for topic %s: %v", ErrInvalidEvent, opts.Topic, err)
	}

	return Event{
		ID:            opts.ID,
		Topic:         opts.Topic,
		AggregateType: opts.AggregateType,
		AggregateID:   opts.AggregateID,
		PayloadJSON:   payloadJSON,
		MetadataJSON:  opts.MetadataJSON,
		OccurredAt:    opts.OccurredAt,
	}, nil
}

func DecodePayload[T any](event Event) (T, error) {
	var payload T
	if err := json.Unmarshal(event.PayloadJSON, &payload); err != nil {
		return payload, fmt.Errorf("%w: decode payload for topic %s: %v", ErrInvalidEvent, event.Topic, err)
	}
	return payload, nil
}

func TransactionalHandler[T any](fn TransactionalHandlerFunc[T]) Handler {
	return transactionalHandler[T]{fn: fn}
}

func (b *Bus) Configure(store Store, sender QueueSender) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.store = store
	b.queue = sender
}

func (b *Bus) Register(sub Subscription, handler Handler) error {
	if handler == nil {
		return fmt.Errorf("%w: handler is nil", ErrInvalidSubscription)
	}

	sub, err := normalizeSubscription(sub)
	if err != nil {
		logEventValidationError(err, sub)
		return err
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	if hasSubscriber(b.subs, sub.Topic, sub.Subscriber) {
		err := fmt.Errorf("%w: %s/%s", ErrDuplicateSubscription, sub.Topic, sub.Subscriber)
		logDuplicateSubscription(err, sub)
		return err
	}

	addSubscription(b.subs, sub)
	b.handlers[sub.Subscriber] = handler
	return nil
}

func (b *Bus) Subscribers(topic string) []Subscription {
	topic = strings.TrimSpace(topic)
	if topic == "" {
		return nil
	}

	b.mu.RLock()
	defer b.mu.RUnlock()

	bySubscriber := b.subs[topic]
	if len(bySubscriber) == 0 {
		return nil
	}

	list := make([]Subscription, 0, len(bySubscriber))
	for _, sub := range bySubscriber {
		list = append(list, sub)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].Subscriber < list[j].Subscriber
	})
	return list
}

func (b *Bus) Publish(ctx fwusecase.Context, event Event) error {
	event, err := normalizeEvent(event)
	if err != nil {
		logPublishValidationError(err, event)
		return err
	}
	return b.publishDurable(ctx, event)
}

func (b *Bus) publishDurable(ctx fwusecase.Context, event Event) error {
	subs := b.Subscribers(event.Topic)
	if len(subs) == 0 {
		return nil
	}

	b.mu.RLock()
	store := b.store
	sender := b.queue
	b.mu.RUnlock()
	if store == nil || sender == nil {
		return ErrDurableEventsNotReady
	}

	eventID, err := store.InsertEvent(ctx.Std(), event)
	if err != nil {
		return err
	}

	for _, sub := range subs {
		message := Message{
			EventID:    eventID,
			Subscriber: sub.Subscriber,
			Topic:      event.Topic,
		}
		messageID, err := sender.SendJSON(ctx.Std(), queue.SendOptions{
			Queue: queue.QueueDomainEvents,
		}, message)
		if err != nil {
			return err
		}
		if err := store.InsertDelivery(ctx.Std(), eventID, sub.Subscriber, messageID); err != nil {
			return err
		}
	}
	return nil
}

func (b *Bus) HandleMessage(ctx context.Context, message []byte) error {
	var payload Message
	if err := json.Unmarshal(message, &payload); err != nil {
		return err
	}
	payload.EventID = strings.TrimSpace(payload.EventID)
	payload.Subscriber = strings.TrimSpace(payload.Subscriber)
	if payload.EventID == "" || payload.Subscriber == "" {
		return fmt.Errorf("%w: durable message requires event_id and subscriber", ErrInvalidEvent)
	}

	b.mu.RLock()
	store := b.store
	handler := b.handlers[payload.Subscriber]
	b.mu.RUnlock()
	if store == nil {
		return ErrDurableEventsNotReady
	}
	if handler == nil {
		return fmt.Errorf("%w: subscriber %s is not registered", ErrInvalidSubscription, payload.Subscriber)
	}

	if err := store.MarkDeliveryRunning(ctx, payload.EventID, payload.Subscriber); err != nil {
		return err
	}

	event, err := store.LoadEvent(ctx, payload.EventID)
	if err != nil {
		_ = store.MarkDeliveryFailed(ctx, payload.EventID, payload.Subscriber, err.Error())
		return err
	}
	if err := handler.Handle(ctx, event); err != nil {
		_ = store.MarkDeliveryFailed(ctx, payload.EventID, payload.Subscriber, err.Error())
		return fmt.Errorf("%w: %s: %v", ErrDurableSubscriberFailed, payload.Subscriber, err)
	}
	return store.MarkDeliverySucceeded(ctx, payload.EventID, payload.Subscriber)
}

type transactionalHandler[T any] struct {
	fn TransactionalHandlerFunc[T]
}

func (h transactionalHandler[T]) Handle(ctx context.Context, event Event) error {
	if h.fn == nil {
		return fmt.Errorf("%w: transactional handler function is nil", ErrInvalidSubscription)
	}

	payload, err := DecodePayload[T](event)
	if err != nil {
		return err
	}

	ucCtx := fwusecase.NewContext(ctx, fwusecase.SurfaceSystem)
	return fwusecase.WithAppTx(ucCtx, func(txCtx fwusecase.Context) error {
		return h.fn(txCtx, event, payload)
	})
}

func normalizeEvent(event Event) (Event, error) {
	event.Topic = strings.TrimSpace(event.Topic)
	event.AggregateType = strings.TrimSpace(event.AggregateType)
	event.AggregateID = strings.TrimSpace(event.AggregateID)
	event.ID = strings.TrimSpace(event.ID)

	if event.Topic == "" {
		return event, fmt.Errorf("%w: topic is required", ErrInvalidEvent)
	}
	if event.ID == "" {
		event.ID = uuid.Must(uuid.NewV7()).String()
	}
	if event.OccurredAt.IsZero() {
		event.OccurredAt = timefmt.NowUTC()
	} else {
		event.OccurredAt = event.OccurredAt.UTC()
	}
	if len(event.MetadataJSON) == 0 {
		event.MetadataJSON = []byte("{}")
	}
	return event, nil
}

func normalizeSubscription(sub Subscription) (Subscription, error) {
	sub.Topic = strings.TrimSpace(sub.Topic)
	sub.Subscriber = strings.TrimSpace(sub.Subscriber)

	if sub.Topic == "" {
		return sub, fmt.Errorf("%w: topic is required", ErrInvalidSubscription)
	}
	if sub.Subscriber == "" {
		return sub, fmt.Errorf("%w: subscriber is required", ErrInvalidSubscription)
	}
	return sub, nil
}

func hasSubscriber(subs map[string]map[string]Subscription, topic string, subscriber string) bool {
	bySubscriber := subs[topic]
	if len(bySubscriber) == 0 {
		return false
	}
	_, ok := bySubscriber[subscriber]
	return ok
}

func addSubscription(subs map[string]map[string]Subscription, sub Subscription) {
	bySubscriber := subs[sub.Topic]
	if bySubscriber == nil {
		bySubscriber = make(map[string]Subscription)
		subs[sub.Topic] = bySubscriber
	}
	bySubscriber[sub.Subscriber] = sub
}

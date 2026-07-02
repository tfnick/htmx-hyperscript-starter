package events

import (
	"context"
	"time"

	fwevents "github.com/tfnick/go-svelte-starter/api/framework/events"
	"github.com/tfnick/go-svelte-starter/api/framework/timefmt"
	"github.com/tfnick/go-svelte-starter/api/models"
)

// DurableStore 是 framework/events 使用的持久化适配器。
//
// framework/events 只关心“如何保存事件、如何标记某个 subscriber 的投递状态”，
// 不直接依赖 app 里的 models。这个结构体把 framework/events.Store 接口
// 适配到项目自己的 domain_events / domain_event_deliveries 表。
//
// 典型链路：
// usecase 发布事件 -> framework/events 调用 DurableStore -> models 写 DB。
type DurableStore struct{}

// InsertEvent 保存一条领域事件事实。
//
// 这里写入的是 domain_events 表，表示“某件业务事实已经发生”，例如 order.paid。
// 注意它只保存事件本身，不保存每个 subscriber 的处理状态；subscriber 状态由
// InsertDelivery 单独写入 domain_event_deliveries。
func (DurableStore) InsertEvent(ctx context.Context, event fwevents.Event) (string, error) {
	persisted, err := models.InsertDomainEvent(ctx, models.InsertDomainEventCmd{
		ID:            event.ID,
		Topic:         event.Topic,
		AggregateType: event.AggregateType,
		AggregateID:   event.AggregateID,
		PayloadJSON:   event.PayloadJSON,
		MetadataJSON:  event.MetadataJSON,
		OccurredAt:    event.OccurredAt,
	})
	if err != nil {
		return "", err
	}
	return persisted.ID, nil
}

// InsertDelivery 为某个 subscriber 创建一条独立投递记录。
//
// 一个 event 可以有多个 subscriber，每个 subscriber 都会有自己的 delivery row
// 和 queue message。这样某个 subscriber 失败重试时，不会影响已经成功的 subscriber。
//
// messageID 是 goqite 队列消息 ID，用来把业务投递状态和底层队列消息关联起来。
func (DurableStore) InsertDelivery(ctx context.Context, eventID string, subscriber string, messageID string) error {
	_, err := models.InsertDomainEventDelivery(ctx, models.InsertDomainEventDeliveryCmd{
		EventID:    eventID,
		Subscriber: subscriber,
		MessageID:  messageID,
	})
	return err
}

// LoadEvent 从 domain_events 表还原 framework/events.Event。
//
// queue message 本身只携带 event_id / subscriber / topic，不携带完整 payload。
// worker 处理消息时，会先根据 event_id 加载持久化事件，再交给对应 subscriber。
// 这样可以保证 payload 来源稳定，也避免队列消息和事件事实出现两份不一致的数据。
func (DurableStore) LoadEvent(ctx context.Context, id string) (fwevents.Event, error) {
	event, err := models.GetDomainEvent(ctx, id)
	if err != nil {
		return fwevents.Event{}, err
	}

	occurredAt, err := time.Parse(time.RFC3339Nano, event.OccurredAt)
	if err != nil {
		occurredAt = timefmt.NowUTC()
	}

	return fwevents.Event{
		ID:            event.ID,
		Topic:         event.Topic,
		AggregateType: event.AggregateType,
		AggregateID:   event.AggregateID,
		PayloadJSON:   []byte(event.PayloadJSON),
		MetadataJSON:  []byte(event.MetadataJSON),
		OccurredAt:    occurredAt.UTC(),
	}, nil
}

// MarkDeliveryRunning 标记某个 subscriber 的投递开始处理。
//
// 这个状态用于后台观察和排查：消息已经被 worker 收到，并进入对应 handler。
func (DurableStore) MarkDeliveryRunning(ctx context.Context, eventID string, subscriber string) error {
	return models.MarkDomainEventDeliveryRunning(ctx, eventID, subscriber)
}

// MarkDeliverySucceeded 标记某个 subscriber 的投递处理成功。
//
// queue runner 会在 handler 返回 nil 后删除队列消息；这里的 succeeded 则保留
// 在业务表里，方便后续查询每个 subscriber 的处理历史。
func (DurableStore) MarkDeliverySucceeded(ctx context.Context, eventID string, subscriber string) error {
	return models.MarkDomainEventDeliverySucceeded(ctx, eventID, subscriber)
}

// MarkDeliveryFailed 标记某个 subscriber 的投递处理失败。
//
// handler 返回 error 时 framework/events 会调用这里记录失败原因。队列消息不会被
// 成功删除，后续会按 queue 的重试/超时机制再次投递，因此 subscriber 自身必须保证幂等。
func (DurableStore) MarkDeliveryFailed(ctx context.Context, eventID string, subscriber string, message string) error {
	return models.MarkDomainEventDeliveryFailed(ctx, eventID, subscriber, message)
}

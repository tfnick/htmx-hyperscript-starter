package models

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tfnick/go-svelte-starter/api/db"
)

const (
	DomainEventDeliveryStatusQueued    = "queued"
	DomainEventDeliveryStatusRunning   = "running"
	DomainEventDeliveryStatusSucceeded = "succeeded"
	DomainEventDeliveryStatusFailed    = "failed"
)

type DomainEvent struct {
	ID            string `db:"id"`
	Topic         string `db:"topic"`
	AggregateType string `db:"aggregate_type"`
	AggregateID   string `db:"aggregate_id"`
	PayloadJSON   string `db:"payload_json"`
	MetadataJSON  string `db:"metadata_json"`
	OccurredAt    string `db:"occurred_at"`
	CreatedAt     string `db:"created_at"`
}

type DomainEventDelivery struct {
	ID         string `db:"id"`
	EventID    string `db:"event_id"`
	Subscriber string `db:"subscriber"`
	MessageID  string `db:"message_id"`
	Status     string `db:"status"`
	Attempts   int    `db:"attempts"`
	LastError  string `db:"last_error"`
	CreatedAt  string `db:"created_at"`
	UpdatedAt  string `db:"updated_at"`
}

type InsertDomainEventCmd struct {
	ID            string
	Topic         string
	AggregateType string
	AggregateID   string
	PayloadJSON   []byte
	MetadataJSON  []byte
	OccurredAt    time.Time
}

type InsertDomainEventDeliveryCmd struct {
	EventID    string
	Subscriber string
	MessageID  string
}

func InsertDomainEvent(ctx context.Context, cmd InsertDomainEventCmd) (DomainEvent, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return DomainEvent{}, fmt.Errorf("database unavailable: %w", err)
	}

	event := DomainEvent{
		ID:            cmd.ID,
		Topic:         cmd.Topic,
		AggregateType: cmd.AggregateType,
		AggregateID:   cmd.AggregateID,
		PayloadJSON:   string(cmd.PayloadJSON),
		MetadataJSON:  string(cmd.MetadataJSON),
		OccurredAt:    cmd.OccurredAt.UTC().Format(time.RFC3339Nano),
	}
	if event.ID == "" {
		event.ID = uuid.Must(uuid.NewV7()).String()
	}
	if event.MetadataJSON == "" {
		event.MetadataJSON = "{}"
	}

	_, err = d.ExecNamed(`
		INSERT INTO domain_events (
			id, topic, aggregate_type, aggregate_id, payload_json, metadata_json, occurred_at
		) VALUES (
			:id, :topic, :aggregate_type, :aggregate_id, :payload_json, :metadata_json, :occurred_at
		)
	`, event)

	if err != nil {
		return DomainEvent{}, fmt.Errorf("insert domain event failed: %w", err)
	}
	return event, nil
}

func InsertDomainEventDelivery(ctx context.Context, cmd InsertDomainEventDeliveryCmd) (DomainEventDelivery, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return DomainEventDelivery{}, fmt.Errorf("database unavailable: %w", err)
	}

	delivery := DomainEventDelivery{
		ID:         uuid.Must(uuid.NewV7()).String(),
		EventID:    cmd.EventID,
		Subscriber: cmd.Subscriber,
		MessageID:  cmd.MessageID,
		Status:     DomainEventDeliveryStatusQueued,
	}
	_, err = d.ExecNamed(`
		INSERT INTO domain_event_deliveries (
			id, event_id, subscriber, message_id, status
		) VALUES (
			:id, :event_id, :subscriber, :message_id, :status
		)
	`, delivery)

	if err != nil {
		return DomainEventDelivery{}, fmt.Errorf("insert domain event delivery failed: %w", err)
	}
	return delivery, nil
}

func GetDomainEvent(ctx context.Context, id string) (DomainEvent, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return DomainEvent{}, fmt.Errorf("database unavailable: %w", err)
	}

	var event DomainEvent
	query := `
		SELECT id, topic, aggregate_type, aggregate_id, payload_json, metadata_json, occurred_at, created_at
		FROM domain_events
		WHERE id = ?
	`
	if err := d.GetP(&event, query, id); err != nil {
		return DomainEvent{}, fmt.Errorf("get domain event failed: %w", err)
	}
	return event, nil
}

func CountDomainEvents(ctx context.Context) (int, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return 0, fmt.Errorf("database unavailable: %w", err)
	}

	var count int
	if err := d.GetP(&count, `SELECT COUNT(*) FROM domain_events`); err != nil {
		return 0, fmt.Errorf("count domain events failed: %w", err)
	}
	return count, nil
}

func ListDomainEvents(ctx context.Context, limit int, offset int) ([]DomainEvent, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return nil, fmt.Errorf("database unavailable: %w", err)
	}

	var events []DomainEvent
	query := `
		SELECT id, topic, aggregate_type, aggregate_id, payload_json, metadata_json, occurred_at, created_at
		FROM domain_events
		ORDER BY created_at DESC, id DESC
		LIMIT ? OFFSET ?
	`
	if err := d.SelectP(&events, query, limit, offset); err != nil {
		return nil, fmt.Errorf("list domain events failed: %w", err)
	}
	return events, nil
}

func ListDomainEventDeliveries(ctx context.Context, eventID string) ([]DomainEventDelivery, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return nil, fmt.Errorf("database unavailable: %w", err)
	}

	var deliveries []DomainEventDelivery
	query := `
		SELECT id, event_id, subscriber,
			COALESCE(message_id, '') AS message_id,
			status, attempts,
			COALESCE(last_error, '') AS last_error,
			created_at, updated_at
		FROM domain_event_deliveries
		WHERE event_id = ?
		ORDER BY created_at ASC, subscriber ASC
	`
	if err := d.SelectP(&deliveries, query, eventID); err != nil {
		return nil, fmt.Errorf("list domain event deliveries failed: %w", err)
	}
	return deliveries, nil
}

func MarkDomainEventDeliveryRunning(ctx context.Context, eventID string, subscriber string) error {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return fmt.Errorf("database unavailable: %w", err)
	}

	query := `
		UPDATE domain_event_deliveries
		SET status = ?, attempts = attempts + 1, last_error = ''
		WHERE event_id = ? AND subscriber = ?
	`
	_, err = d.ExecP(query, DomainEventDeliveryStatusRunning, eventID, subscriber)
	if err != nil {
		return fmt.Errorf("mark domain event delivery running failed: %w", err)
	}
	return nil
}

func MarkDomainEventDeliverySucceeded(ctx context.Context, eventID string, subscriber string) error {
	return finishDomainEventDelivery(ctx, eventID, subscriber, DomainEventDeliveryStatusSucceeded, "")
}

func MarkDomainEventDeliveryFailed(ctx context.Context, eventID string, subscriber string, message string) error {
	return finishDomainEventDelivery(ctx, eventID, subscriber, DomainEventDeliveryStatusFailed, message)
}

func finishDomainEventDelivery(ctx context.Context, eventID string, subscriber string, status string, lastError string) error {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return fmt.Errorf("database unavailable: %w", err)
	}

	query := `
		UPDATE domain_event_deliveries
		SET status = ?, last_error = ?
		WHERE event_id = ? AND subscriber = ?
	`
	_, err = d.ExecP(query, status, lastError, eventID, subscriber)
	if err != nil {
		return fmt.Errorf("update domain event delivery failed: %w", err)
	}
	return nil
}

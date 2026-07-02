package models

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/tfnick/go-svelte-starter/api/db"
	"github.com/tfnick/go-svelte-starter/api/framework/data/modelerror"
	"github.com/tfnick/go-svelte-starter/api/framework/timefmt"
)

const (
	ExternalNotificationDeliveryStatusQueued    = "queued"
	ExternalNotificationDeliveryStatusRunning   = "running"
	ExternalNotificationDeliveryStatusSucceeded = "succeeded"
	ExternalNotificationDeliveryStatusFailed    = "failed"
	ExternalNotificationDeliveryStatusSkipped   = "skipped"
)

type ExternalNotificationDelivery struct {
	ID                   string `db:"id"`
	EventID              string `db:"event_id"`
	ChannelID            string `db:"channel_id"`
	ProviderCode         string `db:"provider_code"`
	AdapterKey           string `db:"adapter_key"`
	IdempotencyKey       string `db:"idempotency_key"`
	Status               string `db:"status"`
	Attempts             int    `db:"attempts"`
	NextAttemptAt        string `db:"next_attempt_at"`
	LastErrorCode        string `db:"last_error_code"`
	LastErrorMessage     string `db:"last_error_message"`
	RequestSnapshotJSON  string `db:"request_snapshot_json"`
	ResponseSnapshotJSON string `db:"response_snapshot_json"`
	Note                 string `db:"note"`
	SentAt               string `db:"sent_at"`
	MessageID            string `db:"message_id"`
	CreatedAt            string `db:"created_at"`
	UpdatedAt            string `db:"updated_at"`
}

type CreateExternalNotificationDeliveryCmd struct {
	EventID              string
	ChannelID            string
	ProviderCode         string
	AdapterKey           string
	IdempotencyKey       string
	Status               string
	RequestSnapshotJSON  string
	ResponseSnapshotJSON string
	Note                 string
}

func CreateExternalNotificationDelivery(ctx context.Context, cmd CreateExternalNotificationDeliveryCmd) (ExternalNotificationDelivery, bool, error) {
	if existing, err := GetExternalNotificationDeliveryByIdempotencyKey(ctx, cmd.IdempotencyKey); err == nil {
		return existing, false, nil
	} else if !errors.Is(err, modelerror.ErrNotFound) {
		return ExternalNotificationDelivery{}, false, err
	}

	now := timefmt.NowSQLiteDateTime()
	delivery := ExternalNotificationDelivery{
		ID:                   uuid.Must(uuid.NewV7()).String(),
		EventID:              cmd.EventID,
		ChannelID:            cmd.ChannelID,
		ProviderCode:         cmd.ProviderCode,
		AdapterKey:           cmd.AdapterKey,
		IdempotencyKey:       cmd.IdempotencyKey,
		Status:               cmd.Status,
		RequestSnapshotJSON:  cmd.RequestSnapshotJSON,
		ResponseSnapshotJSON: cmd.ResponseSnapshotJSON,
		Note:                 cmd.Note,
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	if delivery.Status == "" {
		delivery.Status = ExternalNotificationDeliveryStatusQueued
	}
	if delivery.RequestSnapshotJSON == "" {
		delivery.RequestSnapshotJSON = "{}"
	}
	if delivery.ResponseSnapshotJSON == "" {
		delivery.ResponseSnapshotJSON = "{}"
	}

	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return ExternalNotificationDelivery{}, false, fmt.Errorf("database unavailable: %w", err)
	}
	if _, err := d.ExecNamed(`
		INSERT INTO external_notification_deliveries (
			id, event_id, channel_id, provider_code, adapter_key, idempotency_key,
			status, request_snapshot_json, response_snapshot_json, note, created_at, updated_at
		) VALUES (
			:id, :event_id, :channel_id, :provider_code, :adapter_key, :idempotency_key,
			:status, :request_snapshot_json, :response_snapshot_json, :note, :created_at, :updated_at
		) ON CONFLICT(idempotency_key) DO NOTHING
	`, delivery); err != nil {
		return ExternalNotificationDelivery{}, false, fmt.Errorf("create external notification delivery failed: %w", err)
	}
	created, err := GetExternalNotificationDeliveryByIdempotencyKey(ctx, cmd.IdempotencyKey)
	if err != nil {
		return ExternalNotificationDelivery{}, false, err
	}
	return created, created.ID == delivery.ID, nil
}

func GetExternalNotificationDeliveryByID(ctx context.Context, id string) (ExternalNotificationDelivery, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return ExternalNotificationDelivery{}, fmt.Errorf("database unavailable: %w", err)
	}
	query := externalNotificationDeliverySelectSQL() + ` WHERE id = ? LIMIT 1`
	var delivery ExternalNotificationDelivery
	if err := d.GetP(&delivery, query, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ExternalNotificationDelivery{}, fmt.Errorf("external notification delivery not found: %w", modelerror.ErrNotFound)
		}
		return ExternalNotificationDelivery{}, fmt.Errorf("load external notification delivery failed: %w", err)
	}
	return delivery, nil
}

func GetExternalNotificationDeliveryByIdempotencyKey(ctx context.Context, key string) (ExternalNotificationDelivery, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return ExternalNotificationDelivery{}, fmt.Errorf("database unavailable: %w", err)
	}
	query := externalNotificationDeliverySelectSQL() + ` WHERE idempotency_key = ? LIMIT 1`
	var delivery ExternalNotificationDelivery
	if err := d.GetP(&delivery, query, key); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ExternalNotificationDelivery{}, fmt.Errorf("external notification delivery not found: %w", modelerror.ErrNotFound)
		}
		return ExternalNotificationDelivery{}, fmt.Errorf("load external notification delivery failed: %w", err)
	}
	return delivery, nil
}

func MarkExternalNotificationDeliveryQueued(ctx context.Context, id string, messageID string) error {
	return updateExternalNotificationDelivery(ctx, id, ExternalNotificationDeliveryStatusQueued, externalNotificationDeliveryUpdate{
		MessageID: messageID,
	})
}

func MarkExternalNotificationDeliveryRunning(ctx context.Context, id string) error {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return fmt.Errorf("database unavailable: %w", err)
	}
	query := `
		UPDATE external_notification_deliveries
		SET status = ?, attempts = attempts + 1, last_error_code = '', last_error_message = '', updated_at = ?
		WHERE id = ?
	`
	result, err := d.ExecP(query, ExternalNotificationDeliveryStatusRunning, timefmt.NowSQLiteDateTime(), id)
	if err != nil {
		return fmt.Errorf("mark external notification delivery running failed: %w", err)
	}
	return requireRowsAffected(result, "external notification delivery not found")
}

func MarkExternalNotificationDeliverySucceeded(ctx context.Context, id string, responseSnapshotJSON string) error {
	return updateExternalNotificationDelivery(ctx, id, ExternalNotificationDeliveryStatusSucceeded, externalNotificationDeliveryUpdate{
		ResponseSnapshotJSON: responseSnapshotJSON,
		SentAt:               timefmt.NowSQLiteDateTime(),
	})
}

func MarkExternalNotificationDeliveryFailed(ctx context.Context, id string, errorCode string, errorMessage string, responseSnapshotJSON string) error {
	return updateExternalNotificationDelivery(ctx, id, ExternalNotificationDeliveryStatusFailed, externalNotificationDeliveryUpdate{
		LastErrorCode:        errorCode,
		LastErrorMessage:     errorMessage,
		ResponseSnapshotJSON: responseSnapshotJSON,
	})
}

type externalNotificationDeliveryUpdate struct {
	MessageID            string
	LastErrorCode        string
	LastErrorMessage     string
	ResponseSnapshotJSON string
	SentAt               string
}

func updateExternalNotificationDelivery(ctx context.Context, id string, status string, update externalNotificationDeliveryUpdate) error {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return fmt.Errorf("database unavailable: %w", err)
	}
	responseSnapshot := update.ResponseSnapshotJSON
	if responseSnapshot == "" {
		responseSnapshot = "{}"
	}
	query := `
		UPDATE external_notification_deliveries SET
			status = ?,
			message_id = CASE WHEN ? <> '' THEN ? ELSE message_id END,
			last_error_code = ?,
			last_error_message = ?,
			response_snapshot_json = ?,
			sent_at = NULLIF(?, ''),
			updated_at = ?
		WHERE id = ?
	`
	result, err := d.ExecP(
		query,
		status,
		update.MessageID,
		update.MessageID,
		update.LastErrorCode,
		update.LastErrorMessage,
		responseSnapshot,
		update.SentAt,
		timefmt.NowSQLiteDateTime(),
		id,
	)
	if err != nil {
		return fmt.Errorf("update external notification delivery failed: %w", err)
	}
	return requireRowsAffected(result, "external notification delivery not found")
}

func externalNotificationDeliverySelectSQL() string {
	return `
		SELECT id, event_id, channel_id, provider_code, adapter_key, idempotency_key,
			status, attempts, COALESCE(next_attempt_at, '') AS next_attempt_at,
			last_error_code, last_error_message, request_snapshot_json, response_snapshot_json,
			note, COALESCE(sent_at, '') AS sent_at, message_id, created_at, updated_at
		FROM external_notification_deliveries
	`
}

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
	NotificationStatusPending = "pending"
	NotificationStatusSent    = "sent"
	NotificationStatusFailed  = "failed"
	NotificationStatusSkipped = "skipped"
)

type Notification struct {
	ID               string `db:"id"`
	NotificationType string `db:"notification_type"`
	SourceType       string `db:"source_type"`
	SourceID         string `db:"source_id"`
	UserID           string `db:"user_id"`
	RecipientEmail   string `db:"recipient_email"`
	RecipientPhone   string `db:"recipient_phone"`
	Title            string `db:"title"`
	Summary          string `db:"summary"`
	PayloadJSON      string `db:"payload_json"`
	Status           string `db:"status"`
	LastError        string `db:"last_error"`
	SentAt           string `db:"sent_at"`
	ClearedAt        string `db:"cleared_at"`
	CreatedAt        string `db:"created_at"`
	UpdatedAt        string `db:"updated_at"`
}

type NotificationQuery struct {
	NotificationType string `db:"notification_type"`
	RecipientEmail   string `db:"recipient_email"`
	RecipientPhone   string `db:"recipient_phone"`
	Limit            int    `db:"limit"`
	Offset           int    `db:"offset"`
}

func InsertNotification(ctx context.Context, notification *Notification) error {
	if notification.ID == "" {
		notification.ID = uuid.Must(uuid.NewV7()).String()
	}
	if notification.PayloadJSON == "" {
		notification.PayloadJSON = "{}"
	}
	if notification.Status == "" {
		notification.Status = NotificationStatusPending
	}
	now := timefmt.NowSQLiteDateTime()
	notification.CreatedAt = now
	notification.UpdatedAt = now

	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return fmt.Errorf("database unavailable: %w", err)
	}
	if _, err := d.ExecNamed(`
		INSERT INTO notifications (
			id, notification_type, source_type, source_id, user_id, recipient_email,
			recipient_phone, title, summary, payload_json, status, last_error,
			sent_at, created_at, updated_at
		) VALUES (
			:id, :notification_type, :source_type, :source_id, :user_id, :recipient_email,
			:recipient_phone, :title, :summary, :payload_json, :status, :last_error,
			NULLIF(:sent_at, ''), :created_at, :updated_at
		)
	`, notification); err != nil {
		return fmt.Errorf("insert notification failed: %w", err)
	}
	return nil
}

func GetNotificationByID(ctx context.Context, id string) (Notification, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return Notification{}, fmt.Errorf("database unavailable: %w", err)
	}

	query := notificationSelectSQL() + `
		WHERE id = ?
		LIMIT 1
	`
	var notification Notification
	if err := d.GetP(&notification, query, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Notification{}, fmt.Errorf("notification not found: %w", modelerror.ErrNotFound)
		}
		return Notification{}, fmt.Errorf("get notification failed: %w", err)
	}
	return notification, nil
}

func UpdateNotificationStatus(ctx context.Context, id string, status string, lastError string, sentAt string) error {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return fmt.Errorf("database unavailable: %w", err)
	}

	query := `
		UPDATE notifications SET
			status = ?,
			last_error = ?,
			sent_at = NULLIF(?, ''),
			updated_at = ?
		WHERE id = ?
	`
	result, err := d.ExecP(query, status, lastError, sentAt, timefmt.NowSQLiteDateTime(), id)
	if err != nil {
		return fmt.Errorf("update notification status failed: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read affected rows failed: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("notification not found: %w", modelerror.ErrNotFound)
	}
	return nil
}

func CountNotifications(ctx context.Context, query NotificationQuery) (int, error) {
	eng, err := db.EngineFor(ctx, "app")
	if err != nil {
		return 0, fmt.Errorf("database unavailable: %w", err)
	}

	sql := `
		SELECT COUNT(*)
		FROM notifications
		WHERE 1=1
			#[ AND notification_type = :notification_type ]
			#[ AND recipient_email LIKE :recipient_email ]
			#[ AND recipient_phone LIKE :recipient_phone ]
	`
	var count int
	if err := eng.Get(&count, sql, query); err != nil {
		return 0, fmt.Errorf("count notifications failed: %w", err)
	}
	return count, nil
}

func ListNotifications(ctx context.Context, query NotificationQuery) ([]Notification, error) {
	eng, err := db.EngineFor(ctx, "app")
	if err != nil {
		return nil, fmt.Errorf("database unavailable: %w", err)
	}

	sql := notificationSelectSQL() + `
		WHERE 1=1
			#[ AND notification_type = :notification_type ]
			#[ AND recipient_email LIKE :recipient_email ]
			#[ AND recipient_phone LIKE :recipient_phone ]
		ORDER BY created_at DESC, id DESC
		LIMIT :limit OFFSET :offset
	`
	var notifications []Notification
	if err := eng.Select(&notifications, sql, query); err != nil {
		return nil, fmt.Errorf("list notifications failed: %w", err)
	}
	return notifications, nil
}

func ClearNotificationsByUser(ctx context.Context, userID string) (int, error) {
	now := timefmt.NowSQLiteDateTime()

	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return 0, fmt.Errorf("database unavailable: %w", err)
	}

	query := `
		UPDATE notifications
		SET cleared_at = ?, updated_at = ?
		WHERE user_id = ?
		  AND cleared_at = ''
	`
	result, err := d.ExecP(query, now, now, userID)
	if err != nil {
		return 0, fmt.Errorf("clear notifications failed: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("read affected rows failed: %w", err)
	}
	return int(rows), nil
}

func notificationSelectSQL() string {
	return `
		SELECT id, notification_type, source_type, source_id, user_id, recipient_email,
			recipient_phone, title, summary, payload_json, status, last_error,
			COALESCE(sent_at, '') AS sent_at, cleared_at, created_at, updated_at
		FROM notifications
	`
}

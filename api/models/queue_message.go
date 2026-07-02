package models

import (
	"context"
	"fmt"

	"github.com/tfnick/go-svelte-starter/api/db"
)

type QueueMessage struct {
	ID          string `db:"id"`
	Queue       string `db:"queue"`
	BodyPreview string `db:"body_preview"`
	Created     string `db:"created"`
	Updated     string `db:"updated"`
	Timeout     string `db:"timeout"`
	Received    int    `db:"received"`
	Priority    int    `db:"priority"`
}

type QueueMessageSnapshot struct {
	ID       string `db:"id"`
	Queue    string `db:"queue"`
	Created  string `db:"created"`
	Updated  string `db:"updated"`
	Timeout  string `db:"timeout"`
	Received int    `db:"received"`
}

type QueueMessageReference struct {
	Type        string `db:"type"`
	ID          string `db:"id"`
	Status      string `db:"status"`
	RelatedType string `db:"related_type"`
	RelatedID   string `db:"related_id"`
}

func ListQueueMessages(ctx context.Context, queueName string) ([]QueueMessage, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return nil, fmt.Errorf("database unavailable: %w", err)
	}
	driver, err := db.DriverFor("app")
	if err != nil {
		return nil, fmt.Errorf("database driver unavailable: %w", err)
	}

	var messages []QueueMessage
	if queueName == "" {
		if err := d.SelectP(&messages, queueMessageListSQL(driver)); err != nil {
			return nil, fmt.Errorf("list queue messages failed: %w", err)
		}
		return messages, nil
	}

	query := queueMessageListByQueueSQL(driver)
	if err := d.SelectP(&messages, query, queueName); err != nil {
		return nil, fmt.Errorf("list queue messages failed: %w", err)
	}
	return messages, nil
}

func ListQueueMessageSnapshots(ctx context.Context, queueName string) ([]QueueMessageSnapshot, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return nil, fmt.Errorf("database unavailable: %w", err)
	}

	var messages []QueueMessageSnapshot
	if queueName == "" {
		if err := d.SelectP(&messages, `
			SELECT id, queue, created, updated, timeout, received
			FROM goqite
			ORDER BY queue ASC, created ASC
		`); err != nil {
			return nil, fmt.Errorf("list queue message snapshots failed: %w", err)
		}
		return messages, nil
	}

	if err := d.SelectP(&messages, `
		SELECT id, queue, created, updated, timeout, received
		FROM goqite
		WHERE queue = ?
		ORDER BY queue ASC, created ASC
	`, queueName); err != nil {
		return nil, fmt.Errorf("list queue message snapshots failed: %w", err)
	}
	return messages, nil
}

func ListQueueMessageReferences(ctx context.Context, messageID string) ([]QueueMessageReference, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return nil, fmt.Errorf("database unavailable: %w", err)
	}

	var refs []QueueMessageReference
	query := `
		SELECT 'domain_event_delivery' AS type, id, status, 'domain_event' AS related_type, event_id AS related_id
		FROM domain_event_deliveries
		WHERE message_id = ?
		UNION ALL
		SELECT 'integration_webhook_receipt' AS type, id, status, 'scenario' AS related_type, scenario AS related_id
		FROM integration_webhook_receipts
		WHERE message_id = ?
		UNION ALL
		SELECT 'external_notification_delivery' AS type, id, status, 'domain_event' AS related_type, event_id AS related_id
		FROM external_notification_deliveries
		WHERE message_id = ?
		UNION ALL
		SELECT 'async_task' AS type, id, status, 'task_type' AS related_type, task_type AS related_id
		FROM async_tasks
		WHERE message_id = ?
		UNION ALL
		SELECT 'scheduled_task_execution' AS type, id, status, 'scheduled_task' AS related_type, task_id AS related_id
		FROM scheduled_task_executions
		WHERE message_id = ?
		ORDER BY type ASC, id ASC
	`
	if err := d.SelectP(&refs, query, messageID, messageID, messageID, messageID, messageID); err != nil {
		return nil, fmt.Errorf("list queue message references failed: %w", err)
	}
	return refs, nil
}

func queueMessageListSQL(driver db.Driver) string {
	return fmt.Sprintf(`
		SELECT id, queue, %s AS body_preview, created, updated, timeout, received, priority
		FROM goqite
		ORDER BY created DESC
	`, queueMessageBodyPreviewExpr(driver))
}

func queueMessageListByQueueSQL(driver db.Driver) string {
	return fmt.Sprintf(`
		SELECT id, queue, %s AS body_preview, created, updated, timeout, received, priority
		FROM goqite
		WHERE queue = ?
		ORDER BY created DESC
	`, queueMessageBodyPreviewExpr(driver))
}

func queueMessageBodyPreviewExpr(driver db.Driver) string {
	if driver == db.DriverPostgres {
		return "substr(encode(body, 'escape'), 1, 240)"
	}
	return "substr(CAST(body AS TEXT), 1, 240)"
}

package usecase_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/tfnick/go-svelte-starter/api/db"
	"github.com/tfnick/go-svelte-starter/api/framework/realtime"
	fwusecase "github.com/tfnick/go-svelte-starter/api/framework/usecase"
	"github.com/tfnick/go-svelte-starter/api/models"
	"github.com/tfnick/go-svelte-starter/api/usecase"
	"github.com/tfnick/sqlx"
)

func TestCreateNotificationRealtimeCreatesLedgerAndPublishesSafeRealtimePayload(t *testing.T) {
	setupUsecaseOrderTxDB(t)
	sub := realtime.SubscribeClient("notify-user-1", "notify-client-1")
	defer sub.Close()

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	notification, err := usecase.CreateNotification(ctx, usecase.CreateNotificationCmd{
		NotificationType: "realtime",
		SourceType:       "order",
		SourceID:         "order-1",
		UserID:           "notify-user-1",
		Title:            "Order paid",
		Summary:          "Your points have been awarded",
		PayloadJSON:      `{"secret":"do-not-push","order_id":"order-1"}`,
	})
	if err != nil {
		t.Fatalf("create realtime notification: %v", err)
	}
	if notification.ID == "" || notification.Status != models.NotificationStatusSent || notification.SentAt == "" {
		t.Fatalf("expected sent notification with timestamps, got %#v", notification)
	}
	if notification.NotificationTypeLabel != "Realtime" {
		t.Fatalf("expected dictionary label, got %#v", notification)
	}

	select {
	case raw := <-sub.Messages:
		var message struct {
			Type         string                 `json:"type"`
			Presentation string                 `json:"presentation"`
			Payload      map[string]interface{} `json:"payload"`
		}
		if err := json.Unmarshal(raw, &message); err != nil {
			t.Fatalf("decode realtime message: %v", err)
		}
		if message.Type != "notification" || message.Presentation != "toast" {
			t.Fatalf("unexpected realtime envelope: %s", raw)
		}
		if message.Payload["id"] != notification.ID || message.Payload["title"] != "Order paid" {
			t.Fatalf("unexpected notification payload: %#v", message.Payload)
		}
		if _, exists := message.Payload["status"]; exists {
			t.Fatalf("expected empty status for payload without status, got %#v", message.Payload)
		}
		if _, exists := message.Payload["payload_json"]; exists {
			t.Fatalf("expected realtime payload to omit raw payload_json: %#v", message.Payload)
		}
		if _, exists := message.Payload["secret"]; exists {
			t.Fatalf("expected realtime payload to omit payload JSON fields: %#v", message.Payload)
		}
	case <-time.After(time.Second):
		t.Fatalf("expected realtime notification")
	}

	persisted, err := models.GetNotificationByID(t.Context(), notification.ID)
	if err != nil {
		t.Fatalf("get persisted notification: %v", err)
	}
	if persisted.Status != models.NotificationStatusSent || persisted.PayloadJSON != `{"order_id":"order-1","secret":"do-not-push"}` {
		t.Fatalf("unexpected persisted notification: %#v", persisted)
	}
}

func TestSendNotificationDefaultPolicyStoresAndPublishesNotification(t *testing.T) {
	setupUsecaseOrderTxDB(t)
	sub := realtime.SubscribeClient("notify-user-2", "notify-client-2")
	defer sub.Close()

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	notification, err := usecase.SendNotification(ctx, usecase.SendNotificationCmd{
		UserID:     "notify-user-2",
		SourceType: "async_task",
		SourceID:   "task-1",
		Title:      "Task failed",
		Summary:    "Export failed",
		Payload: map[string]string{
			"task_id": "task-1",
			"status":  "failed",
		},
	})
	if err != nil {
		t.Fatalf("send stored notification: %v", err)
	}
	if notification.ID == "" || notification.Status != models.NotificationStatusSent {
		t.Fatalf("expected stored notification, got %#v", notification)
	}

	select {
	case raw := <-sub.Messages:
		var message struct {
			Type    string `json:"type"`
			Payload struct {
				ID     string `json:"id"`
				Status string `json:"status"`
			} `json:"payload"`
		}
		if err := json.Unmarshal(raw, &message); err != nil {
			t.Fatalf("decode realtime message: %v", err)
		}
		if message.Type != "notification" || message.Payload.ID != notification.ID || message.Payload.Status != "failed" {
			t.Fatalf("unexpected realtime notification: %#v", message)
		}
	case <-time.After(time.Second):
		t.Fatalf("expected realtime notification")
	}
}

func TestCreateNotificationNonRealtimeStoresSkippedLedgerOnly(t *testing.T) {
	setupUsecaseOrderTxDB(t)
	sub := realtime.SubscribeClient("email-user-1", "email-client-1")
	defer sub.Close()

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	notification, err := usecase.CreateNotification(ctx, usecase.CreateNotificationCmd{
		NotificationType: "email",
		UserID:           "email-user-1",
		RecipientEmail:   "ada@example.com",
		Title:            "Welcome",
	})
	if err != nil {
		t.Fatalf("create email notification: %v", err)
	}
	if notification.Status != models.NotificationStatusSkipped {
		t.Fatalf("expected skipped status for non-realtime MVP channel, got %#v", notification)
	}
	if notification.NotificationTypeLabel != "Email" {
		t.Fatalf("expected dictionary label, got %#v", notification)
	}

	select {
	case raw := <-sub.Messages:
		t.Fatalf("expected no realtime message for email ledger entry, got %s", raw)
	default:
	}
}

func TestListNotificationsFiltersByTypeEmailAndPhone(t *testing.T) {
	setupUsecaseOrderTxDB(t)
	appDB, err := db.DefaultManager.GetEngine("app")
	if err != nil {
		t.Fatalf("get app engine: %v", err)
	}

	insertTestNotification(t, appDB, "list-notification-1", "realtime", "sent", "ada@example.com", "13800000001", "2026-01-01 00:00:01")
	insertTestNotification(t, appDB, "list-notification-2", "sms", "skipped", "grace@example.com", "13800000002", "2026-01-01 00:00:02")
	insertTestNotification(t, appDB, "list-notification-3", "email", "skipped", "ada+alerts@example.com", "13900000003", "2026-01-01 00:00:03")

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	result, err := usecase.ListNotifications(ctx, usecase.NotificationsQry{
		Page:     1,
		PageSize: 10,
		Type:     "email",
		Email:    "ada",
		Phone:    "139",
	})
	if err != nil {
		t.Fatalf("list notifications: %v", err)
	}
	if len(result.Items) != 1 || result.Items[0].ID != "list-notification-3" {
		t.Fatalf("unexpected filtered notifications: %#v", result.Items)
	}
	if result.Pagination.TotalItems != 1 || result.Items[0].NotificationTypeLabel != "Email" {
		t.Fatalf("unexpected list metadata: %#v", result)
	}
}

func TestListNotificationsRejectsUnknownType(t *testing.T) {
	setupUsecaseOrderTxDB(t)
	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)

	_, err := usecase.ListNotifications(ctx, usecase.NotificationsQry{
		Page:     1,
		PageSize: 10,
		Type:     "push",
	})
	if fwusecase.CodeOf(err) != fwusecase.CodeValidation {
		t.Fatalf("expected validation error for unknown type, got %v", err)
	}
}

func TestClearMyNotificationsSoftClearsCurrentUserOnly(t *testing.T) {
	setupUsecaseOrderTxDB(t)
	appDB, err := db.DefaultManager.GetEngine("app")
	if err != nil {
		t.Fatalf("get app engine: %v", err)
	}

	insertUserNotification(t, appDB, "clear-notification-1", "u1")
	insertUserNotification(t, appDB, "clear-notification-2", "u1")
	insertUserNotification(t, appDB, "clear-notification-other", "u2")

	result, err := usecase.ClearMyNotifications(authenticatedUsecaseContext(t.Context(), "u1", false))
	if err != nil {
		t.Fatalf("clear my notifications: %v", err)
	}
	if result.ClearedCount != 2 {
		t.Fatalf("expected two notifications cleared, got %#v", result)
	}

	var visibleForU1 int
	if err := appDB.GetP(&visibleForU1, `SELECT COUNT(*) FROM notifications WHERE user_id = 'u1' AND cleared_at = ''`); err != nil {
		t.Fatalf("count visible u1 notifications: %v", err)
	}
	if visibleForU1 != 0 {
		t.Fatalf("expected u1 notifications hidden, got %d visible", visibleForU1)
	}

	var visibleForU2 int
	if err := appDB.GetP(&visibleForU2, `SELECT COUNT(*) FROM notifications WHERE user_id = 'u2' AND cleared_at = ''`); err != nil {
		t.Fatalf("count visible u2 notifications: %v", err)
	}
	if visibleForU2 != 1 {
		t.Fatalf("expected other user notification to remain visible, got %d", visibleForU2)
	}

	var totalRows int
	if err := appDB.GetP(&totalRows, `SELECT COUNT(*) FROM notifications`); err != nil {
		t.Fatalf("count notifications: %v", err)
	}
	if totalRows != 3 {
		t.Fatalf("expected soft clear to preserve rows, got %d", totalRows)
	}
}

func TestClearMyNotificationsRequiresUser(t *testing.T) {
	setupUsecaseOrderTxDB(t)
	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)

	_, err := usecase.ClearMyNotifications(ctx)
	if fwusecase.CodeOf(err) != fwusecase.CodeUnauthorized {
		t.Fatalf("expected unauthorized error, got %q: %v", fwusecase.CodeOf(err), err)
	}
}

func insertTestNotification(t *testing.T, appDB *sqlx.Engine, id string, notificationType string, status string, email string, phone string, createdAt string) {
	t.Helper()

	query := `
		INSERT INTO notifications (
			id, notification_type, source_type, source_id, user_id, recipient_email,
			recipient_phone, title, summary, payload_json, status, last_error,
			created_at, updated_at
		) VALUES (?, ?, '', '', '', ?, ?, 'Test notification', '', '{}', ?, '', ?, ?)
	`
	if _, err := appDB.ExecP(query, id, notificationType, email, phone, status, createdAt, createdAt); err != nil {
		t.Fatalf("insert test notification %s: %v", id, err)
	}
}

func insertUserNotification(t *testing.T, appDB *sqlx.Engine, id string, userID string) {
	t.Helper()

	query := `
		INSERT INTO notifications (
			id, notification_type, source_type, source_id, user_id, recipient_email,
			recipient_phone, title, summary, payload_json, status, last_error,
			created_at, updated_at
		) VALUES (?, 'realtime', '', '', ?, '', '', 'User notification', '', '{}', 'sent', '', '2026-01-01 00:00:00', '2026-01-01 00:00:00')
	`
	if _, err := appDB.ExecP(query, id, userID); err != nil {
		t.Fatalf("insert user notification %s: %v", id, err)
	}
}

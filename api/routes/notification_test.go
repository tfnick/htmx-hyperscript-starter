package routes_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/tfnick/go-svelte-starter/api/db"
	fwcontext "github.com/tfnick/go-svelte-starter/api/framework/http/context"
	"github.com/tfnick/go-svelte-starter/api/models"
	"github.com/tfnick/go-svelte-starter/api/routes"
	"github.com/tfnick/sqlx"
)

func TestListNotificationsReturnsPaginatedFilteredEnvelope(t *testing.T) {
	setupRouteTestDBs(t)
	appDB, err := db.DefaultManager.GetEngine("app")
	if err != nil {
		t.Fatalf("get app engine: %v", err)
	}
	seedRouteNotifications(t, appDB)

	router := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/notifications?type=sms&phone=138&page=1&page_size=2", nil)
	rec := httptest.NewRecorder()
	c := router.NewContext(req, rec)

	if err := routes.ListNotifications(c); err != nil {
		t.Fatalf("list notifications: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var envelope struct {
		Success bool                         `json:"success"`
		Data    routes.NotificationsResponse `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !envelope.Success {
		t.Fatalf("expected success envelope, got %s", rec.Body.String())
	}
	if len(envelope.Data.Items) != 2 {
		t.Fatalf("expected two notification items, got %#v", envelope.Data.Items)
	}
	if envelope.Data.Items[0].ID != "route-notification-03" || envelope.Data.Items[0].NotificationTypeLabel != "SMS" {
		t.Fatalf("unexpected first notification: %#v", envelope.Data.Items[0])
	}
	if envelope.Data.Pagination.TotalItems != 3 || envelope.Data.Pagination.TotalPages != 2 {
		t.Fatalf("unexpected pagination metadata: %#v", envelope.Data.Pagination)
	}
}

func TestListNotificationsRejectsInvalidType(t *testing.T) {
	setupRouteTestDBs(t)

	router := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/notifications?type=push", nil)
	rec := httptest.NewRecorder()
	c := router.NewContext(req, rec)

	if err := routes.ListNotifications(c); err != nil {
		t.Fatalf("list notifications: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	body := strings.TrimSpace(rec.Body.String())
	if !strings.Contains(body, `"success":false`) || !strings.Contains(body, `"notification type is invalid"`) {
		t.Fatalf("expected validation envelope, got %s", body)
	}
}

func TestClearMyNotificationsReturnsClearedCountEnvelope(t *testing.T) {
	setupRouteTestDBs(t)
	appDB, err := db.DefaultManager.GetEngine("app")
	if err != nil {
		t.Fatalf("get app engine: %v", err)
	}
	seedRouteNotificationForUser(t, appDB, "route-clear-notification-1", "u1")
	seedRouteNotificationForUser(t, appDB, "route-clear-notification-2", "u1")
	seedRouteNotificationForUser(t, appDB, "route-clear-notification-other", "u2")

	router := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/user/notifications/clear", nil)
	rec := httptest.NewRecorder()
	c := router.NewContext(req, rec)
	fwcontext.SetCurrentUser(c, &models.User{ID: "u1", Name: "Ada"})

	if err := routes.ClearMyNotifications(c); err != nil {
		t.Fatalf("clear my notifications: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var envelope struct {
		Success bool                              `json:"success"`
		Data    routes.ClearNotificationsResponse `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !envelope.Success || envelope.Data.ClearedCount != 2 {
		t.Fatalf("unexpected clear response: %s", rec.Body.String())
	}

	var otherVisible int
	if err := appDB.GetP(&otherVisible, `SELECT COUNT(*) FROM notifications WHERE user_id = 'u2' AND cleared_at = ''`); err != nil {
		t.Fatalf("count other notifications: %v", err)
	}
	if otherVisible != 1 {
		t.Fatalf("expected other user's notification to remain visible, got %d", otherVisible)
	}
}

func TestClearMyNotificationsRequiresCurrentUser(t *testing.T) {
	router := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/user/notifications/clear", nil)
	rec := httptest.NewRecorder()
	c := router.NewContext(req, rec)

	if err := routes.ClearMyNotifications(c); err != nil {
		t.Fatalf("clear my notifications: %v", err)
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusUnauthorized, rec.Code, rec.Body.String())
	}
}

func seedRouteNotifications(t *testing.T, appDB *sqlx.Engine) {
	t.Helper()

	query := `
		INSERT INTO notifications (
			id, notification_type, source_type, source_id, user_id, recipient_email,
			recipient_phone, title, summary, payload_json, status, last_error,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, '{}', ?, '', ?, ?)
	`
	rows := []struct {
		id               string
		notificationType string
		phone            string
		createdAt        string
	}{
		{"route-notification-01", "sms", "13800000001", "2026-01-01 00:00:01"},
		{"route-notification-02", "sms", "13800000002", "2026-01-01 00:00:02"},
		{"route-notification-03", "sms", "13800000003", "2026-01-01 00:00:03"},
		{"route-notification-04", "email", "13900000004", "2026-01-01 00:00:04"},
	}
	for i, row := range rows {
		_, err := appDB.ExecP(query,
			row.id,
			row.notificationType,
			"order",
			fmt.Sprintf("route-order-%02d", i+1),
			"route-user-1",
			fmt.Sprintf("route-%02d@example.com", i+1),
			row.phone,
			fmt.Sprintf("Route notification %02d", i+1),
			"Route summary",
			"skipped",
			row.createdAt,
			row.createdAt,
		)
		if err != nil {
			t.Fatalf("insert route notification %s: %v", row.id, err)
		}
	}
}

func seedRouteNotificationForUser(t *testing.T, appDB *sqlx.Engine, id string, userID string) {
	t.Helper()

	query := `
		INSERT INTO notifications (
			id, notification_type, source_type, source_id, user_id, recipient_email,
			recipient_phone, title, summary, payload_json, status, last_error,
			created_at, updated_at
		) VALUES (?, 'realtime', '', '', ?, '', '', 'Route user notification', '', '{}', 'sent', '', '2026-01-01 00:00:00', '2026-01-01 00:00:00')
	`
	if _, err := appDB.ExecP(query, id, userID); err != nil {
		t.Fatalf("insert route notification %s: %v", id, err)
	}
}

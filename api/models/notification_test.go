package models_test

import (
	"testing"

	"github.com/tfnick/go-svelte-starter/api/models"
)

func TestClearNotificationsByUserSoftClearsOnlyCurrentUser(t *testing.T) {
	manager := setupModelsTestDB(t)
	appDB, err := manager.GetDB("app")
	if err != nil {
		t.Fatalf("get app db: %v", err)
	}

	seedModelNotification(t, "notification-u1-a", "u1")
	seedModelNotification(t, "notification-u1-b", "u1")
	seedModelNotification(t, "notification-u2-a", "u2")

	cleared, err := models.ClearNotificationsByUser(t.Context(), "u1")
	if err != nil {
		t.Fatalf("clear notifications: %v", err)
	}
	if cleared != 2 {
		t.Fatalf("expected two notifications cleared, got %d", cleared)
	}

	var visibleForU1 int
	if err := appDB.Get(&visibleForU1, `SELECT COUNT(*) FROM notifications WHERE user_id = 'u1' AND cleared_at = ''`); err != nil {
		t.Fatalf("count visible u1 notifications: %v", err)
	}
	if visibleForU1 != 0 {
		t.Fatalf("expected u1 notifications to be hidden, got %d visible", visibleForU1)
	}

	var visibleForU2 int
	if err := appDB.Get(&visibleForU2, `SELECT COUNT(*) FROM notifications WHERE user_id = 'u2' AND cleared_at = ''`); err != nil {
		t.Fatalf("count visible u2 notifications: %v", err)
	}
	if visibleForU2 != 1 {
		t.Fatalf("expected u2 notification to remain visible, got %d", visibleForU2)
	}

	var totalRows int
	if err := appDB.Get(&totalRows, `SELECT COUNT(*) FROM notifications`); err != nil {
		t.Fatalf("count notification rows: %v", err)
	}
	if totalRows != 3 {
		t.Fatalf("expected clear to preserve ledger rows, got %d rows", totalRows)
	}

	clearedAgain, err := models.ClearNotificationsByUser(t.Context(), "u1")
	if err != nil {
		t.Fatalf("clear notifications again: %v", err)
	}
	if clearedAgain != 0 {
		t.Fatalf("expected already-cleared notifications not to be counted again, got %d", clearedAgain)
	}
}

func seedModelNotification(t *testing.T, id string, userID string) {
	t.Helper()

	if err := models.InsertNotification(t.Context(), &models.Notification{
		ID:               id,
		NotificationType: "realtime",
		UserID:           userID,
		Title:            "Test notification",
		Status:           models.NotificationStatusSent,
		PayloadJSON:      "{}",
	}); err != nil {
		t.Fatalf("insert notification %s: %v", id, err)
	}
}

package usecase_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/tfnick/go-svelte-starter/api/db"
	"github.com/tfnick/go-svelte-starter/api/framework/realtime"
	fwusecase "github.com/tfnick/go-svelte-starter/api/framework/usecase"
	"github.com/tfnick/go-svelte-starter/api/usecase"
)

func TestTriggerExportToastCreatesStoredNotificationAndPublishesRealtimeMessage(t *testing.T) {
	setupUsecaseOrderTxDB(t)
	sub := realtime.SubscribeClient("u1", "client-export-toast")
	defer sub.Close()

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	if err := usecase.TriggerExportToast(ctx, usecase.TriggerExportToastCmd{UserID: "u1"}); err != nil {
		t.Fatalf("trigger export toast: %v", err)
	}

	select {
	case raw := <-sub.Messages:
		var message struct {
			Type         string `json:"type"`
			Presentation string `json:"presentation"`
			Payload      struct {
				ID         string `json:"id"`
				Title      string `json:"title"`
				Summary    string `json:"summary"`
				SourceType string `json:"source_type"`
				SourceID   string `json:"source_id"`
				Status     string `json:"status"`
			} `json:"payload"`
		}
		if err := json.Unmarshal(raw, &message); err != nil {
			t.Fatalf("decode realtime message: %v", err)
		}
		if message.Type != "notification" {
			t.Fatalf("expected notification type, got %q", message.Type)
		}
		if message.Presentation != "toast" {
			t.Fatalf("expected toast presentation, got %q", message.Presentation)
		}
		if message.Payload.ID == "" {
			t.Fatalf("expected notification id")
		}
		if message.Payload.Title != "Export completed" || message.Payload.Summary != "Export completed" {
			t.Fatalf("unexpected notification payload: %#v", message.Payload)
		}
		if message.Payload.SourceType != "experiment" || message.Payload.SourceID != "export-toast" || message.Payload.Status != "completed" {
			t.Fatalf("unexpected notification source/status: %#v", message.Payload)
		}
	case <-time.After(time.Second):
		t.Fatalf("expected export toast realtime message")
	}

	appDB, err := db.DefaultManager.GetDB("app")
	if err != nil {
		t.Fatalf("get app db: %v", err)
	}
	var count int
	if err := appDB.Get(&count, `SELECT COUNT(*) FROM notifications WHERE user_id = ? AND source_type = 'experiment'`, "u1"); err != nil {
		t.Fatalf("count notifications: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected stored experiment notification, got %d", count)
	}
}

func TestSendNotificationTransientPublishesWithoutLedger(t *testing.T) {
	setupUsecaseOrderTxDB(t)
	sub := realtime.SubscribeClient("u1", "client-points-refresh")
	defer sub.Close()

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceSystem)
	_, err := usecase.SendNotification(ctx, usecase.SendNotificationCmd{
		StorePolicy:  usecase.StorePolicyTransient,
		MessageType:  usecase.RealtimeMessageTypePoints,
		Presentation: usecase.RealtimePresentationRefresh,
		UserID:       "u1",
		Payload: map[string]interface{}{
			"user_id": "u1",
			"balance": int64(20),
		},
	})
	if err != nil {
		t.Fatalf("send transient notification: %v", err)
	}

	select {
	case raw := <-sub.Messages:
		var message struct {
			Type         string `json:"type"`
			Presentation string `json:"presentation"`
			Payload      struct {
				UserID  string `json:"user_id"`
				Balance int64  `json:"balance"`
			} `json:"payload"`
		}
		if err := json.Unmarshal(raw, &message); err != nil {
			t.Fatalf("decode realtime message: %v", err)
		}
		if message.Type != "points" || message.Presentation != "refresh" || message.Payload.UserID != "u1" || message.Payload.Balance != 20 {
			t.Fatalf("unexpected transient message: %#v", message)
		}
	case <-time.After(time.Second):
		t.Fatalf("expected transient realtime message")
	}

	appDB, err := db.DefaultManager.GetDB("app")
	if err != nil {
		t.Fatalf("get app db: %v", err)
	}
	var count int
	if err := appDB.Get(&count, `SELECT COUNT(*) FROM notifications`); err != nil {
		t.Fatalf("count notifications: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected transient message not to store notification, got %d", count)
	}
}

func TestTriggerExportToastValidatesUserID(t *testing.T) {
	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	err := usecase.TriggerExportToast(ctx, usecase.TriggerExportToastCmd{})
	if err == nil {
		t.Fatalf("expected validation error")
	}
	if fwusecase.CodeOf(err) != fwusecase.CodeValidation {
		t.Fatalf("expected validation code, got %q", fwusecase.CodeOf(err))
	}
}

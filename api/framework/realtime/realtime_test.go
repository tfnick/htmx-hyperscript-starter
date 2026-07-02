package realtime

import (
	"testing"
	"time"
)

func TestHubPublishesOnlyToUserSubscribers(t *testing.T) {
	hub := NewHub()
	ada := hub.SubscribeClient("u1", "client-1")
	defer ada.Close()
	grace := hub.SubscribeClient("u2", "client-2")
	defer grace.Close()

	if ada.ClientID() != "client-1" || ada.UserID() != "u1" {
		t.Fatalf("expected explicit client identity")
	}

	if err := hub.Publish("u1", map[string]interface{}{"type": "raw.event", "balance": 10}); err != nil {
		t.Fatalf("publish: %v", err)
	}

	select {
	case msg := <-ada.Messages:
		if string(msg) != `{"balance":10,"type":"raw.event"}` {
			t.Fatalf("unexpected payload: %s", msg)
		}
	case <-time.After(time.Second):
		t.Fatalf("expected user subscriber message")
	}

	select {
	case msg := <-grace.Messages:
		t.Fatalf("unexpected other user message: %s", msg)
	default:
	}
}

func TestHubPublishesToSingleClient(t *testing.T) {
	hub := NewHub()
	left := hub.SubscribeClient("u1", "left")
	defer left.Close()
	right := hub.SubscribeClient("u1", "right")
	defer right.Close()

	if err := hub.PublishClient("right", map[string]interface{}{"type": "client.refresh"}); err != nil {
		t.Fatalf("publish client: %v", err)
	}

	select {
	case msg := <-right.Messages:
		if string(msg) != `{"type":"client.refresh"}` {
			t.Fatalf("unexpected payload: %s", msg)
		}
	case <-time.After(time.Second):
		t.Fatalf("expected target client message")
	}

	select {
	case msg := <-left.Messages:
		t.Fatalf("unexpected other client message: %s", msg)
	default:
	}
}

func TestMessageDefaultsPresentation(t *testing.T) {
	tests := []struct {
		name         string
		messageType  MessageType
		presentation Presentation
		expected     Presentation
	}{
		{
			name:        "points",
			messageType: MessageTypePoints,
			expected:    PresentationRefresh,
		},
		{
			name:        "async export task",
			messageType: MessageTypeAsyncExportTask,
			expected:    PresentationToast,
		},
		{
			name:        "notification",
			messageType: MessageTypeNotification,
			expected:    PresentationToast,
		},
		{
			name:         "explicit presentation wins",
			messageType:  MessageTypePoints,
			presentation: PresentationToast,
			expected:     PresentationToast,
		},
		{
			name:        "unknown defaults to refresh",
			messageType: MessageType("unknown"),
			expected:    PresentationRefresh,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			message := NewMessage(tt.messageType, tt.presentation, nil)
			if message.Presentation != tt.expected {
				t.Fatalf("expected presentation %q, got %q", tt.expected, message.Presentation)
			}
		})
	}
}

func TestHubPublishesRealtimeEnvelope(t *testing.T) {
	hub := NewHub()
	sub := hub.SubscribeClient("u1", "client-1")
	defer sub.Close()

	err := hub.Publish("u1", NewPointsMessage(PointsPayload{
		UserID:   "u1",
		ClientID: "client-1",
		Balance:  10,
	}, ""))
	if err != nil {
		t.Fatalf("publish realtime envelope: %v", err)
	}

	select {
	case msg := <-sub.Messages:
		expected := `{"type":"points","presentation":"refresh","payload":{"user_id":"u1","client_id":"client-1","balance":10}}`
		if string(msg) != expected {
			t.Fatalf("unexpected payload: %s", msg)
		}
	case <-time.After(time.Second):
		t.Fatalf("expected realtime envelope message")
	}
}

func TestAsyncExportTaskMessageDefaultsToToast(t *testing.T) {
	message := NewAsyncExportTaskMessage(AsyncExportTaskPayload{
		TaskID:  "export-1",
		Status:  "completed",
		Message: "Export completed",
	}, "")

	if message.Presentation != PresentationToast {
		t.Fatalf("expected async export task to default to toast, got %q", message.Presentation)
	}
}

func TestNotificationMessageDefaultsToToast(t *testing.T) {
	message := NewNotificationMessage(NotificationPayload{
		ID:      "notification-1",
		Title:   "Order paid",
		Summary: "Points awarded",
	}, "")

	if message.Presentation != PresentationToast {
		t.Fatalf("expected notification to default to toast, got %q", message.Presentation)
	}
}

func TestHubGeneratesClientIDWhenMissing(t *testing.T) {
	hub := NewHub()
	sub := hub.Subscribe("u1")
	defer sub.Close()

	if sub.ClientID() == "" {
		t.Fatalf("expected generated client ID")
	}
}

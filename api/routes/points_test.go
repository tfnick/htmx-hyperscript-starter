package routes_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/labstack/echo/v4"
	fwcontext "github.com/tfnick/go-svelte-starter/api/framework/http/context"
	"github.com/tfnick/go-svelte-starter/api/framework/realtime"
	"github.com/tfnick/go-svelte-starter/api/models"
	"github.com/tfnick/go-svelte-starter/api/routes"
)

func TestUserRealtimeWebSocketStreamsInitialPointsMessage(t *testing.T) {
	setupRouteTestDBs(t)

	server := newUserRealtimeTestServer(&models.User{ID: "u1", Name: "Ada"})
	defer server.Close()

	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, strings.Replace(server.URL, "http://", "ws://", 1)+"/api/user/realtime/ws?client_id=route-realtime-client", nil)
	if err != nil {
		t.Fatalf("dial realtime websocket: %v", err)
	}
	defer conn.CloseNow()

	message := readRealtimeTestMessage(t, ctx, conn)
	if message.Type != "points" || message.Presentation != "refresh" {
		t.Fatalf("unexpected realtime envelope: %#v", message)
	}
	if message.Payload["user_id"] != "u1" || message.Payload["client_id"] != "route-realtime-client" || message.Payload["balance"] != float64(0) {
		t.Fatalf("unexpected points payload: %#v", message.Payload)
	}
}

func TestUserRealtimeWebSocketReceivesOnlyCurrentUserMessages(t *testing.T) {
	setupRouteTestDBs(t)

	server := newUserRealtimeTestServer(&models.User{ID: "u1", Name: "Ada"})
	defer server.Close()

	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, strings.Replace(server.URL, "http://", "ws://", 1)+"/api/user/realtime/ws?client_id=route-realtime-target", nil)
	if err != nil {
		t.Fatalf("dial realtime websocket: %v", err)
	}
	defer conn.CloseNow()

	_ = readRealtimeTestMessage(t, ctx, conn)
	if err := realtime.Publish("u2", realtime.NewNotificationMessage(realtime.NotificationPayload{
		ID:    "other-notification",
		Title: "Other user",
	}, "")); err != nil {
		t.Fatalf("publish other user message: %v", err)
	}
	if err := realtime.Publish("u1", realtime.NewNotificationMessage(realtime.NotificationPayload{
		ID:    "current-notification",
		Title: "Current user",
	}, "")); err != nil {
		t.Fatalf("publish current user message: %v", err)
	}

	message := readRealtimeTestMessage(t, ctx, conn)
	if message.Type != "notification" || message.Payload["id"] != "current-notification" {
		t.Fatalf("expected current-user notification, got %#v", message)
	}
}

func TestUserRealtimeWebSocketRequiresCurrentUser(t *testing.T) {
	router := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/user/realtime/ws", nil)
	rec := httptest.NewRecorder()
	c := router.NewContext(req, rec)

	if err := routes.UserRealtimeWebSocket(c); err != nil {
		t.Fatalf("realtime websocket: %v", err)
	}

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"message":"not logged in"`) {
		t.Fatalf("expected unauthorized envelope, got %s", rec.Body.String())
	}
}

func TestUserRealtimeWebSocketUsesConfiguredOriginAllowlist(t *testing.T) {
	setupRouteTestDBs(t)
	t.Setenv(realtime.EnvWebSocketOriginPatterns, "https://allowed.example.com")

	server := newUserRealtimeTestServer(&models.User{ID: "u1", Name: "Ada"})
	defer server.Close()

	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()

	deniedHeader := http.Header{}
	deniedHeader.Set("Origin", "https://denied.example.com")
	if conn, resp, err := websocket.Dial(ctx, realtimeTestWSURL(server.URL, "/api/user/realtime/ws"), &websocket.DialOptions{HTTPHeader: deniedHeader}); err == nil {
		conn.CloseNow()
		t.Fatalf("expected denied origin to fail")
	} else if resp == nil || resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected denied origin status 403, got resp=%v err=%v", resp, err)
	}

	allowedHeader := http.Header{}
	allowedHeader.Set("Origin", "https://allowed.example.com")
	conn, _, err := websocket.Dial(ctx, realtimeTestWSURL(server.URL, "/api/user/realtime/ws"), &websocket.DialOptions{HTTPHeader: allowedHeader})
	if err != nil {
		t.Fatalf("expected allowed origin to connect: %v", err)
	}
	defer conn.CloseNow()
}

func newUserRealtimeTestServer(user *models.User) *httptest.Server {
	router := echo.New()
	router.GET("/api/user/realtime/ws", func(c echo.Context) error {
		fwcontext.SetCurrentUser(c, user)
		return routes.UserRealtimeWebSocket(c)
	})
	return httptest.NewServer(router)
}

func realtimeTestWSURL(serverURL string, path string) string {
	return strings.Replace(serverURL, "http://", "ws://", 1) + path
}

type realtimeTestMessage struct {
	Type         string                 `json:"type"`
	Presentation string                 `json:"presentation"`
	Payload      map[string]interface{} `json:"payload"`
}

func readRealtimeTestMessage(t *testing.T, ctx context.Context, conn *websocket.Conn) realtimeTestMessage {
	t.Helper()

	typ, reader, err := conn.Reader(ctx)
	if err != nil {
		t.Fatalf("read websocket message: %v", err)
	}
	if typ != websocket.MessageText {
		t.Fatalf("expected text message, got %v", typ)
	}

	var message realtimeTestMessage
	if err := json.NewDecoder(reader).Decode(&message); err != nil {
		t.Fatalf("decode websocket message: %v", err)
	}
	return message
}

package routes_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/tfnick/go-svelte-starter/api/db"
	"github.com/tfnick/go-svelte-starter/api/framework/queue"
	"github.com/tfnick/go-svelte-starter/api/routes"
	"github.com/tfnick/go-svelte-starter/api/usecase"
)

func TestGetQueueSummaryReturnsQueueMetricsEnvelope(t *testing.T) {
	setupRouteTestDBs(t)
	now := time.Now().UTC()
	seedRouteQueueMessage(t, "route-ready-msg", queue.QueueHeavyTasks, now.Add(-10*time.Minute), now.Add(-1*time.Minute), 0)
	seedRouteQueueMessage(t, "route-dead-msg", queue.QueueHeavyTasks, now.Add(-5*time.Minute), now.Add(-1*time.Minute), queue.DefaultMaxReceive)

	router := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/admin/messages/summary?queue="+queue.QueueHeavyTasks, nil)
	rec := httptest.NewRecorder()
	c := router.NewContext(req, rec)

	if err := routes.GetQueueSummary(c); err != nil {
		t.Fatalf("get queue summary: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var envelope struct {
		Success bool                        `json:"success"`
		Data    routes.QueueSummaryResponse `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !envelope.Success || len(envelope.Data.Queues) != 1 {
		t.Fatalf("unexpected summary envelope: %s", rec.Body.String())
	}
	item := envelope.Data.Queues[0]
	if item.Queue != queue.QueueHeavyTasks || item.Total != 2 || item.Ready != 1 || item.Exhausted != 1 {
		t.Fatalf("unexpected queue summary item: %#v", item)
	}
	if item.OldestExhaustedAgeSec <= 0 {
		t.Fatalf("expected exhausted age metric, got %#v", item)
	}
}

func TestListMessagesReturnsOperationalStateEnvelope(t *testing.T) {
	setupRouteTestDBs(t)
	now := time.Now().UTC()
	seedRouteQueueMessage(t, "route-ready-msg", queue.QueueHeavyTasks, now.Add(-10*time.Minute), now.Add(-1*time.Minute), 0)
	seedRouteQueueMessage(t, "route-dead-msg", queue.QueueHeavyTasks, now.Add(-5*time.Minute), now.Add(-1*time.Minute), queue.DefaultMaxReceive)

	router := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/admin/messages?queue="+queue.QueueHeavyTasks+"&state=exhausted", nil)
	rec := httptest.NewRecorder()
	c := router.NewContext(req, rec)

	if err := routes.ListMessages(c); err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var envelope struct {
		Success bool                          `json:"success"`
		Data    []routes.QueueMessageResponse `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !envelope.Success || len(envelope.Data) != 1 {
		t.Fatalf("unexpected messages envelope: %s", rec.Body.String())
	}
	message := envelope.Data[0]
	if message.ID != "route-dead-msg" || message.State != "exhausted" || message.MaxReceive != queue.DefaultMaxReceive {
		t.Fatalf("unexpected queue message response: %#v", message)
	}
	if message.AgeSec <= 0 {
		t.Fatalf("expected message age metric, got %#v", message)
	}
}

func TestGetQueueMessageCorrelationReturnsReferencesEnvelope(t *testing.T) {
	setupRouteTestDBs(t)
	appDB, err := db.DefaultManager.GetEngine("app")
	if err != nil {
		t.Fatalf("get app engine: %v", err)
	}
	if _, err := appDB.ExecP(`
		INSERT INTO async_tasks (
			id, user_id, task_type, status, payload_json, message_id
		) VALUES ('route-corr-task', 'u1', 'test_export', 'queued', '{}', 'route-corr-message')
	`); err != nil {
		t.Fatalf("insert async task: %v", err)
	}

	router := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/admin/messages/route-corr-message/correlation", nil)
	rec := httptest.NewRecorder()
	c := router.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("route-corr-message")

	if err := routes.GetQueueMessageCorrelation(c); err != nil {
		t.Fatalf("get queue message correlation: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var envelope struct {
		Success bool                                   `json:"success"`
		Data    routes.QueueMessageCorrelationResponse `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !envelope.Success || envelope.Data.MessageID != "route-corr-message" || len(envelope.Data.References) != 1 {
		t.Fatalf("unexpected correlation envelope: %s", rec.Body.String())
	}
	ref := envelope.Data.References[0]
	if ref.Type != "async_task" || ref.ID != "route-corr-task" || ref.Status != "queued" || ref.RelatedType != "task_type" || ref.RelatedID != "test_export" {
		t.Fatalf("unexpected correlation reference: %#v", ref)
	}
}

func TestRetryQueueMessageNowReturnsMessageEnvelope(t *testing.T) {
	setupRouteTestDBs(t)
	queueManager, err := queue.NewManager()
	if err != nil {
		t.Fatalf("new queue manager: %v", err)
	}
	previousQueue := usecase.DefaultQueueManager
	usecase.DefaultQueueManager = queueManager
	t.Cleanup(func() {
		usecase.DefaultQueueManager = previousQueue
	})

	now := time.Now().UTC()
	seedRouteQueueMessage(t, "route-retry-dead", queue.QueueHeavyTasks, now.Add(-10*time.Minute), now.Add(-1*time.Minute), queue.DefaultMaxReceive)

	router := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/admin/messages/route-retry-dead/retry-now?queue="+queue.QueueHeavyTasks, nil)
	rec := httptest.NewRecorder()
	c := router.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("route-retry-dead")

	if err := routes.RetryQueueMessageNow(c); err != nil {
		t.Fatalf("retry queue message: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var envelope struct {
		Success bool `json:"success"`
		Data    struct {
			Message string `json:"message"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !envelope.Success || envelope.Data.Message != "queue message retried" {
		t.Fatalf("unexpected retry envelope: %s", rec.Body.String())
	}
}

func TestRetryQueueMessageNowMapsNonExhaustedConflict(t *testing.T) {
	setupRouteTestDBs(t)
	queueManager, err := queue.NewManager()
	if err != nil {
		t.Fatalf("new queue manager: %v", err)
	}
	previousQueue := usecase.DefaultQueueManager
	usecase.DefaultQueueManager = queueManager
	t.Cleanup(func() {
		usecase.DefaultQueueManager = previousQueue
	})

	now := time.Now().UTC()
	seedRouteQueueMessage(t, "route-retry-ready", queue.QueueHeavyTasks, now.Add(-10*time.Minute), now.Add(-1*time.Minute), 0)

	router := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/admin/messages/route-retry-ready/retry-now?queue="+queue.QueueHeavyTasks, nil)
	rec := httptest.NewRecorder()
	c := router.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("route-retry-ready")

	if err := routes.RetryQueueMessageNow(c); err != nil {
		t.Fatalf("retry queue message: %v", err)
	}
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusConflict, rec.Code, rec.Body.String())
	}
}

func TestDeleteQueueMessageReturnsMessageEnvelope(t *testing.T) {
	setupRouteTestDBs(t)
	queueManager, err := queue.NewManager()
	if err != nil {
		t.Fatalf("new queue manager: %v", err)
	}
	previousQueue := usecase.DefaultQueueManager
	usecase.DefaultQueueManager = queueManager
	t.Cleanup(func() {
		usecase.DefaultQueueManager = previousQueue
	})

	now := time.Now().UTC()
	seedRouteQueueMessage(t, "route-delete-message", queue.QueueHeavyTasks, now.Add(-10*time.Minute), now.Add(-1*time.Minute), queue.DefaultMaxReceive)

	router := echo.New()
	req := httptest.NewRequest(http.MethodDelete, "/api/admin/messages/route-delete-message?queue="+queue.QueueHeavyTasks, nil)
	rec := httptest.NewRecorder()
	c := router.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("route-delete-message")

	if err := routes.DeleteQueueMessage(c); err != nil {
		t.Fatalf("delete queue message: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var envelope struct {
		Success bool `json:"success"`
		Data    struct {
			Message string `json:"message"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !envelope.Success || envelope.Data.Message != "queue message deleted" {
		t.Fatalf("unexpected delete envelope: %s", rec.Body.String())
	}
}

func TestDeleteQueueMessageMapsMissingMessageNotFound(t *testing.T) {
	setupRouteTestDBs(t)
	queueManager, err := queue.NewManager()
	if err != nil {
		t.Fatalf("new queue manager: %v", err)
	}
	previousQueue := usecase.DefaultQueueManager
	usecase.DefaultQueueManager = queueManager
	t.Cleanup(func() {
		usecase.DefaultQueueManager = previousQueue
	})

	router := echo.New()
	req := httptest.NewRequest(http.MethodDelete, "/api/admin/messages/missing-message?queue="+queue.QueueHeavyTasks, nil)
	rec := httptest.NewRecorder()
	c := router.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("missing-message")

	if err := routes.DeleteQueueMessage(c); err != nil {
		t.Fatalf("delete queue message: %v", err)
	}
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusNotFound, rec.Code, rec.Body.String())
	}
}

func seedRouteQueueMessage(t *testing.T, id string, queueName string, created time.Time, timeoutAt time.Time, received int) {
	t.Helper()

	appDB, err := db.DefaultManager.GetEngine("app")
	if err != nil {
		t.Fatalf("get app engine: %v", err)
	}
	if _, err := appDB.ExecP(`
		INSERT INTO goqite (id, queue, body, created, updated, timeout, received, priority)
		VALUES (?, ?, CAST(? AS BLOB), ?, ?, ?, ?, 0)
	`, id, queueName, `{"id":"`+id+`"}`, formatRouteGoqiteTestTime(created), formatRouteGoqiteTestTime(created), formatRouteGoqiteTestTime(timeoutAt), received); err != nil {
		t.Fatalf("insert goqite message %s: %v", id, err)
	}
}

func formatRouteGoqiteTestTime(value time.Time) string {
	return value.UTC().Format("2006-01-02T15:04:05.000Z07:00")
}

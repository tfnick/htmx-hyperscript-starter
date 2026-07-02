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
	"github.com/tfnick/go-svelte-starter/api/routes"
	"github.com/tfnick/sqlx"
)

func TestListScheduledTaskHistoryReturnsPaginatedEnvelope(t *testing.T) {
	setupRouteTestDBs(t)
	appDB, err := db.DefaultManager.GetEngine("app")
	if err != nil {
		t.Fatalf("get app engine: %v", err)
	}
	seedRouteScheduledTaskExecutions(t, appDB, "route-task-1", 5)

	router := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/admin/scheduler/tasks/route-task-1/history?page=2&page_size=2", nil)
	rec := httptest.NewRecorder()
	c := router.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("route-task-1")

	if err := routes.ListScheduledTaskHistory(c); err != nil {
		t.Fatalf("list scheduled task history: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var envelope struct {
		Success bool                                   `json:"success"`
		Data    routes.ScheduledTaskExecutionsResponse `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !envelope.Success {
		t.Fatalf("expected success envelope, got %s", rec.Body.String())
	}
	if len(envelope.Data.Items) != 2 {
		t.Fatalf("expected two history items, got %#v", envelope.Data.Items)
	}
	if envelope.Data.Items[0].ID != "route-execution-03" || envelope.Data.Items[1].ID != "route-execution-02" {
		t.Fatalf("expected stable page items, got %#v", envelope.Data.Items)
	}
	if envelope.Data.Pagination.TotalItems != 5 || envelope.Data.Pagination.TotalPages != 3 {
		t.Fatalf("unexpected pagination metadata: %#v", envelope.Data.Pagination)
	}
}

func TestListScheduledTaskHistoryRejectsInvalidPageQuery(t *testing.T) {
	setupRouteTestDBs(t)

	router := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/admin/scheduler/tasks/route-task-1/history?page=0&page_size=10", nil)
	rec := httptest.NewRecorder()
	c := router.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("route-task-1")

	if err := routes.ListScheduledTaskHistory(c); err != nil {
		t.Fatalf("list scheduled task history: %v", err)
	}

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}
	body := strings.TrimSpace(rec.Body.String())
	if !strings.Contains(body, `"success":false`) || !strings.Contains(body, `"code":"validation"`) {
		t.Fatalf("expected validation envelope, got %s", body)
	}
}

func seedRouteScheduledTaskExecutions(t *testing.T, appDB *sqlx.Engine, taskID string, count int) {
	t.Helper()

	_, err := appDB.ExecP(`
		INSERT INTO scheduled_tasks (
			id, name, job_name, schedule_type, schedule_value, payload_json, enabled, created_at, updated_at
		) VALUES (?, 'Route task', 'scheduler.noop', 'cron', '*/5 * * * *', '{}', 1, '2026-01-01 00:00:00', '2026-01-01 00:00:00')
	`, taskID)
	if err != nil {
		t.Fatalf("insert scheduled task: %v", err)
	}

	query := `
		INSERT INTO scheduled_task_executions (
			id, task_id, job_name, message_id, status, scheduled_at, started_at, finished_at, error_message, created_at
		) VALUES (?, ?, 'scheduler.noop', ?, 'succeeded', ?, ?, ?, '', ?)
	`
	for i := 1; i <= count; i++ {
		createdAt := fmt.Sprintf("2026-01-01 00:00:%02d", i)
		_, err := appDB.ExecP(query,
			fmt.Sprintf("route-execution-%02d", i),
			taskID,
			fmt.Sprintf("route-message-%02d", i),
			createdAt,
			createdAt,
			createdAt,
			createdAt,
		)
		if err != nil {
			t.Fatalf("insert scheduled task execution %d: %v", i, err)
		}
	}
}

package routes_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	fwcontext "github.com/tfnick/go-svelte-starter/api/framework/http/context"
	"github.com/tfnick/go-svelte-starter/api/models"
	"github.com/tfnick/go-svelte-starter/api/routes"
	"github.com/tfnick/go-svelte-starter/api/usecase"
)

func TestClearMyTasksReturnsClearedCountEnvelope(t *testing.T) {
	setupRouteTestDBs(t)
	seedRouteAsyncTask(t, "route-completed", "u1", models.AsyncTaskStatusCompleted)
	seedRouteAsyncTask(t, "route-processing", "u1", models.AsyncTaskStatusProcessing)

	router := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/user/tasks/clear", nil)
	rec := httptest.NewRecorder()
	c := router.NewContext(req, rec)
	fwcontext.SetCurrentUser(c, &models.User{ID: "u1", Name: "Ada"})

	if err := routes.ClearMyTasks(c); err != nil {
		t.Fatalf("clear my tasks: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var envelope struct {
		Success bool                      `json:"success"`
		Data    routes.ClearTasksResponse `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !envelope.Success || envelope.Data.ClearedCount != 1 {
		t.Fatalf("unexpected clear response: %s", rec.Body.String())
	}
}

func TestClearMyTasksRequiresCurrentUser(t *testing.T) {
	router := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/user/tasks/clear", nil)
	rec := httptest.NewRecorder()
	c := router.NewContext(req, rec)

	if err := routes.ClearMyTasks(c); err != nil {
		t.Fatalf("clear my tasks: %v", err)
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusUnauthorized, rec.Code, rec.Body.String())
	}
}

func TestEnqueueTaskRejectsUnknownTaskType(t *testing.T) {
	router := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/user/tasks", strings.NewReader(`{"task_type":"unknown_task","payload_json":"{}"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := router.NewContext(req, rec)
	fwcontext.SetCurrentUser(c, &models.User{ID: "u1", Name: "Ada"})

	if err := routes.EnqueueTask(c); err != nil {
		t.Fatalf("enqueue task: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"message":"task type is not allowed"`) {
		t.Fatalf("expected task type validation envelope, got %s", rec.Body.String())
	}
}

func TestEnqueueTaskRejectsOrderExportTaskType(t *testing.T) {
	router := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/user/tasks", strings.NewReader(`{"task_type":"`+usecase.HeavyTaskTypeOrdersExcelExport+`","payload_json":"{}"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := router.NewContext(req, rec)
	fwcontext.SetCurrentUser(c, &models.User{ID: "u1", Name: "Ada"})

	if err := routes.EnqueueTask(c); err != nil {
		t.Fatalf("enqueue task: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}
}

func seedRouteAsyncTask(t *testing.T, id string, userID string, status string) {
	t.Helper()

	if err := models.InsertAsyncTask(t.Context(), &models.AsyncTask{
		ID:          id,
		UserID:      userID,
		TaskType:    "test_export",
		Status:      status,
		PayloadJSON: "{}",
		ResultJSON:  "{}",
	}); err != nil {
		t.Fatalf("insert route async task %s: %v", id, err)
	}
}

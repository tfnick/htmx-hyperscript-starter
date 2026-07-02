package models_test

import (
	"testing"

	"github.com/tfnick/go-svelte-starter/api/models"
)

func TestClearTerminalAsyncTasksByUserOnlyClearsCompletedAndFailed(t *testing.T) {
	manager := setupModelsTestDB(t)
	appDB, err := manager.GetDB("app")
	if err != nil {
		t.Fatalf("get app db: %v", err)
	}

	seedModelAsyncTask(t, "task-completed", "u1", models.AsyncTaskStatusCompleted)
	seedModelAsyncTask(t, "task-failed", "u1", models.AsyncTaskStatusFailed)
	seedModelAsyncTask(t, "task-queued", "u1", models.AsyncTaskStatusQueued)
	seedModelAsyncTask(t, "task-processing", "u1", models.AsyncTaskStatusProcessing)
	seedModelAsyncTask(t, "task-other", "u2", models.AsyncTaskStatusCompleted)

	cleared, err := models.ClearTerminalAsyncTasksByUser(t.Context(), "u1")
	if err != nil {
		t.Fatalf("clear async tasks: %v", err)
	}
	if cleared != 2 {
		t.Fatalf("expected two terminal tasks cleared, got %d", cleared)
	}

	visible, err := models.ListAsyncTasksByUser(t.Context(), "u1", 10, 0)
	if err != nil {
		t.Fatalf("list visible async tasks: %v", err)
	}
	if len(visible) != 2 {
		t.Fatalf("expected queued and processing tasks to remain visible, got %#v", visible)
	}
	for _, task := range visible {
		if task.Status != models.AsyncTaskStatusQueued && task.Status != models.AsyncTaskStatusProcessing {
			t.Fatalf("expected only non-terminal visible tasks, got %#v", task)
		}
	}

	total, err := models.CountAsyncTasksByUser(t.Context(), "u1")
	if err != nil {
		t.Fatalf("count visible async tasks: %v", err)
	}
	if total != 2 {
		t.Fatalf("expected visible count 2, got %d", total)
	}

	var otherClearedAt string
	if err := appDB.Get(&otherClearedAt, `SELECT cleared_at FROM async_tasks WHERE id = 'task-other'`); err != nil {
		t.Fatalf("load other task cleared_at: %v", err)
	}
	if otherClearedAt != "" {
		t.Fatalf("expected other user's task to stay visible, got cleared_at=%q", otherClearedAt)
	}
}

func seedModelAsyncTask(t *testing.T, id string, userID string, status string) {
	t.Helper()

	if err := models.InsertAsyncTask(t.Context(), &models.AsyncTask{
		ID:          id,
		UserID:      userID,
		TaskType:    "test_export",
		Status:      status,
		PayloadJSON: "{}",
		ResultJSON:  "{}",
	}); err != nil {
		t.Fatalf("insert async task %s: %v", id, err)
	}
}

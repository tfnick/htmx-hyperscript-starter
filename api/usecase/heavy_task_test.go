package usecase_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/tfnick/go-svelte-starter/api/db"
	"github.com/tfnick/go-svelte-starter/api/framework/queue"
	fwusecase "github.com/tfnick/go-svelte-starter/api/framework/usecase"
	"github.com/tfnick/go-svelte-starter/api/models"
	"github.com/tfnick/go-svelte-starter/api/usecase"
)

func TestClearMyTasksClearsOnlyCurrentUsersTerminalTasks(t *testing.T) {
	setupUsecaseOrderTxDB(t)

	seedUsecaseAsyncTask(t, "task-completed", "u1", models.AsyncTaskStatusCompleted)
	seedUsecaseAsyncTask(t, "task-failed", "u1", models.AsyncTaskStatusFailed)
	seedUsecaseAsyncTask(t, "task-processing", "u1", models.AsyncTaskStatusProcessing)
	seedUsecaseAsyncTask(t, "task-other", "u2", models.AsyncTaskStatusCompleted)

	ctx := authenticatedUsecaseContext(t.Context(), "u1", false)
	result, err := usecase.ClearMyTasks(ctx)
	if err != nil {
		t.Fatalf("clear my tasks: %v", err)
	}
	if result.ClearedCount != 2 {
		t.Fatalf("expected two tasks cleared, got %#v", result)
	}

	tasks, err := usecase.ListMyTasks(ctx, usecase.ListMyTasksQry{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("list my tasks: %v", err)
	}
	if len(tasks.Items) != 1 || tasks.Items[0].ID != "task-processing" {
		t.Fatalf("expected only processing task visible, got %#v", tasks.Items)
	}

	otherTasks, err := usecase.ListMyTasks(authenticatedUsecaseContext(t.Context(), "u2", false), usecase.ListMyTasksQry{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("list other tasks: %v", err)
	}
	if len(otherTasks.Items) != 1 || otherTasks.Items[0].ID != "task-other" {
		t.Fatalf("expected other user's terminal task to remain visible, got %#v", otherTasks.Items)
	}
}

func TestEnqueueHeavyTaskStoresQueueMessageID(t *testing.T) {
	setupUsecaseOrderTxDB(t)
	queueManager, err := queue.NewManager()
	if err != nil {
		t.Fatalf("new queue manager: %v", err)
	}
	previousQueue := usecase.DefaultQueueManager
	usecase.DefaultQueueManager = queueManager
	t.Cleanup(func() {
		usecase.DefaultQueueManager = previousQueue
	})

	ctx := authenticatedUsecaseContext(t.Context(), "u1", false)
	result, err := usecase.EnqueueHeavyTask(ctx, usecase.EnqueueHeavyTaskCmd{
		UserID:      "u1",
		TaskType:    usecase.HeavyTaskTypeTestExport,
		PayloadJSON: "{}",
	})
	if err != nil {
		t.Fatalf("enqueue heavy task: %v", err)
	}

	appDB, err := db.DefaultManager.GetDB("app")
	if err != nil {
		t.Fatalf("get app db: %v", err)
	}
	var task models.AsyncTask
	if err := appDB.Get(&task, `SELECT * FROM async_tasks WHERE id = ?`, result.TaskID); err != nil {
		t.Fatalf("get async task: %v", err)
	}
	if task.MessageID == "" {
		t.Fatalf("expected async task message_id to be stored")
	}

	var body string
	if err := appDB.Get(&body, `SELECT CAST(body AS TEXT) FROM goqite WHERE id = ? AND queue = ?`, task.MessageID, queue.QueueHeavyTasks); err != nil {
		t.Fatalf("get heavy task queue message: %v", err)
	}
	var message usecase.HeavyTaskMessage
	if err := json.Unmarshal([]byte(body), &message); err != nil {
		t.Fatalf("unmarshal heavy task message: %v", err)
	}
	if message.TaskID != result.TaskID || message.UserID != "u1" || message.TaskType != usecase.HeavyTaskTypeTestExport {
		t.Fatalf("unexpected heavy task message: %#v", message)
	}
}

func TestEnqueueHeavyTaskRejectsUnknownTaskType(t *testing.T) {
	setupUsecaseOrderTxDB(t)
	previousQueue := usecase.DefaultQueueManager
	usecase.DefaultQueueManager = nil
	t.Cleanup(func() {
		usecase.DefaultQueueManager = previousQueue
	})

	ctx := authenticatedUsecaseContext(t.Context(), "u1", false)
	_, err := usecase.EnqueueHeavyTask(ctx, usecase.EnqueueHeavyTaskCmd{
		UserID:      "u1",
		TaskType:    "unknown_task",
		PayloadJSON: "{}",
	})
	if fwusecase.CodeOf(err) != fwusecase.CodeValidation {
		t.Fatalf("expected validation error for unknown task type, got %v", err)
	}

	appDB, dbErr := db.DefaultManager.GetDB("app")
	if dbErr != nil {
		t.Fatalf("get app db: %v", dbErr)
	}
	var count int
	if dbErr := appDB.Get(&count, `SELECT COUNT(*) FROM async_tasks WHERE task_type = 'unknown_task'`); dbErr != nil {
		t.Fatalf("count async tasks: %v", dbErr)
	}
	if count != 0 {
		t.Fatalf("expected unknown task not to be inserted, got count %d", count)
	}
}

func TestEnqueueUserHeavyTaskRejectsOrderExportTaskType(t *testing.T) {
	setupUsecaseOrderTxDB(t)
	previousQueue := usecase.DefaultQueueManager
	usecase.DefaultQueueManager = nil
	t.Cleanup(func() {
		usecase.DefaultQueueManager = previousQueue
	})

	ctx := authenticatedUsecaseContext(t.Context(), "u1", false)
	_, err := usecase.EnqueueUserHeavyTask(ctx, usecase.EnqueueHeavyTaskCmd{
		UserID:      "u1",
		TaskType:    usecase.HeavyTaskTypeOrdersExcelExport,
		PayloadJSON: "{}",
	})
	if fwusecase.CodeOf(err) != fwusecase.CodeValidation {
		t.Fatalf("expected validation error for direct order export task, got %v", err)
	}
}

func seedUsecaseAsyncTask(t *testing.T, id string, userID string, status string) {
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

func authenticatedUsecaseContext(ctx context.Context, userID string, admin bool) fwusecase.Context {
	ucCtx := fwusecase.NewContext(ctx, fwusecase.SurfaceInternalAPI)
	ucCtx.Actor = fwusecase.ActorContext{
		Authenticated: true,
		UserID:        userID,
		IsAdmin:       admin,
	}
	return ucCtx
}

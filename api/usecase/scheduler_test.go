package usecase_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/tfnick/go-svelte-starter/api/framework/queue"
	fwusecase "github.com/tfnick/go-svelte-starter/api/framework/usecase"
	"github.com/tfnick/go-svelte-starter/api/usecase"
	"github.com/tfnick/sqlx"
)

func TestEnqueueDueScheduledTasksClearsClaimAfterEnqueue(t *testing.T) {
	manager := setupUsecaseOrderTxDB(t)
	appDB, err := manager.GetDB("app")
	if err != nil {
		t.Fatalf("get app db: %v", err)
	}
	setupScheduledTaskQueueManager(t)

	now := time.Date(2026, 6, 19, 8, 0, 0, 0, time.UTC)
	seedUsecaseDueScheduledTask(t, appDB, "usecase-scheduled-clear-claim", now.Add(-time.Minute), "", "")

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceSystem)
	enqueued, err := usecase.EnqueueDueScheduledTasks(ctx, usecase.EnqueueDueScheduledTasksCmd{Now: now})
	if err != nil {
		t.Fatalf("enqueue due scheduled tasks: %v", err)
	}
	if enqueued != 1 {
		t.Fatalf("expected one enqueued task, got %d", enqueued)
	}
	if queueCount := countRows(t, appDB, `SELECT COUNT(*) FROM goqite WHERE queue = ?`, queue.QueueScheduledTasks); queueCount != 1 {
		t.Fatalf("expected one scheduled queue message, got %d", queueCount)
	}
	if executionCount := countRows(t, appDB, `SELECT COUNT(*) FROM scheduled_task_executions WHERE task_id = ?`, "usecase-scheduled-clear-claim"); executionCount != 1 {
		t.Fatalf("expected one scheduled execution, got %d", executionCount)
	}

	var claimToken string
	if err := appDB.Get(&claimToken, `SELECT claim_token FROM scheduled_tasks WHERE id = ?`, "usecase-scheduled-clear-claim"); err != nil {
		t.Fatalf("get claim token: %v", err)
	}
	if claimToken != "" {
		t.Fatalf("expected claim token to be cleared, got %q", claimToken)
	}
	var claimExpiresAt string
	if err := appDB.Get(&claimExpiresAt, `SELECT COALESCE(claim_expires_at, '') FROM scheduled_tasks WHERE id = ?`, "usecase-scheduled-clear-claim"); err != nil {
		t.Fatalf("get claim expiry: %v", err)
	}
	if claimExpiresAt != "" {
		t.Fatalf("expected claim expiry to be cleared, got %q", claimExpiresAt)
	}
}

func TestEnqueueDueScheduledTasksSkipsActiveClaim(t *testing.T) {
	manager := setupUsecaseOrderTxDB(t)
	appDB, err := manager.GetDB("app")
	if err != nil {
		t.Fatalf("get app db: %v", err)
	}
	setupScheduledTaskQueueManager(t)

	now := time.Date(2026, 6, 19, 8, 0, 0, 0, time.UTC)
	seedUsecaseDueScheduledTask(t, appDB, "usecase-scheduled-active-claim", now.Add(-time.Minute), "active-token", now.Add(time.Minute).Format(time.RFC3339))

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceSystem)
	enqueued, err := usecase.EnqueueDueScheduledTasks(ctx, usecase.EnqueueDueScheduledTasksCmd{Now: now})
	if err != nil {
		t.Fatalf("enqueue due scheduled tasks: %v", err)
	}
	if enqueued != 0 {
		t.Fatalf("expected active claim to skip enqueue, got %d", enqueued)
	}
	if queueCount := countRows(t, appDB, `SELECT COUNT(*) FROM goqite WHERE queue = ?`, queue.QueueScheduledTasks); queueCount != 0 {
		t.Fatalf("expected no scheduled queue messages, got %d", queueCount)
	}
	if executionCount := countRows(t, appDB, `SELECT COUNT(*) FROM scheduled_task_executions WHERE task_id = ?`, "usecase-scheduled-active-claim"); executionCount != 0 {
		t.Fatalf("expected no scheduled executions, got %d", executionCount)
	}
}

func TestConcurrentEnqueueDueScheduledTasksOnlyEnqueuesOnce(t *testing.T) {
	manager := setupUsecaseOrderTxDB(t)
	appDB, err := manager.GetDB("app")
	if err != nil {
		t.Fatalf("get app db: %v", err)
	}
	setupScheduledTaskQueueManager(t)

	now := time.Date(2026, 6, 19, 8, 0, 0, 0, time.UTC)
	seedUsecaseDueScheduledTask(t, appDB, "usecase-scheduled-concurrent", now.Add(-time.Minute), "", "")

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceSystem)
	start := make(chan struct{})
	type result struct {
		enqueued int
		err      error
	}
	results := make(chan result, 2)

	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			enqueued, err := usecase.EnqueueDueScheduledTasks(ctx, usecase.EnqueueDueScheduledTasksCmd{Now: now})
			results <- result{enqueued: enqueued, err: err}
		}()
	}
	close(start)
	wg.Wait()
	close(results)

	totalEnqueued := 0
	for result := range results {
		if result.err != nil {
			t.Fatalf("enqueue due scheduled tasks concurrently: %v", result.err)
		}
		totalEnqueued += result.enqueued
	}
	if totalEnqueued != 1 {
		t.Fatalf("expected concurrent schedulers to enqueue once, got %d", totalEnqueued)
	}
	if queueCount := countRows(t, appDB, `SELECT COUNT(*) FROM goqite WHERE queue = ?`, queue.QueueScheduledTasks); queueCount != 1 {
		t.Fatalf("expected one scheduled queue message, got %d", queueCount)
	}
	if executionCount := countRows(t, appDB, `SELECT COUNT(*) FROM scheduled_task_executions WHERE task_id = ?`, "usecase-scheduled-concurrent"); executionCount != 1 {
		t.Fatalf("expected one scheduled execution, got %d", executionCount)
	}
}

func TestEnqueueDueScheduledTasksRollsBackClaimWhenQueueFails(t *testing.T) {
	manager := setupUsecaseOrderTxDB(t)
	appDB, err := manager.GetDB("app")
	if err != nil {
		t.Fatalf("get app db: %v", err)
	}
	setupScheduledTaskQueueManager(t)

	now := time.Date(2026, 6, 19, 8, 0, 0, 0, time.UTC)
	seedUsecaseDueScheduledTask(t, appDB, "usecase-scheduled-queue-fails", now.Add(-time.Minute), "", "")

	if _, err := appDB.Exec(`DROP TABLE goqite`); err != nil {
		t.Fatalf("drop goqite to force queue failure: %v", err)
	}

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceSystem)
	_, err = usecase.EnqueueDueScheduledTasks(ctx, usecase.EnqueueDueScheduledTasksCmd{Now: now})
	if err == nil {
		t.Fatal("expected queue failure")
	}
	if fwusecase.CodeOf(err) != fwusecase.CodeInternal {
		t.Fatalf("expected internal error, got %q: %v", fwusecase.CodeOf(err), err)
	}
	if executionCount := countRows(t, appDB, `SELECT COUNT(*) FROM scheduled_task_executions WHERE task_id = ?`, "usecase-scheduled-queue-fails"); executionCount != 0 {
		t.Fatalf("expected scheduled execution rollback, got %d", executionCount)
	}

	var claimToken string
	if err := appDB.Get(&claimToken, `SELECT claim_token FROM scheduled_tasks WHERE id = ?`, "usecase-scheduled-queue-fails"); err != nil {
		t.Fatalf("get claim token: %v", err)
	}
	if claimToken != "" {
		t.Fatalf("expected claim token rollback, got %q", claimToken)
	}
}

func TestListScheduledTaskExecutionsReturnsRequestedPageAndMetadata(t *testing.T) {
	manager := setupUsecaseOrderTxDB(t)
	appDB, err := manager.GetEngine("app")
	if err != nil {
		t.Fatalf("get app engine: %v", err)
	}
	seedUsecaseScheduledTaskExecutions(t, appDB, "usecase-task-1", 5)

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	result, err := usecase.ListScheduledTaskExecutions(ctx, usecase.ScheduledTaskHistoryQry{
		TaskID:   "usecase-task-1",
		Page:     2,
		PageSize: 2,
	})
	if err != nil {
		t.Fatalf("list scheduled task executions: %v", err)
	}

	if len(result.Items) != 2 {
		t.Fatalf("expected two executions on page 2, got %#v", result.Items)
	}
	if result.Items[0].ID != "usecase-execution-03" || result.Items[1].ID != "usecase-execution-02" {
		t.Fatalf("expected stable created_at desc page order, got %#v", result.Items)
	}

	page := result.Pagination
	if page.Page != 2 || page.PageSize != 2 || page.TotalItems != 5 || page.TotalPages != 3 {
		t.Fatalf("unexpected pagination metadata: %#v", page)
	}
	if !page.HasPrevious || !page.HasNext {
		t.Fatalf("expected page 2 of 3 to have previous and next: %#v", page)
	}
}

func TestListScheduledTaskExecutionsRejectsInvalidInputs(t *testing.T) {
	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)

	if _, err := usecase.ListScheduledTaskExecutions(ctx, usecase.ScheduledTaskHistoryQry{}); err == nil {
		t.Fatal("expected empty task ID validation error")
	}

	if _, err := usecase.ListScheduledTaskExecutions(ctx, usecase.ScheduledTaskHistoryQry{
		TaskID:   "task-1",
		Page:     1,
		PageSize: 51,
	}); err == nil {
		t.Fatal("expected oversized page size validation error")
	}
}

func setupScheduledTaskQueueManager(t *testing.T) {
	t.Helper()

	queueManager, err := queue.NewManager()
	if err != nil {
		t.Fatalf("new queue manager: %v", err)
	}
	previous := usecase.DefaultQueueManager
	usecase.DefaultQueueManager = queueManager
	t.Cleanup(func() {
		usecase.DefaultQueueManager = previous
	})
}

func seedUsecaseDueScheduledTask(t *testing.T, appDB *sqlx.DB, taskID string, dueAt time.Time, claimToken string, claimExpiresAt string) {
	t.Helper()

	dueAtText := dueAt.UTC().Format(time.RFC3339)
	_, err := appDB.Exec(`
		INSERT INTO scheduled_tasks (
			id, name, job_name, schedule_type, schedule_value, payload_json,
			enabled, next_run_at, claim_token, claim_expires_at, created_at, updated_at
		) VALUES (?, 'Usecase scheduled task', 'scheduler.noop', 'once_at', ?, '{}', 1, ?, ?, NULLIF(?, ''), '2026-01-01 00:00:00', '2026-01-01 00:00:00')
	`, taskID, dueAtText, dueAtText, claimToken, claimExpiresAt)
	if err != nil {
		t.Fatalf("insert scheduled task: %v", err)
	}
}

func seedUsecaseScheduledTaskExecutions(t *testing.T, appDB *sqlx.Engine, taskID string, count int) {
	t.Helper()

	_, err := appDB.ExecP(`
		INSERT INTO scheduled_tasks (
			id, name, job_name, schedule_type, schedule_value, payload_json, enabled, created_at, updated_at
		) VALUES (?, 'Usecase task', 'scheduler.noop', 'cron', '*/5 * * * *', '{}', 1, '2026-01-01 00:00:00', '2026-01-01 00:00:00')
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
			fmt.Sprintf("usecase-execution-%02d", i),
			taskID,
			fmt.Sprintf("usecase-message-%02d", i),
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

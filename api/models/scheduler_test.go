package models_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/tfnick/go-svelte-starter/api/models"
	"github.com/tfnick/sqlx"
)

func TestClaimScheduledTaskAllowsOnlyOneActiveClaim(t *testing.T) {
	manager := setupModelsTestDB(t)
	appDB, err := manager.GetEngine("app")
	if err != nil {
		t.Fatalf("get app engine: %v", err)
	}

	now := time.Date(2026, 6, 19, 8, 0, 0, 0, time.UTC)
	seedModelScheduledTask(t, appDB, "model-claim-active", now.Add(-time.Minute).Format(time.RFC3339), "", "")

	claimed, err := models.ClaimScheduledTask(
		t.Context(),
		"model-claim-active",
		"claim-token-1",
		now.Format(time.RFC3339),
		now.Add(2*time.Minute).Format(time.RFC3339),
	)
	if err != nil {
		t.Fatalf("claim scheduled task: %v", err)
	}
	if !claimed {
		t.Fatal("expected first claim to succeed")
	}

	claimed, err = models.ClaimScheduledTask(
		t.Context(),
		"model-claim-active",
		"claim-token-2",
		now.Format(time.RFC3339),
		now.Add(2*time.Minute).Format(time.RFC3339),
	)
	if err != nil {
		t.Fatalf("claim scheduled task again: %v", err)
	}
	if claimed {
		t.Fatal("expected active claim to block duplicate claim")
	}

	var token string
	if err := appDB.GetP(&token, `SELECT claim_token FROM scheduled_tasks WHERE id = ?`, "model-claim-active"); err != nil {
		t.Fatalf("get claim token: %v", err)
	}
	if token != "claim-token-1" {
		t.Fatalf("expected original claim token to remain, got %q", token)
	}
}

func TestClaimScheduledTaskAllowsExpiredClaim(t *testing.T) {
	manager := setupModelsTestDB(t)
	appDB, err := manager.GetEngine("app")
	if err != nil {
		t.Fatalf("get app engine: %v", err)
	}

	now := time.Date(2026, 6, 19, 8, 0, 0, 0, time.UTC)
	seedModelScheduledTask(
		t,
		appDB,
		"model-claim-expired",
		now.Add(-time.Minute).Format(time.RFC3339),
		"old-token",
		now.Add(-time.Minute).Format(time.RFC3339),
	)

	claimed, err := models.ClaimScheduledTask(
		t.Context(),
		"model-claim-expired",
		"new-token",
		now.Format(time.RFC3339),
		now.Add(2*time.Minute).Format(time.RFC3339),
	)
	if err != nil {
		t.Fatalf("claim expired scheduled task: %v", err)
	}
	if !claimed {
		t.Fatal("expected expired claim to be claimable")
	}

	var token string
	if err := appDB.GetP(&token, `SELECT claim_token FROM scheduled_tasks WHERE id = ?`, "model-claim-expired"); err != nil {
		t.Fatalf("get claim token: %v", err)
	}
	if token != "new-token" {
		t.Fatalf("expected new token after expired claim, got %q", token)
	}
}

func TestListDueScheduledTasksSkipsActiveClaims(t *testing.T) {
	manager := setupModelsTestDB(t)
	appDB, err := manager.GetEngine("app")
	if err != nil {
		t.Fatalf("get app engine: %v", err)
	}

	now := time.Date(2026, 6, 19, 8, 0, 0, 0, time.UTC)
	nowText := now.Format(time.RFC3339)
	seedModelScheduledTask(t, appDB, "model-due-unclaimed", now.Add(-time.Minute).Format(time.RFC3339), "", "")
	seedModelScheduledTask(t, appDB, "model-due-active-claim", now.Add(-time.Minute).Format(time.RFC3339), "active-token", now.Add(time.Minute).Format(time.RFC3339))
	seedModelScheduledTask(t, appDB, "model-due-expired-claim", now.Add(-time.Minute).Format(time.RFC3339), "expired-token", now.Add(-time.Minute).Format(time.RFC3339))

	tasks, err := models.ListDueScheduledTasks(t.Context(), nowText)
	if err != nil {
		t.Fatalf("list due scheduled tasks: %v", err)
	}

	ids := map[string]bool{}
	for _, task := range tasks {
		ids[task.ID] = true
	}
	if !ids["model-due-unclaimed"] || !ids["model-due-expired-claim"] {
		t.Fatalf("expected unclaimed and expired tasks to be due, got %#v", ids)
	}
	if ids["model-due-active-claim"] {
		t.Fatalf("expected active claim task to be skipped, got %#v", ids)
	}
}

func TestListScheduledTaskExecutionsUsesPaginationAndTaskScope(t *testing.T) {
	manager := setupModelsTestDB(t)
	appDB, err := manager.GetEngine("app")
	if err != nil {
		t.Fatalf("get app engine: %v", err)
	}

	seedModelScheduledTaskExecutions(t, appDB, "model-task-1", "model-execution", 5)
	seedModelScheduledTaskExecutions(t, appDB, "model-task-2", "other-execution", 1)

	total, err := models.CountScheduledTaskExecutions(t.Context(), "model-task-1")
	if err != nil {
		t.Fatalf("count scheduled task executions: %v", err)
	}
	if total != 5 {
		t.Fatalf("expected task-scoped count 5, got %d", total)
	}

	executions, err := models.ListScheduledTaskExecutions(t.Context(), "model-task-1", 2, 2)
	if err != nil {
		t.Fatalf("list scheduled task executions: %v", err)
	}
	if len(executions) != 2 {
		t.Fatalf("expected two executions on page 2, got %#v", executions)
	}
	if executions[0].ID != "model-execution-03" || executions[1].ID != "model-execution-02" {
		t.Fatalf("expected stable created_at desc page order, got %#v", executions)
	}
	for _, execution := range executions {
		if execution.TaskID != "model-task-1" {
			t.Fatalf("expected task-scoped execution, got %#v", execution)
		}
	}
}

func seedModelScheduledTask(t *testing.T, appDB *sqlx.Engine, taskID string, nextRunAt string, claimToken string, claimExpiresAt string) {
	t.Helper()

	_, err := appDB.ExecP(`
		INSERT INTO scheduled_tasks (
			id, name, job_name, schedule_type, schedule_value, payload_json,
			enabled, next_run_at, claim_token, claim_expires_at, created_at, updated_at
		) VALUES (?, ?, 'scheduler.noop', 'cron', '*/5 * * * *', '{}', 1, ?, ?, NULLIF(?, ''), '2026-01-01 00:00:00', '2026-01-01 00:00:00')
	`, taskID, taskID, nextRunAt, claimToken, claimExpiresAt)
	if err != nil {
		t.Fatalf("insert scheduled task %s: %v", taskID, err)
	}
}

func seedModelScheduledTaskExecutions(t *testing.T, appDB *sqlx.Engine, taskID string, executionPrefix string, count int) {
	t.Helper()

	_, err := appDB.ExecP(`
		INSERT INTO scheduled_tasks (
			id, name, job_name, schedule_type, schedule_value, payload_json, enabled, created_at, updated_at
		) VALUES (?, ?, 'scheduler.noop', 'cron', '*/5 * * * *', '{}', 1, '2026-01-01 00:00:00', '2026-01-01 00:00:00')
	`, taskID, taskID)
	if err != nil {
		t.Fatalf("insert scheduled task %s: %v", taskID, err)
	}

	query := `
		INSERT INTO scheduled_task_executions (
			id, task_id, job_name, message_id, status, scheduled_at, started_at, finished_at, error_message, created_at
		) VALUES (?, ?, 'scheduler.noop', ?, 'succeeded', ?, ?, ?, '', ?)
	`
	for i := 1; i <= count; i++ {
		createdAt := fmt.Sprintf("2026-01-01 00:00:%02d", i)
		_, err := appDB.ExecP(query,
			fmt.Sprintf("%s-%02d", executionPrefix, i),
			taskID,
			fmt.Sprintf("%s-message-%02d", executionPrefix, i),
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

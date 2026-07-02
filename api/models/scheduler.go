package models

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/tfnick/go-svelte-starter/api/db"
	"github.com/tfnick/go-svelte-starter/api/framework/data/modelerror"
)

const (
	ScheduleTypeCron   = "cron"
	ScheduleTypeOnceAt = "once_at"

	ScheduledExecutionStatusQueued    = "queued"
	ScheduledExecutionStatusRunning   = "running"
	ScheduledExecutionStatusSucceeded = "succeeded"
	ScheduledExecutionStatusFailed    = "failed"
)

type ScheduledTask struct {
	ID             string `db:"id"`
	Name           string `db:"name"`
	JobName        string `db:"job_name"`
	ScheduleType   string `db:"schedule_type"`
	ScheduleValue  string `db:"schedule_value"`
	PayloadJSON    string `db:"payload_json"`
	Enabled        bool   `db:"enabled"`
	NextRunAt      string `db:"next_run_at"`
	LastRunAt      string `db:"last_run_at"`
	ClaimToken     string `db:"claim_token"`
	ClaimExpiresAt string `db:"claim_expires_at"`
	CreatedAt      string `db:"created_at"`
	UpdatedAt      string `db:"updated_at"`
}

type ScheduledTaskExecution struct {
	ID           string `db:"id"`
	TaskID       string `db:"task_id"`
	JobName      string `db:"job_name"`
	MessageID    string `db:"message_id"`
	Status       string `db:"status"`
	ScheduledAt  string `db:"scheduled_at"`
	StartedAt    string `db:"started_at"`
	FinishedAt   string `db:"finished_at"`
	ErrorMessage string `db:"error_message"`
	CreatedAt    string `db:"created_at"`
}

type ScheduledTaskInput struct {
	Name          string
	JobName       string
	ScheduleType  string
	ScheduleValue string
	PayloadJSON   string
	Enabled       bool
	NextRunAt     string
}

func ListScheduledTasks(ctx context.Context) ([]ScheduledTask, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return nil, fmt.Errorf("database unavailable: %w", err)
	}

	var tasks []ScheduledTask
	if err := d.SelectP(&tasks, `
		SELECT id, name, job_name, schedule_type, schedule_value, payload_json, enabled,
			COALESCE(next_run_at, '') AS next_run_at,
			COALESCE(last_run_at, '') AS last_run_at,
			COALESCE(claim_token, '') AS claim_token,
			COALESCE(claim_expires_at, '') AS claim_expires_at,
			created_at, updated_at
		FROM scheduled_tasks
		ORDER BY created_at DESC
	`); err != nil {
		return nil, fmt.Errorf("list scheduled tasks failed: %w", err)
	}
	return tasks, nil
}

func InsertScheduledTask(ctx context.Context, input ScheduledTaskInput) (ScheduledTask, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return ScheduledTask{}, fmt.Errorf("database unavailable: %w", err)
	}

	task := ScheduledTask{
		ID:            uuid.Must(uuid.NewV7()).String(),
		Name:          input.Name,
		JobName:       input.JobName,
		ScheduleType:  input.ScheduleType,
		ScheduleValue: input.ScheduleValue,
		PayloadJSON:   input.PayloadJSON,
		Enabled:       input.Enabled,
		NextRunAt:     input.NextRunAt,
	}
	if task.PayloadJSON == "" {
		task.PayloadJSON = "{}"
	}

	_, err = d.ExecNamed(`
		INSERT INTO scheduled_tasks (
			id, name, job_name, schedule_type, schedule_value, payload_json, enabled, next_run_at
		) VALUES (
			:id, :name, :job_name, :schedule_type, :schedule_value, :payload_json, :enabled, :next_run_at
		)
	`, task)

	if err != nil {
		return ScheduledTask{}, fmt.Errorf("insert scheduled task failed: %w", err)
	}
	return GetScheduledTask(ctx, task.ID)
}

func UpdateScheduledTask(ctx context.Context, id string, input ScheduledTaskInput) (ScheduledTask, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return ScheduledTask{}, fmt.Errorf("database unavailable: %w", err)
	}

	task := ScheduledTask{
		ID:            id,
		Name:          input.Name,
		JobName:       input.JobName,
		ScheduleType:  input.ScheduleType,
		ScheduleValue: input.ScheduleValue,
		PayloadJSON:   input.PayloadJSON,
		Enabled:       input.Enabled,
		NextRunAt:     input.NextRunAt,
	}
	if task.PayloadJSON == "" {
		task.PayloadJSON = "{}"
	}

	result, err := d.ExecNamed(`
		UPDATE scheduled_tasks
		SET name = :name,
			job_name = :job_name,
			schedule_type = :schedule_type,
			schedule_value = :schedule_value,
			payload_json = :payload_json,
			enabled = :enabled,
			next_run_at = :next_run_at,
			claim_token = '',
			claim_expires_at = NULL
		WHERE id = :id
	`, task)

	if err != nil {
		return ScheduledTask{}, fmt.Errorf("update scheduled task failed: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return ScheduledTask{}, fmt.Errorf("read affected rows failed: %w", err)
	}
	if rows == 0 {
		return ScheduledTask{}, fmt.Errorf("scheduled task not found: %w", modelerror.ErrNotFound)
	}
	return GetScheduledTask(ctx, id)
}

func SetScheduledTaskEnabled(ctx context.Context, id string, enabled bool, nextRunAt string) (ScheduledTask, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return ScheduledTask{}, fmt.Errorf("database unavailable: %w", err)
	}

	query := `
		UPDATE scheduled_tasks
		SET enabled = ?, next_run_at = ?, claim_token = '', claim_expires_at = NULL
		WHERE id = ?
	`
	result, err := d.ExecP(query, enabled, nextRunAt, id)
	if err != nil {
		return ScheduledTask{}, fmt.Errorf("set scheduled task enabled failed: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return ScheduledTask{}, fmt.Errorf("read affected rows failed: %w", err)
	}
	if rows == 0 {
		return ScheduledTask{}, fmt.Errorf("scheduled task not found: %w", modelerror.ErrNotFound)
	}
	return GetScheduledTask(ctx, id)
}

func GetScheduledTask(ctx context.Context, id string) (ScheduledTask, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return ScheduledTask{}, fmt.Errorf("database unavailable: %w", err)
	}

	var task ScheduledTask
	query := `
		SELECT id, name, job_name, schedule_type, schedule_value, payload_json, enabled,
			COALESCE(next_run_at, '') AS next_run_at,
			COALESCE(last_run_at, '') AS last_run_at,
			COALESCE(claim_token, '') AS claim_token,
			COALESCE(claim_expires_at, '') AS claim_expires_at,
			created_at, updated_at
		FROM scheduled_tasks
		WHERE id = ?
	`
	if err := d.GetP(&task, query, id); err != nil {
		return ScheduledTask{}, fmt.Errorf("get scheduled task failed: %w", err)
	}
	return task, nil
}

func ListDueScheduledTasks(ctx context.Context, now string) ([]ScheduledTask, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return nil, fmt.Errorf("database unavailable: %w", err)
	}

	var tasks []ScheduledTask
	query := `
		SELECT id, name, job_name, schedule_type, schedule_value, payload_json, enabled,
			COALESCE(next_run_at, '') AS next_run_at,
			COALESCE(last_run_at, '') AS last_run_at,
			COALESCE(claim_token, '') AS claim_token,
			COALESCE(claim_expires_at, '') AS claim_expires_at,
			created_at, updated_at
		FROM scheduled_tasks
		WHERE enabled = 1
			AND next_run_at IS NOT NULL
			AND next_run_at <= ?
			AND (claim_token = '' OR claim_expires_at IS NULL OR claim_expires_at <= ?)
		ORDER BY next_run_at ASC
	`
	if err := d.SelectP(&tasks, query, now, now); err != nil {
		return nil, fmt.Errorf("list due scheduled tasks failed: %w", err)
	}
	return tasks, nil
}

func ClaimScheduledTask(ctx context.Context, taskID string, token string, now string, expiresAt string) (bool, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return false, fmt.Errorf("database unavailable: %w", err)
	}

	result, err := d.ExecP(`
		UPDATE scheduled_tasks
		SET claim_token = ?, claim_expires_at = ?
		WHERE id = ?
			AND enabled = 1
			AND next_run_at IS NOT NULL
			AND next_run_at <= ?
			AND (claim_token = '' OR claim_expires_at IS NULL OR claim_expires_at <= ?)
	`, token, expiresAt, taskID, now, now)
	if err != nil {
		return false, fmt.Errorf("claim scheduled task failed: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("read affected rows failed: %w", err)
	}
	return rows == 1, nil
}

func RecordScheduledTaskEnqueued(ctx context.Context, taskID string, claimToken string, executionID string, messageID string, scheduledAt string, nextRunAt string, enabled bool) error {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return fmt.Errorf("database unavailable: %w", err)
	}

	execution := ScheduledTaskExecution{
		ID:          executionID,
		TaskID:      taskID,
		MessageID:   messageID,
		Status:      ScheduledExecutionStatusQueued,
		ScheduledAt: scheduledAt,
	}
	task, err := GetScheduledTask(ctx, taskID)
	if err != nil {
		return err
	}
	execution.JobName = task.JobName

	_, err = d.ExecNamed(`
		INSERT INTO scheduled_task_executions (
			id, task_id, job_name, message_id, status, scheduled_at
		) VALUES (
			:id, :task_id, :job_name, :message_id, :status, :scheduled_at
		)
	`, execution)

	if err != nil {
		return fmt.Errorf("insert scheduled task execution failed: %w", err)
	}

	query := `
		UPDATE scheduled_tasks
		SET last_run_at = ?, next_run_at = ?, enabled = ?, claim_token = '', claim_expires_at = NULL
		WHERE id = ? AND claim_token = ?
	`
	result, err := d.ExecP(query, scheduledAt, nextRunAt, enabled, taskID, claimToken)
	if err != nil {
		return fmt.Errorf("update scheduled task run markers failed: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read affected rows failed: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("scheduled task claim not found: %w", modelerror.ErrNotFound)
	}
	return nil
}

func MarkScheduledTaskExecutionRunning(ctx context.Context, executionID string) error {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return fmt.Errorf("database unavailable: %w", err)
	}

	query := `UPDATE scheduled_task_executions SET status = ?, started_at = CURRENT_TIMESTAMP WHERE id = ?`
	result, err := d.ExecP(query, ScheduledExecutionStatusRunning, executionID)
	if err != nil {
		return fmt.Errorf("mark scheduled task execution running failed: %w", err)
	}
	return requireAffected(result, "scheduled task execution not found")
}

func MarkScheduledTaskExecutionSucceeded(ctx context.Context, executionID string) error {
	return finishScheduledTaskExecution(ctx, executionID, ScheduledExecutionStatusSucceeded, "")
}

func MarkScheduledTaskExecutionFailed(ctx context.Context, executionID string, errorMessage string) error {
	return finishScheduledTaskExecution(ctx, executionID, ScheduledExecutionStatusFailed, errorMessage)
}

func finishScheduledTaskExecution(ctx context.Context, executionID string, status string, errorMessage string) error {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return fmt.Errorf("database unavailable: %w", err)
	}

	query := `UPDATE scheduled_task_executions SET status = ?, finished_at = CURRENT_TIMESTAMP, error_message = ? WHERE id = ?`
	result, err := d.ExecP(query, status, errorMessage, executionID)
	if err != nil {
		return fmt.Errorf("finish scheduled task execution failed: %w", err)
	}
	return requireAffected(result, "scheduled task execution not found")
}

func CountScheduledTaskExecutions(ctx context.Context, taskID string) (int, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return 0, fmt.Errorf("database unavailable: %w", err)
	}

	var count int
	query := `SELECT COUNT(*) FROM scheduled_task_executions WHERE task_id = ?`
	if err := d.GetP(&count, query, taskID); err != nil {
		return 0, fmt.Errorf("count scheduled task executions failed: %w", err)
	}
	return count, nil
}

func ListScheduledTaskExecutions(ctx context.Context, taskID string, limit int, offset int) ([]ScheduledTaskExecution, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return nil, fmt.Errorf("database unavailable: %w", err)
	}

	var executions []ScheduledTaskExecution
	query := `
		SELECT id, task_id, job_name,
			COALESCE(message_id, '') AS message_id,
			status,
			COALESCE(scheduled_at, '') AS scheduled_at,
			COALESCE(started_at, '') AS started_at,
			COALESCE(finished_at, '') AS finished_at,
			COALESCE(error_message, '') AS error_message,
			created_at
		FROM scheduled_task_executions
		WHERE task_id = ?
		ORDER BY created_at DESC, id DESC
		LIMIT ? OFFSET ?
	`
	if err := d.SelectP(&executions, query, taskID, limit, offset); err != nil {
		return nil, fmt.Errorf("list scheduled task executions failed: %w", err)
	}
	return executions, nil
}

type affectedResult interface {
	RowsAffected() (int64, error)
}

func requireAffected(result affectedResult, message string) error {
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read affected rows failed: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("%s: %w", message, modelerror.ErrNotFound)
	}
	return nil
}

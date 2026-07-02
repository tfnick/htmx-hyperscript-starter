package models

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/tfnick/go-svelte-starter/api/db"
	"github.com/tfnick/go-svelte-starter/api/framework/data/modelerror"
	"github.com/tfnick/go-svelte-starter/api/framework/timefmt"
)

const (
	AsyncTaskStatusQueued     = "queued"
	AsyncTaskStatusProcessing = "processing"
	AsyncTaskStatusCompleted  = "completed"
	AsyncTaskStatusFailed     = "failed"

	MaxAsyncTaskRetries = 3
)

type AsyncTask struct {
	ID           string `db:"id"`
	UserID       string `db:"user_id"`
	TaskType     string `db:"task_type"`
	Status       string `db:"status"`
	PayloadJSON  string `db:"payload_json"`
	ResultJSON   string `db:"result_json"`
	ErrorMessage string `db:"error_message"`
	RetryCount   int    `db:"retry_count"`
	MessageID    string `db:"message_id"`
	ClearedAt    string `db:"cleared_at"`
	CreatedAt    string `db:"created_at"`
	UpdatedAt    string `db:"updated_at"`
}

func InsertAsyncTask(ctx context.Context, task *AsyncTask) error {
	if task.Status == "" {
		task.Status = AsyncTaskStatusQueued
	}

	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return fmt.Errorf("get executor for insert async task: %w", err)
	}

	query := `INSERT INTO async_tasks (id, user_id, task_type, status, payload_json, result_json, error_message, retry_count, message_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err = d.ExecP(query, task.ID, task.UserID, task.TaskType, task.Status, task.PayloadJSON, task.ResultJSON, task.ErrorMessage, task.RetryCount, task.MessageID)
	if err != nil {
		return fmt.Errorf("insert async task: %w", err)
	}
	return nil
}

func GetAsyncTaskByID(ctx context.Context, id string) (*AsyncTask, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return nil, fmt.Errorf("get executor for get async task: %w", err)
	}

	query := "SELECT * FROM async_tasks WHERE id = ?"
	var task AsyncTask
	if err := d.GetP(&task, query, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, modelerror.ErrNotFound
		}
		return nil, fmt.Errorf("get async task by id: %w", err)
	}
	return &task, nil
}

func UpdateAsyncTaskStatus(ctx context.Context, id string, status string, resultJSON string, errorMessage string) error {
	now := timefmt.NowSQLiteDateTime()

	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return fmt.Errorf("get executor for update async task: %w", err)
	}

	query := `UPDATE async_tasks SET status = ?, result_json = ?, error_message = ?, updated_at = ? WHERE id = ?`
	result, err := d.ExecP(query, status, resultJSON, errorMessage, now, id)
	if err != nil {
		return fmt.Errorf("update async task status: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read affected rows: %w", err)
	}
	if rows == 0 {
		return modelerror.ErrNotFound
	}
	return nil
}

func IncrementAsyncTaskRetryCount(ctx context.Context, id string, errorMessage string) error {
	now := timefmt.NowSQLiteDateTime()

	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return fmt.Errorf("get executor for increment retry: %w", err)
	}

	query := `UPDATE async_tasks SET retry_count = retry_count + 1, error_message = ?, updated_at = ? WHERE id = ?`
	result, err := d.ExecP(query, errorMessage, now, id)
	if err != nil {
		return fmt.Errorf("increment async task retry: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read affected rows: %w", err)
	}
	if rows == 0 {
		return modelerror.ErrNotFound
	}
	return nil
}

func ListAsyncTasksByUser(ctx context.Context, userID string, limit int, offset int) ([]AsyncTask, error) {
	eng, err := db.EngineFor(ctx, "app")
	if err != nil {
		return nil, fmt.Errorf("get executor for list async tasks: %w", err)
	}

	sql := `
		SELECT * FROM async_tasks
		WHERE user_id = ?
		  AND cleared_at = ''
		ORDER BY created_at DESC, id DESC
		LIMIT ? OFFSET ?
	`

	var tasks []AsyncTask
	if err := eng.SelectP(&tasks, sql, userID, limit, offset); err != nil {
		return nil, fmt.Errorf("list async tasks: %w", err)
	}
	return tasks, nil
}

func CountAsyncTasksByUser(ctx context.Context, userID string) (int, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return 0, fmt.Errorf("get executor for count async tasks: %w", err)
	}

	query := "SELECT COUNT(*) FROM async_tasks WHERE user_id = ? AND cleared_at = ''"
	var count int
	if err := d.GetP(&count, query, userID); err != nil {
		return 0, fmt.Errorf("count async tasks: %w", err)
	}
	return count, nil
}

func ClearTerminalAsyncTasksByUser(ctx context.Context, userID string) (int, error) {
	now := timefmt.NowSQLiteDateTime()

	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return 0, fmt.Errorf("get executor for clear async tasks: %w", err)
	}

	query := `
		UPDATE async_tasks
		SET cleared_at = ?, updated_at = ?
		WHERE user_id = ?
		  AND cleared_at = ''
		  AND status IN (?, ?)
	`
	result, err := d.ExecP(query, now, now, userID, AsyncTaskStatusCompleted, AsyncTaskStatusFailed)
	if err != nil {
		return 0, fmt.Errorf("clear async tasks: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("read affected rows: %w", err)
	}
	return int(rows), nil
}

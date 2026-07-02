package usecase

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
	"github.com/tfnick/go-svelte-starter/api/framework/data/modelerror"
	"github.com/tfnick/go-svelte-starter/api/framework/queue"
	"github.com/tfnick/go-svelte-starter/api/framework/timefmt"
	fwusecase "github.com/tfnick/go-svelte-starter/api/framework/usecase"
	"github.com/tfnick/go-svelte-starter/api/models"
)

type ScheduledTaskQry struct{}

type ScheduledTaskHistoryQry struct {
	TaskID   string
	Page     int
	PageSize int
}

type CreateScheduledTaskCmd struct {
	Name          string
	JobName       string
	ScheduleType  string
	ScheduleValue string
	PayloadJSON   string
	Enabled       bool
}

type UpdateScheduledTaskCmd struct {
	ID            string
	Name          string
	JobName       string
	ScheduleType  string
	ScheduleValue string
	PayloadJSON   string
	Enabled       bool
}

type SetScheduledTaskEnabledCmd struct {
	ID      string
	Enabled bool
}

type EnqueueDueScheduledTasksCmd struct {
	Now time.Time
}

type ScheduledTaskCo struct {
	ID            string
	Name          string
	JobName       string
	ScheduleType  string
	ScheduleValue string
	PayloadJSON   string
	Enabled       bool
	NextRunAt     string
	LastRunAt     string
	CreatedAt     string
	UpdatedAt     string
}

type ScheduledTaskExecutionCo struct {
	ID           string
	TaskID       string
	JobName      string
	MessageID    string
	Status       string
	ScheduledAt  string
	StartedAt    string
	FinishedAt   string
	ErrorMessage string
	CreatedAt    string
}

type ScheduledTaskExecutionsCo struct {
	Items      []ScheduledTaskExecutionCo
	Pagination fwusecase.PageResult
}

type ScheduledTaskJobPayload struct {
	TaskID      string          `json:"task_id"`
	ExecutionID string          `json:"execution_id"`
	Payload     json.RawMessage `json:"payload"`
}

const (
	BuiltInScheduledTaskJob = "scheduler.noop"

	scheduledTaskClaimLease = 2 * time.Minute
)

var DefaultQueueManager *queue.Manager

func ListScheduledTasks(ctx fwusecase.Context, _ ScheduledTaskQry) ([]ScheduledTaskCo, error) {
	tasks, err := models.ListScheduledTasks(ctx.Std())
	if err != nil {
		return nil, fwusecase.E(fwusecase.CodeInternal, "failed to load scheduled tasks", err)
	}
	return scheduledTaskCosFromModels(tasks), nil
}

func CreateScheduledTask(ctx fwusecase.Context, cmd CreateScheduledTaskCmd) (ScheduledTaskCo, error) {
	input, err := scheduledTaskInput(cmd.Name, cmd.JobName, cmd.ScheduleType, cmd.ScheduleValue, cmd.PayloadJSON, cmd.Enabled)
	if err != nil {
		return ScheduledTaskCo{}, err
	}

	task, err := models.InsertScheduledTask(ctx.Std(), input)
	if err != nil {
		return ScheduledTaskCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to create scheduled task", err)
	}
	return scheduledTaskCoFromModel(task), nil
}

func UpdateScheduledTask(ctx fwusecase.Context, cmd UpdateScheduledTaskCmd) (ScheduledTaskCo, error) {
	if strings.TrimSpace(cmd.ID) == "" {
		return ScheduledTaskCo{}, fwusecase.E(fwusecase.CodeValidation, "task ID is required", nil)
	}

	input, err := scheduledTaskInput(cmd.Name, cmd.JobName, cmd.ScheduleType, cmd.ScheduleValue, cmd.PayloadJSON, cmd.Enabled)
	if err != nil {
		return ScheduledTaskCo{}, err
	}

	task, err := models.UpdateScheduledTask(ctx.Std(), cmd.ID, input)
	if err != nil {
		if errors.Is(err, modelerror.ErrNotFound) {
			return ScheduledTaskCo{}, fwusecase.E(fwusecase.CodeNotFound, "scheduled task not found", err)
		}
		return ScheduledTaskCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to update scheduled task", err)
	}
	return scheduledTaskCoFromModel(task), nil
}

func SetScheduledTaskEnabled(ctx fwusecase.Context, cmd SetScheduledTaskEnabledCmd) (ScheduledTaskCo, error) {
	if strings.TrimSpace(cmd.ID) == "" {
		return ScheduledTaskCo{}, fwusecase.E(fwusecase.CodeValidation, "task ID is required", nil)
	}

	task, err := models.GetScheduledTask(ctx.Std(), cmd.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ScheduledTaskCo{}, fwusecase.E(fwusecase.CodeNotFound, "scheduled task not found", err)
		}
		return ScheduledTaskCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to load scheduled task", err)
	}

	nextRunAt := ""
	if cmd.Enabled {
		next, err := computeNextRunAt(task.ScheduleType, task.ScheduleValue, timefmt.NowUTC())
		if err != nil {
			return ScheduledTaskCo{}, err
		}
		nextRunAt = formatTime(next)
	}

	task, err = models.SetScheduledTaskEnabled(ctx.Std(), cmd.ID, cmd.Enabled, nextRunAt)
	if err != nil {
		if errors.Is(err, modelerror.ErrNotFound) {
			return ScheduledTaskCo{}, fwusecase.E(fwusecase.CodeNotFound, "scheduled task not found", err)
		}
		return ScheduledTaskCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to update scheduled task enabled state", err)
	}
	return scheduledTaskCoFromModel(task), nil
}

func ListScheduledTaskExecutions(ctx fwusecase.Context, qry ScheduledTaskHistoryQry) (ScheduledTaskExecutionsCo, error) {
	if strings.TrimSpace(qry.TaskID) == "" {
		return ScheduledTaskExecutionsCo{}, fwusecase.E(fwusecase.CodeValidation, "task ID is required", nil)
	}

	pageQuery, err := fwusecase.NormalizePageQuery(fwusecase.PageQuery{
		Page:     qry.Page,
		PageSize: qry.PageSize,
	})
	if err != nil {
		return ScheduledTaskExecutionsCo{}, err
	}

	totalItems, err := models.CountScheduledTaskExecutions(ctx.Std(), qry.TaskID)
	if err != nil {
		return ScheduledTaskExecutionsCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to count scheduled task history", err)
	}

	executions, err := models.ListScheduledTaskExecutions(ctx.Std(), qry.TaskID, pageQuery.Limit(), pageQuery.Offset())
	if err != nil {
		return ScheduledTaskExecutionsCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to load scheduled task history", err)
	}
	return ScheduledTaskExecutionsCo{
		Items:      scheduledTaskExecutionCosFromModels(executions),
		Pagination: fwusecase.NewPageResult(pageQuery, totalItems),
	}, nil
}

func EnqueueDueScheduledTasks(ctx fwusecase.Context, cmd EnqueueDueScheduledTasksCmd) (int, error) {
	if DefaultQueueManager == nil {
		return 0, fwusecase.E(fwusecase.CodeInternal, "queue manager is not configured", nil)
	}

	now := cmd.Now
	if now.IsZero() {
		now = timefmt.NowUTC()
	} else {
		now = now.UTC()
	}

	tasks, err := models.ListDueScheduledTasks(ctx.Std(), formatTime(now))
	if err != nil {
		return 0, fwusecase.E(fwusecase.CodeInternal, "failed to load due scheduled tasks", err)
	}

	enqueued := 0
	for _, task := range tasks {
		claimed, err := enqueueScheduledTask(ctx, task, now)
		if err != nil {
			return enqueued, err
		}
		if claimed {
			enqueued++
		}
	}
	return enqueued, nil
}

func enqueueScheduledTask(ctx fwusecase.Context, task models.ScheduledTask, now time.Time) (bool, error) {
	claimToken := uuid.Must(uuid.NewV7()).String()
	executionID := uuid.Must(uuid.NewV7()).String()
	payload := ScheduledTaskJobPayload{
		TaskID:      task.ID,
		ExecutionID: executionID,
		Payload:     json.RawMessage(task.PayloadJSON),
	}

	var nextRunAt string
	nextEnabled := task.Enabled
	if task.ScheduleType == models.ScheduleTypeCron {
		next, err := computeNextRunAt(task.ScheduleType, task.ScheduleValue, now)
		if err != nil {
			return false, err
		}
		nextRunAt = formatTime(next)
	} else if task.ScheduleType == models.ScheduleTypeOnceAt {
		nextEnabled = false
	}

	var messageID string
	claimed := false
	err := fwusecase.WithAppTx(ctx, func(txCtx fwusecase.Context) error {
		ok, err := models.ClaimScheduledTask(
			txCtx.Std(),
			task.ID,
			claimToken,
			formatTime(now),
			formatTime(now.Add(scheduledTaskClaimLease)),
		)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
		claimed = true

		messageID, err = DefaultQueueManager.CreateJSONJob(txCtx.Std(), queue.QueueScheduledTasks, task.JobName, payload, 0, 0)
		if err != nil {
			return err
		}
		return models.RecordScheduledTaskEnqueued(txCtx.Std(), task.ID, claimToken, executionID, messageID, formatTime(now), nextRunAt, nextEnabled)
	})
	if err != nil {
		return false, fwusecase.E(fwusecase.CodeInternal, "failed to enqueue scheduled task", err)
	}
	return claimed, nil
}

func HandleScheduledTaskJob(ctx context.Context, message []byte) error {
	return queue.HandleJSON(handleScheduledTaskJob)(ctx, message)
}

func handleScheduledTaskJob(ctx context.Context, payload ScheduledTaskJobPayload) error {
	payload.ExecutionID = strings.TrimSpace(payload.ExecutionID)
	if payload.ExecutionID == "" {
		return fwusecase.E(fwusecase.CodeValidation, "scheduled execution ID is required", nil)
	}

	if err := models.MarkScheduledTaskExecutionRunning(ctx, payload.ExecutionID); err != nil {
		return err
	}

	if err := models.MarkScheduledTaskExecutionSucceeded(ctx, payload.ExecutionID); err != nil {
		_ = models.MarkScheduledTaskExecutionFailed(ctx, payload.ExecutionID, err.Error())
		return err
	}
	return nil
}

func scheduledTaskInput(name, jobName, scheduleType, scheduleValue, payloadJSON string, enabled bool) (models.ScheduledTaskInput, error) {
	name = strings.TrimSpace(name)
	jobName = strings.TrimSpace(jobName)
	scheduleType = strings.TrimSpace(scheduleType)
	scheduleValue = strings.TrimSpace(scheduleValue)
	payloadJSON = strings.TrimSpace(payloadJSON)

	if name == "" {
		return models.ScheduledTaskInput{}, fwusecase.E(fwusecase.CodeValidation, "task name is required", nil)
	}
	if jobName == "" {
		return models.ScheduledTaskInput{}, fwusecase.E(fwusecase.CodeValidation, "job name is required", nil)
	}
	if jobName != BuiltInScheduledTaskJob {
		return models.ScheduledTaskInput{}, fwusecase.E(fwusecase.CodeValidation, "job name is not registered", nil)
	}
	if payloadJSON == "" {
		payloadJSON = "{}"
	}
	if !json.Valid([]byte(payloadJSON)) {
		return models.ScheduledTaskInput{}, fwusecase.E(fwusecase.CodeValidation, "payload JSON is invalid", nil)
	}

	var nextRunAt string
	now := timefmt.NowUTC()
	if enabled {
		next, err := computeNextRunAt(scheduleType, scheduleValue, now)
		if err != nil {
			return models.ScheduledTaskInput{}, err
		}
		nextRunAt = formatTime(next)
	} else if _, err := computeNextRunAt(scheduleType, scheduleValue, now); err != nil {
		return models.ScheduledTaskInput{}, err
	}

	return models.ScheduledTaskInput{
		Name:          name,
		JobName:       jobName,
		ScheduleType:  scheduleType,
		ScheduleValue: scheduleValue,
		PayloadJSON:   payloadJSON,
		Enabled:       enabled,
		NextRunAt:     nextRunAt,
	}, nil
}

func computeNextRunAt(scheduleType, value string, now time.Time) (time.Time, error) {
	switch scheduleType {
	case models.ScheduleTypeCron:
		schedule, err := cron.ParseStandard(value)
		if err != nil {
			return time.Time{}, fwusecase.E(fwusecase.CodeValidation, "invalid cron expression", err)
		}
		return schedule.Next(now), nil
	case models.ScheduleTypeOnceAt:
		parsed, err := time.Parse(time.RFC3339, value)
		if err != nil {
			return time.Time{}, fwusecase.E(fwusecase.CodeValidation, "once_at schedule value must be RFC3339", err)
		}
		return parsed, nil
	default:
		return time.Time{}, fwusecase.E(fwusecase.CodeValidation, "invalid schedule type", nil)
	}
}

func formatTime(value time.Time) string {
	return timefmt.RFC3339(value)
}

func scheduledTaskCoFromModel(task models.ScheduledTask) ScheduledTaskCo {
	return ScheduledTaskCo{
		ID:            task.ID,
		Name:          task.Name,
		JobName:       task.JobName,
		ScheduleType:  task.ScheduleType,
		ScheduleValue: task.ScheduleValue,
		PayloadJSON:   task.PayloadJSON,
		Enabled:       task.Enabled,
		NextRunAt:     task.NextRunAt,
		LastRunAt:     task.LastRunAt,
		CreatedAt:     task.CreatedAt,
		UpdatedAt:     task.UpdatedAt,
	}
}

func scheduledTaskCosFromModels(tasks []models.ScheduledTask) []ScheduledTaskCo {
	result := make([]ScheduledTaskCo, 0, len(tasks))
	for i := range tasks {
		result = append(result, scheduledTaskCoFromModel(tasks[i]))
	}
	return result
}

func scheduledTaskExecutionCoFromModel(execution models.ScheduledTaskExecution) ScheduledTaskExecutionCo {
	return ScheduledTaskExecutionCo{
		ID:           execution.ID,
		TaskID:       execution.TaskID,
		JobName:      execution.JobName,
		MessageID:    execution.MessageID,
		Status:       execution.Status,
		ScheduledAt:  execution.ScheduledAt,
		StartedAt:    execution.StartedAt,
		FinishedAt:   execution.FinishedAt,
		ErrorMessage: execution.ErrorMessage,
		CreatedAt:    execution.CreatedAt,
	}
}

func scheduledTaskExecutionCosFromModels(executions []models.ScheduledTaskExecution) []ScheduledTaskExecutionCo {
	result := make([]ScheduledTaskExecutionCo, 0, len(executions))
	for i := range executions {
		result = append(result, scheduledTaskExecutionCoFromModel(executions[i]))
	}
	return result
}

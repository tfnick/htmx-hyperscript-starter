package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/tfnick/go-svelte-starter/api/framework/queue"
	fwusecase "github.com/tfnick/go-svelte-starter/api/framework/usecase"
	"github.com/tfnick/go-svelte-starter/api/models"
)

type EnqueueHeavyTaskCmd struct {
	UserID      string
	TaskType    string
	PayloadJSON string
}

type EnqueueHeavyTaskResult struct {
	TaskID string
}

type HeavyTaskMessage struct {
	TaskID   string `json:"task_id"`
	TaskType string `json:"task_type"`
	UserID   string `json:"user_id"`
}

type heavyTaskExecutionResult struct {
	ResultJSON string
	Message    string
}

const (
	HeavyTaskTypeTestExport     = "test_export"
	HeavyTaskTypeGeoXDBDownload = "geo_xdb_download"
)

type ListMyTasksQry struct {
	Page     int
	PageSize int
}

type TaskCo struct {
	ID           string `json:"id"`
	UserID       string `json:"user_id"`
	TaskType     string `json:"task_type"`
	Status       string `json:"status"`
	PayloadJSON  string `json:"payload_json"`
	ResultJSON   string `json:"result_json"`
	ErrorMessage string `json:"error_message"`
	RetryCount   int    `json:"retry_count"`
	ClearedAt    string `json:"cleared_at"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

type TasksCo struct {
	Items      []TaskCo             `json:"items"`
	Pagination fwusecase.PageResult `json:"pagination"`
}

type ClearMyTasksCo struct {
	ClearedCount int `json:"cleared_count"`
}

func EnqueueHeavyTask(ctx fwusecase.Context, cmd EnqueueHeavyTaskCmd) (EnqueueHeavyTaskResult, error) {
	if cmd.UserID == "" {
		return EnqueueHeavyTaskResult{}, fwusecase.E(fwusecase.CodeValidation, "user ID is required", nil)
	}
	if cmd.TaskType == "" {
		return EnqueueHeavyTaskResult{}, fwusecase.E(fwusecase.CodeValidation, "task type is required", nil)
	}
	if !isAllowedHeavyTaskEnqueueType(cmd.TaskType) {
		return EnqueueHeavyTaskResult{}, fwusecase.E(fwusecase.CodeValidation, "task type is not allowed", nil)
	}
	if DefaultQueueManager == nil {
		return EnqueueHeavyTaskResult{}, fwusecase.E(fwusecase.CodeInternal, "queue manager is not configured", nil)
	}

	payload := cmd.PayloadJSON
	if payload == "" {
		payload = "{}"
	}
	if !json.Valid([]byte(payload)) {
		return EnqueueHeavyTaskResult{}, fwusecase.E(fwusecase.CodeValidation, "payload must be valid JSON", nil)
	}

	taskID := uuid.Must(uuid.NewV7()).String()

	task := &models.AsyncTask{
		ID:          taskID,
		UserID:      cmd.UserID,
		TaskType:    cmd.TaskType,
		Status:      models.AsyncTaskStatusQueued,
		PayloadJSON: payload,
	}

	err := fwusecase.WithAppTx(ctx, func(txCtx fwusecase.Context) error {
		msgID, err := DefaultQueueManager.SendJSON(txCtx.Std(), queue.SendOptions{
			Queue: queue.QueueHeavyTasks,
		}, HeavyTaskMessage{
			TaskID:   taskID,
			TaskType: cmd.TaskType,
			UserID:   cmd.UserID,
		})
		if err != nil {
			return fmt.Errorf("enqueue heavy task: %w", err)
		}

		task.MessageID = msgID
		return models.InsertAsyncTask(txCtx.Std(), task)
	})
	if err != nil {
		return EnqueueHeavyTaskResult{}, err
	}

	return EnqueueHeavyTaskResult{TaskID: taskID}, nil
}

func EnqueueUserHeavyTask(ctx fwusecase.Context, cmd EnqueueHeavyTaskCmd) (EnqueueHeavyTaskResult, error) {
	if !isAllowedUserSubmittedHeavyTaskType(cmd.TaskType) {
		return EnqueueHeavyTaskResult{}, fwusecase.E(fwusecase.CodeValidation, "task type is not allowed", nil)
	}
	return EnqueueHeavyTask(ctx, cmd)
}

func HandleHeavyTaskMessage(ctx context.Context, message []byte) error {
	return queue.HandleJSON(handleHeavyTaskMessage)(ctx, message)
}

func handleHeavyTaskMessage(ctx context.Context, msg HeavyTaskMessage) error {
	task, err := models.GetAsyncTaskByID(ctx, msg.TaskID)
	if err != nil {
		return fmt.Errorf("get async task: %w", err)
	}

	if task.Status == models.AsyncTaskStatusCompleted {
		return nil
	}

	if task.RetryCount >= models.MaxAsyncTaskRetries {
		_ = models.UpdateAsyncTaskStatus(ctx, task.ID, models.AsyncTaskStatusFailed, "{}", "max retries exceeded")
		publishHeavyTaskFinished(ctx, *task, models.AsyncTaskStatusFailed, "max retries exceeded")
		return nil
	}

	_ = models.UpdateAsyncTaskStatus(ctx, task.ID, models.AsyncTaskStatusProcessing, "{}", "")

	result, err := executeHeavyTask(ctx, msg, *task)

	if err != nil {
		_ = models.IncrementAsyncTaskRetryCount(ctx, task.ID, err.Error())

		if task.RetryCount+1 >= models.MaxAsyncTaskRetries {
			_ = models.UpdateAsyncTaskStatus(ctx, task.ID, models.AsyncTaskStatusFailed, "{}", err.Error())
			publishHeavyTaskFinished(ctx, *task, models.AsyncTaskStatusFailed, heavyTaskFailedMessage(*task, err))
			return nil
		}
		return err
	}

	if strings.TrimSpace(result.ResultJSON) == "" {
		result.ResultJSON = "{}"
	}
	if strings.TrimSpace(result.Message) == "" {
		result.Message = "Task completed"
	}
	_ = models.UpdateAsyncTaskStatus(ctx, task.ID, models.AsyncTaskStatusCompleted, result.ResultJSON, "")
	publishHeavyTaskFinished(ctx, *task, models.AsyncTaskStatusCompleted, result.Message)
	return nil
}

func executeHeavyTask(ctx context.Context, msg HeavyTaskMessage, task models.AsyncTask) (heavyTaskExecutionResult, error) {
	switch msg.TaskType {
	case HeavyTaskTypeTestExport:
		return heavyTaskExecutionResult{ResultJSON: "{}", Message: "Task completed"}, nil
	case HeavyTaskTypeGeoXDBDownload:
		return executeGeoXDBDownloadTask(ctx, task)
	case HeavyTaskTypeOrdersExcelExport:
		result, err := executeOrdersExcelExportTask(ctx, task)
		return heavyTaskExecutionResult{
			ResultJSON: result.ResultJSON,
			Message:    result.Message,
		}, err
	default:
		return heavyTaskExecutionResult{}, fmt.Errorf("unknown task type: %s", msg.TaskType)
	}
}

func isAllowedHeavyTaskEnqueueType(taskType string) bool {
	switch strings.TrimSpace(taskType) {
	case HeavyTaskTypeTestExport, HeavyTaskTypeOrdersExcelExport, HeavyTaskTypeGeoXDBDownload:
		return true
	default:
		return false
	}
}

func isAllowedUserSubmittedHeavyTaskType(taskType string) bool {
	switch strings.TrimSpace(taskType) {
	case HeavyTaskTypeTestExport:
		return true
	default:
		return false
	}
}

func publishHeavyTaskFinished(ctx context.Context, task models.AsyncTask, status string, message string) {
	if isOrderExportTaskType(task.TaskType) {
		publishOrderExportTaskFinished(ctx, task, status, message)
		return
	}

	ucCtx := fwusecase.NewContext(ctx, fwusecase.SurfaceSystem)
	_, _ = SendNotification(ucCtx, SendNotificationCmd{
		StorePolicy:  StorePolicyStore,
		MessageType:  RealtimeMessageTypeHeavyTask,
		Presentation: RealtimePresentationToast,
		SourceType:   "async_task",
		SourceID:     task.ID,
		UserID:       task.UserID,
		Title:        heavyTaskNotificationTitle(status),
		Summary:      message,
		Payload: map[string]interface{}{
			"task_id": task.ID,
			"status":  status,
			"message": message,
		},
	})
}

func heavyTaskNotificationTitle(status string) string {
	if status == models.AsyncTaskStatusFailed {
		return "Task failed"
	}
	return "Task completed"
}

func heavyTaskFailedMessage(task models.AsyncTask, err error) string {
	if isOrderExportTaskType(task.TaskType) {
		return "Order export failed"
	}
	if task.TaskType == HeavyTaskTypeGeoXDBDownload {
		return "Geo xdb download failed"
	}
	if err != nil {
		return err.Error()
	}
	return "Task failed"
}

func ListMyTasks(ctx fwusecase.Context, qry ListMyTasksQry) (TasksCo, error) {
	userID := ctx.Actor.UserID
	if userID == "" {
		return TasksCo{}, fwusecase.E(fwusecase.CodeUnauthorized, "not logged in", nil)
	}

	pageQry, err := fwusecase.NormalizePageQuery(fwusecase.PageQuery{
		Page:     qry.Page,
		PageSize: qry.PageSize,
	})
	if err != nil {
		return TasksCo{}, err
	}

	limit := pageQry.Limit()
	offset := pageQry.Offset()

	total, err := models.CountAsyncTasksByUser(ctx.Std(), userID)
	if err != nil {
		return TasksCo{}, fmt.Errorf("count async tasks: %w", err)
	}

	tasks, err := models.ListAsyncTasksByUser(ctx.Std(), userID, limit, offset)
	if err != nil {
		return TasksCo{}, fmt.Errorf("list async tasks: %w", err)
	}

	items := make([]TaskCo, 0, len(tasks))
	for _, t := range tasks {
		items = append(items, TaskCo{
			ID:           t.ID,
			UserID:       t.UserID,
			TaskType:     t.TaskType,
			Status:       t.Status,
			PayloadJSON:  t.PayloadJSON,
			ResultJSON:   t.ResultJSON,
			ErrorMessage: t.ErrorMessage,
			RetryCount:   t.RetryCount,
			ClearedAt:    t.ClearedAt,
			CreatedAt:    t.CreatedAt,
			UpdatedAt:    t.UpdatedAt,
		})
	}

	return TasksCo{
		Items:      items,
		Pagination: fwusecase.NewPageResult(pageQry, total),
	}, nil
}

func ClearMyTasks(ctx fwusecase.Context) (ClearMyTasksCo, error) {
	userID := ctx.Actor.UserID
	if userID == "" {
		return ClearMyTasksCo{}, fwusecase.E(fwusecase.CodeUnauthorized, "not logged in", nil)
	}

	count, err := models.ClearTerminalAsyncTasksByUser(ctx.Std(), userID)
	if err != nil {
		return ClearMyTasksCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to clear tasks", err)
	}
	return ClearMyTasksCo{ClearedCount: count}, nil
}

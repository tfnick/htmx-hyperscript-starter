package routes

import (
	"github.com/labstack/echo/v4"
	fwcontext "github.com/tfnick/go-svelte-starter/api/framework/http/context"
	"github.com/tfnick/go-svelte-starter/api/framework/http/middleware"
	fwrequest "github.com/tfnick/go-svelte-starter/api/framework/http/request"
	httpresponse "github.com/tfnick/go-svelte-starter/api/framework/http/response"
	"github.com/tfnick/go-svelte-starter/api/usecase"
)

type EnqueueTaskRequest struct {
	TaskType    string `json:"task_type"`
	PayloadJSON string `json:"payload_json,omitempty"`
}

type EnqueueTaskResponse struct {
	TaskID  string `json:"task_id"`
	Message string `json:"message"`
}

type TaskResponse struct {
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

type TasksResponse struct {
	Items      []TaskResponse   `json:"items"`
	Pagination paginationResult `json:"pagination"`
}

type TaskDownloadResponse struct {
	URL       string `json:"url"`
	ExpiresAt string `json:"expires_at"`
	Filename  string `json:"filename"`
}

type ClearTasksResponse struct {
	ClearedCount int `json:"cleared_count"`
}

type paginationResult struct {
	Page        int  `json:"page"`
	PageSize    int  `json:"page_size"`
	TotalItems  int  `json:"total_items"`
	TotalPages  int  `json:"total_pages"`
	HasPrevious bool `json:"has_previous"`
	HasNext     bool `json:"has_next"`
}

func ToTaskResponse(task usecase.TaskCo) TaskResponse {
	return TaskResponse{
		ID:           task.ID,
		UserID:       task.UserID,
		TaskType:     task.TaskType,
		Status:       task.Status,
		PayloadJSON:  task.PayloadJSON,
		ResultJSON:   task.ResultJSON,
		ErrorMessage: task.ErrorMessage,
		RetryCount:   task.RetryCount,
		ClearedAt:    task.ClearedAt,
		CreatedAt:    task.CreatedAt,
		UpdatedAt:    task.UpdatedAt,
	}
}

func ToTasksResponse(tasks usecase.TasksCo) TasksResponse {
	items := make([]TaskResponse, 0, len(tasks.Items))
	for _, t := range tasks.Items {
		items = append(items, ToTaskResponse(t))
	}
	return TasksResponse{
		Items: items,
		Pagination: paginationResult{
			Page:        tasks.Pagination.Page,
			PageSize:    tasks.Pagination.PageSize,
			TotalItems:  tasks.Pagination.TotalItems,
			TotalPages:  tasks.Pagination.TotalPages,
			HasPrevious: tasks.Pagination.HasPrevious,
			HasNext:     tasks.Pagination.HasNext,
		},
	}
}

func EnqueueTask(c echo.Context) error {
	currentUser := middleware.GetCurrentUser(c)
	if currentUser == nil {
		return httpresponse.Unauthorized(c, "not logged in")
	}

	var req EnqueueTaskRequest
	if err := c.Bind(&req); err != nil {
		return httpresponse.BadRequest(c, "invalid request data")
	}

	ctx := fwcontext.InternalUsecaseContext(c)
	result, err := usecase.EnqueueUserHeavyTask(ctx, usecase.EnqueueHeavyTaskCmd{
		UserID:      currentUser.ID,
		TaskType:    req.TaskType,
		PayloadJSON: req.PayloadJSON,
	})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}

	return httpresponse.Created(c, EnqueueTaskResponse{
		TaskID:  result.TaskID,
		Message: "task enqueued",
	})
}

func ListMyTasks(c echo.Context) error {
	currentUser := middleware.GetCurrentUser(c)
	if currentUser == nil {
		return httpresponse.Unauthorized(c, "not logged in")
	}

	pageQry := fwrequest.PageQuery(c)

	ctx := fwcontext.InternalUsecaseContext(c)
	tasks, err := usecase.ListMyTasks(ctx, usecase.ListMyTasksQry{
		Page:     pageQry.Page,
		PageSize: pageQry.PageSize,
	})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}

	return httpresponse.OK(c, ToTasksResponse(tasks))
}

func ClearMyTasks(c echo.Context) error {
	currentUser := middleware.GetCurrentUser(c)
	if currentUser == nil {
		return httpresponse.Unauthorized(c, "not logged in")
	}

	ctx := fwcontext.InternalUsecaseContext(c)
	result, err := usecase.ClearMyTasks(ctx)
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}

	return httpresponse.OK(c, ClearTasksResponse{
		ClearedCount: result.ClearedCount,
	})
}

func GetMyTaskDownload(c echo.Context) error {
	currentUser := middleware.GetCurrentUser(c)
	if currentUser == nil {
		return httpresponse.Unauthorized(c, "not logged in")
	}

	ctx := fwcontext.InternalUsecaseContext(c)
	result, err := usecase.GetMyTaskDownload(ctx, usecase.TaskDownloadQry{
		TaskID: c.Param("id"),
	})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}

	return httpresponse.OK(c, TaskDownloadResponse{
		URL:       result.URL,
		ExpiresAt: result.ExpiresAt,
		Filename:  result.Filename,
	})
}

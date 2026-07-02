package routes

import (
	"github.com/labstack/echo/v4"
	fwcontext "github.com/tfnick/go-svelte-starter/api/framework/http/context"
	fwrequest "github.com/tfnick/go-svelte-starter/api/framework/http/request"
	httpresponse "github.com/tfnick/go-svelte-starter/api/framework/http/response"
	"github.com/tfnick/go-svelte-starter/api/usecase"
)

type ScheduledTaskRequest struct {
	Name          string `json:"name"`
	JobName       string `json:"job_name"`
	ScheduleType  string `json:"schedule_type"`
	ScheduleValue string `json:"schedule_value"`
	PayloadJSON   string `json:"payload_json"`
	Enabled       bool   `json:"enabled"`
}

type SetScheduledTaskEnabledRequest struct {
	Enabled bool `json:"enabled"`
}

type ScheduledTaskResponse struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	JobName       string `json:"job_name"`
	ScheduleType  string `json:"schedule_type"`
	ScheduleValue string `json:"schedule_value"`
	PayloadJSON   string `json:"payload_json"`
	Enabled       bool   `json:"enabled"`
	NextRunAt     string `json:"next_run_at"`
	LastRunAt     string `json:"last_run_at"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

type ScheduledTaskExecutionResponse struct {
	ID           string `json:"id"`
	TaskID       string `json:"task_id"`
	JobName      string `json:"job_name"`
	MessageID    string `json:"message_id"`
	Status       string `json:"status"`
	ScheduledAt  string `json:"scheduled_at"`
	StartedAt    string `json:"started_at"`
	FinishedAt   string `json:"finished_at"`
	ErrorMessage string `json:"error_message"`
	CreatedAt    string `json:"created_at"`
}

type ScheduledTaskExecutionsResponse struct {
	Items      []ScheduledTaskExecutionResponse `json:"items"`
	Pagination PaginationResponse               `json:"pagination"`
}

func ListScheduledTasks(c echo.Context) error {
	ctx := fwcontext.InternalUsecaseContext(c)
	tasks, err := usecase.ListScheduledTasks(ctx, usecase.ScheduledTaskQry{})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}
	return httpresponse.OK(c, toScheduledTaskResponses(tasks))
}

func CreateScheduledTask(c echo.Context) error {
	var req ScheduledTaskRequest
	if err := c.Bind(&req); err != nil {
		return httpresponse.BadRequest(c, "invalid request data")
	}

	ctx := fwcontext.InternalUsecaseContext(c)
	task, err := usecase.CreateScheduledTask(ctx, usecase.CreateScheduledTaskCmd{
		Name:          req.Name,
		JobName:       req.JobName,
		ScheduleType:  req.ScheduleType,
		ScheduleValue: req.ScheduleValue,
		PayloadJSON:   req.PayloadJSON,
		Enabled:       req.Enabled,
	})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}
	return httpresponse.Created(c, toScheduledTaskResponse(task))
}

func UpdateScheduledTask(c echo.Context) error {
	var req ScheduledTaskRequest
	if err := c.Bind(&req); err != nil {
		return httpresponse.BadRequest(c, "invalid request data")
	}

	ctx := fwcontext.InternalUsecaseContext(c)
	task, err := usecase.UpdateScheduledTask(ctx, usecase.UpdateScheduledTaskCmd{
		ID:            c.Param("id"),
		Name:          req.Name,
		JobName:       req.JobName,
		ScheduleType:  req.ScheduleType,
		ScheduleValue: req.ScheduleValue,
		PayloadJSON:   req.PayloadJSON,
		Enabled:       req.Enabled,
	})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}
	return httpresponse.OK(c, toScheduledTaskResponse(task))
}

func SetScheduledTaskEnabled(c echo.Context) error {
	var req SetScheduledTaskEnabledRequest
	if err := c.Bind(&req); err != nil {
		return httpresponse.BadRequest(c, "invalid request data")
	}

	ctx := fwcontext.InternalUsecaseContext(c)
	task, err := usecase.SetScheduledTaskEnabled(ctx, usecase.SetScheduledTaskEnabledCmd{
		ID:      c.Param("id"),
		Enabled: req.Enabled,
	})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}
	return httpresponse.OK(c, toScheduledTaskResponse(task))
}

func ListScheduledTaskHistory(c echo.Context) error {
	page := fwrequest.PageQuery(c)
	ctx := fwcontext.InternalUsecaseContext(c)
	history, err := usecase.ListScheduledTaskExecutions(ctx, usecase.ScheduledTaskHistoryQry{
		TaskID:   c.Param("id"),
		Page:     page.Page,
		PageSize: page.PageSize,
	})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}
	return httpresponse.OK(c, toScheduledTaskExecutionsResponse(history))
}

func toScheduledTaskResponse(task usecase.ScheduledTaskCo) ScheduledTaskResponse {
	return ScheduledTaskResponse{
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

func toScheduledTaskResponses(tasks []usecase.ScheduledTaskCo) []ScheduledTaskResponse {
	responses := make([]ScheduledTaskResponse, 0, len(tasks))
	for i := range tasks {
		responses = append(responses, toScheduledTaskResponse(tasks[i]))
	}
	return responses
}

func toScheduledTaskExecutionResponse(execution usecase.ScheduledTaskExecutionCo) ScheduledTaskExecutionResponse {
	return ScheduledTaskExecutionResponse{
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

func toScheduledTaskExecutionResponses(executions []usecase.ScheduledTaskExecutionCo) []ScheduledTaskExecutionResponse {
	responses := make([]ScheduledTaskExecutionResponse, 0, len(executions))
	for i := range executions {
		responses = append(responses, toScheduledTaskExecutionResponse(executions[i]))
	}
	return responses
}

func toScheduledTaskExecutionsResponse(executions usecase.ScheduledTaskExecutionsCo) ScheduledTaskExecutionsResponse {
	return ScheduledTaskExecutionsResponse{
		Items:      toScheduledTaskExecutionResponses(executions.Items),
		Pagination: ToPaginationResponse(executions.Pagination),
	}
}

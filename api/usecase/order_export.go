package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/tfnick/go-svelte-starter/api/framework/timefmt"
	fwusecase "github.com/tfnick/go-svelte-starter/api/framework/usecase"
	"github.com/tfnick/go-svelte-starter/api/models"
	"github.com/tfnick/go-svelte-starter/api/usecase/integrations/oss"
	"github.com/xuri/excelize/v2"
)

const (
	HeavyTaskTypeOrdersExcelExport = "orders_excel_export"

	orderExportContentType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	orderExportBatchSize   = 1000
	orderExportPresignTTL  = time.Hour
)

var orderExportMaxRows = 100000

type EnqueueMyOrdersExcelExportQry struct {
	Status string
}

type EnqueueAdminOrdersExcelExportQry struct {
	UserID string
	Status string
}

type OrderExportTaskCo struct {
	TaskID string
}

type TaskDownloadQry struct {
	TaskID string
}

type TaskDownloadCo struct {
	URL       string
	ExpiresAt string
	Filename  string
}

type orderExportTaskPayload struct {
	Scope           string `json:"scope"`
	RequesterUserID string `json:"requester_user_id"`
	UserID          string `json:"user_id,omitempty"`
	Status          string `json:"status,omitempty"`
	CreatedAt       string `json:"created_at"`
}

type orderExportTaskResult struct {
	ObjectKey    string `json:"object_key"`
	Filename     string `json:"filename"`
	ContentType  string `json:"content_type"`
	Size         int64  `json:"size"`
	RowCount     int    `json:"row_count"`
	ChannelCode  string `json:"channel_code"`
	ProviderCode string `json:"provider_code"`
	AdapterKey   string `json:"adapter_key"`
}

type orderExportExecutionResult struct {
	ResultJSON string
	Message    string
}

func EnqueueMyOrdersExcelExport(ctx fwusecase.Context, qry EnqueueMyOrdersExcelExportQry) (OrderExportTaskCo, error) {
	if !ctx.Actor.Authenticated || strings.TrimSpace(ctx.Actor.UserID) == "" {
		return OrderExportTaskCo{}, fwusecase.E(fwusecase.CodeUnauthorized, "not logged in", nil)
	}

	payload := orderExportTaskPayload{
		Scope:           "user",
		RequesterUserID: strings.TrimSpace(ctx.Actor.UserID),
		UserID:          strings.TrimSpace(ctx.Actor.UserID),
		Status:          strings.TrimSpace(qry.Status),
		CreatedAt:       timefmt.RFC3339Nano(timefmt.NowUTC()),
	}
	query := models.OrderQuery{
		UserID: payload.UserID,
		Status: payload.Status,
	}

	return enqueueOrdersExcelExport(ctx, payload, query)
}

func EnqueueAdminOrdersExcelExport(ctx fwusecase.Context, qry EnqueueAdminOrdersExcelExportQry) (OrderExportTaskCo, error) {
	if !ctx.Actor.Authenticated {
		return OrderExportTaskCo{}, fwusecase.E(fwusecase.CodeUnauthorized, "not logged in", nil)
	}
	if !ctx.Actor.IsAdmin {
		return OrderExportTaskCo{}, fwusecase.E(fwusecase.CodeForbidden, "admin access is required", nil)
	}

	payload := orderExportTaskPayload{
		Scope:           "admin",
		RequesterUserID: strings.TrimSpace(ctx.Actor.UserID),
		UserID:          strings.TrimSpace(qry.UserID),
		Status:          strings.TrimSpace(qry.Status),
		CreatedAt:       timefmt.RFC3339Nano(timefmt.NowUTC()),
	}
	query := models.OrderQuery{
		UserID: payload.UserID,
		Status: payload.Status,
	}

	return enqueueOrdersExcelExport(ctx, payload, query)
}

func enqueueOrdersExcelExport(ctx fwusecase.Context, payload orderExportTaskPayload, query models.OrderQuery) (OrderExportTaskCo, error) {
	if query.Status != "" && !isValidOrderStatus(query.Status) {
		return OrderExportTaskCo{}, fwusecase.E(fwusecase.CodeValidation, "invalid order status", nil)
	}
	if strings.TrimSpace(payload.RequesterUserID) == "" {
		return OrderExportTaskCo{}, fwusecase.E(fwusecase.CodeValidation, "requester user ID is required", nil)
	}
	if _, err := primaryOSSProvider(ctx, "order export storage is not configured"); err != nil {
		return OrderExportTaskCo{}, err
	}

	total, err := models.CountOrders(ctx.Std(), query)
	if err != nil {
		return OrderExportTaskCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to count orders", err)
	}
	if total > orderExportMaxRows {
		return OrderExportTaskCo{}, fwusecase.E(fwusecase.CodeValidation, fmt.Sprintf("order export cannot exceed %d rows", orderExportMaxRows), nil)
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		return OrderExportTaskCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to encode order export payload", err)
	}

	task, err := EnqueueHeavyTask(ctx, EnqueueHeavyTaskCmd{
		UserID:      payload.RequesterUserID,
		TaskType:    HeavyTaskTypeOrdersExcelExport,
		PayloadJSON: string(encoded),
	})
	if err != nil {
		return OrderExportTaskCo{}, err
	}
	return OrderExportTaskCo{TaskID: task.TaskID}, nil
}

func executeOrdersExcelExportTask(ctx context.Context, task models.AsyncTask) (orderExportExecutionResult, error) {
	payload, err := parseOrderExportPayload(task.PayloadJSON)
	if err != nil {
		return orderExportExecutionResult{}, err
	}

	ucCtx := fwusecase.NewContext(ctx, fwusecase.SurfaceSystem)
	ucCtx.Actor = fwusecase.ActorContext{
		Authenticated: true,
		UserID:        payload.RequesterUserID,
		IsAdmin:       payload.Scope == "admin",
	}

	query, err := orderExportQueryFromPayload(payload)
	if err != nil {
		return orderExportExecutionResult{}, err
	}

	provider, err := primaryOSSProvider(ucCtx, "order export storage is not configured")
	if err != nil {
		return orderExportExecutionResult{}, err
	}

	result, err := writeAndUploadOrdersExcel(ucCtx, task.ID, query, provider)
	if err != nil {
		return orderExportExecutionResult{}, err
	}

	encoded, err := json.Marshal(result)
	if err != nil {
		return orderExportExecutionResult{}, fwusecase.E(fwusecase.CodeInternal, "failed to encode order export result", err)
	}

	return orderExportExecutionResult{
		ResultJSON: string(encoded),
		Message:    "Order export completed",
	}, nil
}

func parseOrderExportPayload(value string) (orderExportTaskPayload, error) {
	var payload orderExportTaskPayload
	if err := json.Unmarshal([]byte(strings.TrimSpace(value)), &payload); err != nil {
		return orderExportTaskPayload{}, fwusecase.E(fwusecase.CodeInternal, "order export payload is invalid", err)
	}
	payload.Scope = strings.TrimSpace(payload.Scope)
	payload.RequesterUserID = strings.TrimSpace(payload.RequesterUserID)
	payload.UserID = strings.TrimSpace(payload.UserID)
	payload.Status = strings.TrimSpace(payload.Status)
	if payload.Scope == "" {
		payload.Scope = "user"
	}
	return payload, nil
}

func orderExportQueryFromPayload(payload orderExportTaskPayload) (models.OrderQuery, error) {
	if payload.Status != "" && !isValidOrderStatus(payload.Status) {
		return models.OrderQuery{}, fwusecase.E(fwusecase.CodeValidation, "invalid order status", nil)
	}

	switch payload.Scope {
	case "user":
		if payload.RequesterUserID == "" {
			return models.OrderQuery{}, fwusecase.E(fwusecase.CodeValidation, "requester user ID is required", nil)
		}
		return models.OrderQuery{
			UserID: payload.RequesterUserID,
			Status: payload.Status,
		}, nil
	case "admin":
		return models.OrderQuery{
			UserID: payload.UserID,
			Status: payload.Status,
		}, nil
	default:
		return models.OrderQuery{}, fwusecase.E(fwusecase.CodeValidation, "order export scope is invalid", nil)
	}
}

func writeAndUploadOrdersExcel(ctx fwusecase.Context, taskID string, query models.OrderQuery, provider resolvedOSSProvider) (orderExportTaskResult, error) {
	tmp, err := os.CreateTemp("", "orders-export-*.xlsx")
	if err != nil {
		return orderExportTaskResult{}, fwusecase.E(fwusecase.CodeInternal, "failed to create order export file", err)
	}
	defer tmp.Close()
	defer os.Remove(tmp.Name())

	rowCount, err := writeOrdersExcel(ctx, query, tmp)
	if err != nil {
		return orderExportTaskResult{}, err
	}
	if _, err := tmp.Seek(0, 0); err != nil {
		return orderExportTaskResult{}, fwusecase.E(fwusecase.CodeInternal, "failed to prepare order export file", err)
	}
	stat, err := tmp.Stat()
	if err != nil {
		return orderExportTaskResult{}, fwusecase.E(fwusecase.CodeInternal, "failed to inspect order export file", err)
	}

	now := timefmt.NowUTC()
	filename := "orders-" + now.Format("20060102-150405") + ".xlsx"
	objectKey := path.Join("exports", "orders", now.Format("2006"), now.Format("01"), taskID+".xlsx")
	putResult, err := provider.Adapter.PutObject(ctx.Std(), provider.Config, oss.PutObjectRequest{
		Key:         objectKey,
		Body:        tmp,
		Size:        stat.Size(),
		ContentType: orderExportContentType,
		Metadata: map[string]string{
			"task_id":   taskID,
			"filename":  filename,
			"row_count": fmt.Sprintf("%d", rowCount),
		},
	})
	if err != nil {
		return orderExportTaskResult{}, fwusecase.E(fwusecase.CodeInternal, "failed to store order export", err)
	}
	if strings.TrimSpace(putResult.Key) != "" {
		objectKey = putResult.Key
	}

	return orderExportTaskResult{
		ObjectKey:    objectKey,
		Filename:     filename,
		ContentType:  orderExportContentType,
		Size:         stat.Size(),
		RowCount:     rowCount,
		ChannelCode:  provider.Config.ChannelCode,
		ProviderCode: provider.Config.ProviderCode,
		AdapterKey:   provider.Config.AdapterKey,
	}, nil
}

func writeOrdersExcel(ctx fwusecase.Context, query models.OrderQuery, output *os.File) (int, error) {
	file := excelize.NewFile()
	defer file.Close()

	const sheetName = "Orders"
	defaultSheet := file.GetSheetName(0)
	if defaultSheet == "" {
		defaultSheet = "Sheet1"
	}
	if err := file.SetSheetName(defaultSheet, sheetName); err != nil {
		return 0, fwusecase.E(fwusecase.CodeInternal, "failed to prepare order export sheet", err)
	}

	writer, err := file.NewStreamWriter(sheetName)
	if err != nil {
		return 0, fwusecase.E(fwusecase.CodeInternal, "failed to create order export writer", err)
	}

	headers := []interface{}{
		"Order ID",
		"Product ID",
		"Product Name",
		"User ID",
		"User Name",
		"Status",
		"Subscription Status",
		"Amount",
		"Created At",
	}
	if err := writer.SetRow("A1", headers); err != nil {
		return 0, fwusecase.E(fwusecase.CodeInternal, "failed to write order export header", err)
	}

	rowIndex := 2
	rowCount := 0
	err = models.IterateOrders(ctx.Std(), query, orderExportBatchSize, func(batch []models.Order) error {
		names, err := resolveOrderNames(ctx, batch, nil)
		if err != nil {
			return fwusecase.E(fwusecase.CodeInternal, "failed to load order display names", err)
		}

		for i := range batch {
			order := orderCoFromModel(&batch[i], names)
			cell, err := excelize.CoordinatesToCellName(1, rowIndex)
			if err != nil {
				return fwusecase.E(fwusecase.CodeInternal, "failed to locate order export row", err)
			}
			values := []interface{}{
				order.ID,
				order.ProductID,
				order.ProductName,
				order.UserID,
				order.UserName,
				order.Status,
				order.SubscriptionStatus,
				order.Amount,
				order.CreatedAt,
			}
			if err := writer.SetRow(cell, values); err != nil {
				return fwusecase.E(fwusecase.CodeInternal, "failed to write order export row", err)
			}
			rowIndex++
			rowCount++
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	if err := writer.Flush(); err != nil {
		return 0, fwusecase.E(fwusecase.CodeInternal, "failed to flush order export rows", err)
	}
	if err := file.Write(output); err != nil {
		return 0, fwusecase.E(fwusecase.CodeInternal, "failed to write order export workbook", err)
	}
	return rowCount, nil
}

func GetMyTaskDownload(ctx fwusecase.Context, qry TaskDownloadQry) (TaskDownloadCo, error) {
	if !ctx.Actor.Authenticated || strings.TrimSpace(ctx.Actor.UserID) == "" {
		return TaskDownloadCo{}, fwusecase.E(fwusecase.CodeUnauthorized, "not logged in", nil)
	}

	taskID := strings.TrimSpace(qry.TaskID)
	if taskID == "" {
		return TaskDownloadCo{}, fwusecase.E(fwusecase.CodeValidation, "task ID is required", nil)
	}

	task, err := models.GetAsyncTaskByID(ctx.Std(), taskID)
	if err != nil {
		return TaskDownloadCo{}, fwusecase.E(fwusecase.CodeNotFound, "task not found", err)
	}
	if task.UserID != strings.TrimSpace(ctx.Actor.UserID) {
		return TaskDownloadCo{}, fwusecase.E(fwusecase.CodeForbidden, "cannot download another user's task result", nil)
	}
	if task.Status != models.AsyncTaskStatusCompleted {
		return TaskDownloadCo{}, fwusecase.E(fwusecase.CodeConflict, "task is not completed", nil)
	}
	if task.TaskType != HeavyTaskTypeOrdersExcelExport {
		return TaskDownloadCo{}, fwusecase.E(fwusecase.CodeValidation, "task does not have a downloadable result", nil)
	}

	var result orderExportTaskResult
	if err := json.Unmarshal([]byte(strings.TrimSpace(task.ResultJSON)), &result); err != nil {
		return TaskDownloadCo{}, fwusecase.E(fwusecase.CodeInternal, "task result is invalid", err)
	}
	if strings.TrimSpace(result.ObjectKey) == "" {
		return TaskDownloadCo{}, fwusecase.E(fwusecase.CodeInternal, "task result is missing export object", nil)
	}

	provider, err := ossProviderFromMetadata(ctx, result.ChannelCode, result.AdapterKey, "order export storage is not configured")
	if err != nil {
		return TaskDownloadCo{}, err
	}
	contentType := strings.TrimSpace(result.ContentType)
	if contentType == "" {
		contentType = orderExportContentType
	}
	presigned, err := provider.Adapter.PresignObject(ctx.Std(), provider.Config, oss.PresignObjectRequest{
		Key:         result.ObjectKey,
		Method:      http.MethodGet,
		ExpiresIn:   orderExportPresignTTL,
		ContentType: contentType,
	})
	if err != nil {
		return TaskDownloadCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to prepare order export download", err)
	}
	if strings.TrimSpace(presigned.URL) == "" {
		return TaskDownloadCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to prepare order export download", fmt.Errorf("presigned URL is empty"))
	}

	return TaskDownloadCo{
		URL:       presigned.URL,
		ExpiresAt: timefmt.RFC3339Nano(presigned.ExpiresAt),
		Filename:  result.Filename,
	}, nil
}

func publishOrderExportTaskFinished(ctx context.Context, task models.AsyncTask, status string, message string) {
	title := "Order export completed"
	if status == models.AsyncTaskStatusFailed {
		title = "Order export failed"
	}
	payload := map[string]string{
		"task_id": task.ID,
		"status":  status,
		"message": message,
	}
	ucCtx := fwusecase.NewContext(ctx, fwusecase.SurfaceSystem)
	_, _ = SendNotification(ucCtx, SendNotificationCmd{
		StorePolicy:  StorePolicyStore,
		MessageType:  RealtimeMessageTypeAsyncExportTask,
		Presentation: RealtimePresentationToast,
		SourceType:   "async_task",
		SourceID:     task.ID,
		UserID:       task.UserID,
		Title:        title,
		Summary:      message,
		Payload:      payload,
	})
}

func isOrderExportTaskType(taskType string) bool {
	return taskType == HeavyTaskTypeOrdersExcelExport
}

package routes

import (
	"github.com/labstack/echo/v4"
	fwcontext "github.com/tfnick/go-svelte-starter/api/framework/http/context"
	fwrequest "github.com/tfnick/go-svelte-starter/api/framework/http/request"
	httpresponse "github.com/tfnick/go-svelte-starter/api/framework/http/response"
	"github.com/tfnick/go-svelte-starter/api/usecase"
)

type NotificationResponse struct {
	ID                    string `json:"id"`
	NotificationType      string `json:"notification_type"`
	NotificationTypeLabel string `json:"notification_type_label"`
	SourceType            string `json:"source_type"`
	SourceID              string `json:"source_id"`
	UserID                string `json:"user_id"`
	RecipientEmail        string `json:"recipient_email"`
	RecipientPhone        string `json:"recipient_phone"`
	Title                 string `json:"title"`
	Summary               string `json:"summary"`
	PayloadJSON           string `json:"payload_json"`
	Status                string `json:"status"`
	LastError             string `json:"last_error"`
	SentAt                string `json:"sent_at"`
	ClearedAt             string `json:"cleared_at"`
	CreatedAt             string `json:"created_at"`
	UpdatedAt             string `json:"updated_at"`
}

type NotificationsResponse struct {
	Items      []NotificationResponse `json:"items"`
	Pagination PaginationResponse     `json:"pagination"`
}

type ClearNotificationsResponse struct {
	ClearedCount int `json:"cleared_count"`
}

func ListNotifications(c echo.Context) error {
	page := fwrequest.PageQuery(c)
	ctx := fwcontext.InternalUsecaseContext(c)
	notifications, err := usecase.ListNotifications(ctx, usecase.NotificationsQry{
		Page:     page.Page,
		PageSize: page.PageSize,
		Type:     c.QueryParam("type"),
		Email:    c.QueryParam("email"),
		Phone:    c.QueryParam("phone"),
	})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}

	return httpresponse.OK(c, ToNotificationsResponse(notifications))
}

func ClearMyNotifications(c echo.Context) error {
	ctx := fwcontext.InternalUsecaseContext(c)
	result, err := usecase.ClearMyNotifications(ctx)
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}

	return httpresponse.OK(c, ClearNotificationsResponse{
		ClearedCount: result.ClearedCount,
	})
}

func ToNotificationResponse(notification usecase.NotificationCo) NotificationResponse {
	return NotificationResponse{
		ID:                    notification.ID,
		NotificationType:      notification.NotificationType,
		NotificationTypeLabel: notification.NotificationTypeLabel,
		SourceType:            notification.SourceType,
		SourceID:              notification.SourceID,
		UserID:                notification.UserID,
		RecipientEmail:        notification.RecipientEmail,
		RecipientPhone:        notification.RecipientPhone,
		Title:                 notification.Title,
		Summary:               notification.Summary,
		PayloadJSON:           notification.PayloadJSON,
		Status:                notification.Status,
		LastError:             notification.LastError,
		SentAt:                notification.SentAt,
		ClearedAt:             notification.ClearedAt,
		CreatedAt:             notification.CreatedAt,
		UpdatedAt:             notification.UpdatedAt,
	}
}

func ToNotificationResponses(notifications []usecase.NotificationCo) []NotificationResponse {
	responses := make([]NotificationResponse, 0, len(notifications))
	for i := range notifications {
		responses = append(responses, ToNotificationResponse(notifications[i]))
	}
	return responses
}

func ToNotificationsResponse(notifications usecase.NotificationsCo) NotificationsResponse {
	return NotificationsResponse{
		Items:      ToNotificationResponses(notifications.Items),
		Pagination: ToPaginationResponse(notifications.Pagination),
	}
}

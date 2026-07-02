package usecase

import (
	"encoding/json"
	"strings"

	"github.com/tfnick/go-svelte-starter/api/framework/realtime"
	"github.com/tfnick/go-svelte-starter/api/framework/timefmt"
	fwusecase "github.com/tfnick/go-svelte-starter/api/framework/usecase"
	"github.com/tfnick/go-svelte-starter/api/models"
)

const (
	DictionaryTypeNotificationType = "notification_type"
	NotificationTypeRealtime       = "realtime"
)

type CreateNotificationCmd struct {
	NotificationType string
	SourceType       string
	SourceID         string
	UserID           string
	RecipientEmail   string
	RecipientPhone   string
	Title            string
	Summary          string
	PayloadJSON      string
}

type NotificationStorePolicy string

const (
	StorePolicyDefault   NotificationStorePolicy = ""
	StorePolicyStore     NotificationStorePolicy = "store"
	StorePolicyTransient NotificationStorePolicy = "transient"

	RealtimeMessageTypePoints          = string(realtime.MessageTypePoints)
	RealtimeMessageTypeAsyncExportTask = string(realtime.MessageTypeAsyncExportTask)
	RealtimeMessageTypeNotification    = string(realtime.MessageTypeNotification)
	RealtimeMessageTypeHeavyTask       = string(realtime.MessageTypeHeavyTask)

	RealtimePresentationRefresh = string(realtime.PresentationRefresh)
	RealtimePresentationToast   = string(realtime.PresentationToast)
)

type SendNotificationCmd struct {
	StorePolicy      NotificationStorePolicy
	MessageType      string
	Presentation     string
	Payload          interface{}
	NotificationType string
	SourceType       string
	SourceID         string
	UserID           string
	RecipientEmail   string
	RecipientPhone   string
	Title            string
	Summary          string
	PayloadJSON      string
}

type NotificationsQry struct {
	Page     int
	PageSize int
	Type     string
	Email    string
	Phone    string
}

type NotificationCo struct {
	ID                    string
	NotificationType      string
	NotificationTypeLabel string
	SourceType            string
	SourceID              string
	UserID                string
	RecipientEmail        string
	RecipientPhone        string
	Title                 string
	Summary               string
	PayloadJSON           string
	Status                string
	LastError             string
	SentAt                string
	ClearedAt             string
	CreatedAt             string
	UpdatedAt             string
}

type NotificationsCo struct {
	Items      []NotificationCo
	Pagination fwusecase.PageResult
}

type ClearMyNotificationsCo struct {
	ClearedCount int
}

func SendNotification(ctx fwusecase.Context, cmd SendNotificationCmd) (NotificationCo, error) {
	storePolicy, err := normalizeStorePolicy(cmd.StorePolicy)
	if err != nil {
		return NotificationCo{}, err
	}

	if storePolicy == StorePolicyTransient {
		if err := publishTransientRealtimeMessage(cmd); err != nil {
			if fwusecase.CodeOf(err) != fwusecase.CodeInternal {
				return NotificationCo{}, err
			}
			return NotificationCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to publish realtime notification", err)
		}
		return NotificationCo{}, nil
	}

	notificationType := strings.TrimSpace(cmd.NotificationType)
	if notificationType == "" {
		notificationType = NotificationTypeRealtime
	}
	payloadJSON, err := notificationPayloadJSON(cmd)
	if err != nil {
		return NotificationCo{}, err
	}

	return createNotification(ctx, CreateNotificationCmd{
		NotificationType: notificationType,
		SourceType:       cmd.SourceType,
		SourceID:         cmd.SourceID,
		UserID:           cmd.UserID,
		RecipientEmail:   cmd.RecipientEmail,
		RecipientPhone:   cmd.RecipientPhone,
		Title:            cmd.Title,
		Summary:          cmd.Summary,
		PayloadJSON:      payloadJSON,
	}, func(notification models.Notification) (string, error) {
		return publishStoredRealtimeMessage(notification, cmd)
	})
}

func CreateNotification(ctx fwusecase.Context, cmd CreateNotificationCmd) (NotificationCo, error) {
	return createNotification(ctx, cmd, publishRealtimeNotification)
}

type notificationPublisher func(models.Notification) (string, error)

func createNotification(ctx fwusecase.Context, cmd CreateNotificationCmd, publisher notificationPublisher) (NotificationCo, error) {
	notification, labels, err := notificationInput(ctx, cmd)
	if err != nil {
		return NotificationCo{}, err
	}

	if notification.NotificationType != NotificationTypeRealtime {
		notification.Status = models.NotificationStatusSkipped
	}

	if err := models.InsertNotification(ctx.Std(), &notification); err != nil {
		return NotificationCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to create notification", err)
	}

	if notification.NotificationType == NotificationTypeRealtime {
		published, err := publisher(notification)
		if err != nil {
			notification.Status = models.NotificationStatusFailed
			notification.LastError = "failed to publish realtime notification"
			if updateErr := models.UpdateNotificationStatus(ctx.Std(), notification.ID, notification.Status, notification.LastError, ""); updateErr != nil {
				return NotificationCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to update notification status", updateErr)
			}
			return NotificationCo{}, fwusecase.E(fwusecase.CodeInternal, notification.LastError, err)
		}

		notification.Status = models.NotificationStatusSent
		notification.LastError = ""
		notification.SentAt = published
		if err := models.UpdateNotificationStatus(ctx.Std(), notification.ID, notification.Status, notification.LastError, notification.SentAt); err != nil {
			return NotificationCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to update notification status", err)
		}
	}

	return notificationCoFromModel(notification, labels), nil
}

func ListNotifications(ctx fwusecase.Context, qry NotificationsQry) (NotificationsCo, error) {
	pageQuery, err := fwusecase.NormalizePageQuery(fwusecase.PageQuery{
		Page:     qry.Page,
		PageSize: qry.PageSize,
	})
	if err != nil {
		return NotificationsCo{}, err
	}

	labels, err := notificationTypeLabels(ctx)
	if err != nil {
		return NotificationsCo{}, err
	}

	notificationType := strings.TrimSpace(strings.ToLower(qry.Type))
	if notificationType != "" {
		if _, ok := labels[notificationType]; !ok {
			return NotificationsCo{}, fwusecase.E(fwusecase.CodeValidation, "notification type is invalid", nil)
		}
	}

	query := models.NotificationQuery{
		NotificationType: notificationType,
		RecipientEmail:   containsLike(strings.TrimSpace(qry.Email)),
		RecipientPhone:   containsLike(strings.TrimSpace(qry.Phone)),
		Limit:            pageQuery.Limit(),
		Offset:           pageQuery.Offset(),
	}

	totalItems, err := models.CountNotifications(ctx.Std(), query)
	if err != nil {
		return NotificationsCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to count notifications", err)
	}

	notifications, err := models.ListNotifications(ctx.Std(), query)
	if err != nil {
		return NotificationsCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to load notifications", err)
	}

	return NotificationsCo{
		Items:      notificationCosFromModels(notifications, labels),
		Pagination: fwusecase.NewPageResult(pageQuery, totalItems),
	}, nil
}

func ClearMyNotifications(ctx fwusecase.Context) (ClearMyNotificationsCo, error) {
	userID := strings.TrimSpace(ctx.Actor.UserID)
	if userID == "" {
		return ClearMyNotificationsCo{}, fwusecase.E(fwusecase.CodeUnauthorized, "not logged in", nil)
	}

	count, err := models.ClearNotificationsByUser(ctx.Std(), userID)
	if err != nil {
		return ClearMyNotificationsCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to clear notifications", err)
	}
	return ClearMyNotificationsCo{ClearedCount: count}, nil
}

func normalizeStorePolicy(policy NotificationStorePolicy) (NotificationStorePolicy, error) {
	switch policy {
	case StorePolicyDefault, StorePolicyStore:
		return StorePolicyStore, nil
	case StorePolicyTransient:
		return StorePolicyTransient, nil
	default:
		return "", fwusecase.E(fwusecase.CodeValidation, "notification store policy is invalid", nil)
	}
}

func notificationInput(ctx fwusecase.Context, cmd CreateNotificationCmd) (models.Notification, map[string]string, error) {
	labels, err := notificationTypeLabels(ctx)
	if err != nil {
		return models.Notification{}, nil, err
	}

	notificationType := strings.TrimSpace(strings.ToLower(cmd.NotificationType))
	if notificationType == "" {
		return models.Notification{}, nil, fwusecase.E(fwusecase.CodeValidation, "notification type is required", nil)
	}
	if _, ok := labels[notificationType]; !ok {
		return models.Notification{}, nil, fwusecase.E(fwusecase.CodeValidation, "notification type is invalid", nil)
	}

	userID := strings.TrimSpace(cmd.UserID)
	if notificationType == NotificationTypeRealtime && userID == "" {
		return models.Notification{}, nil, fwusecase.E(fwusecase.CodeValidation, "user ID is required for realtime notification", nil)
	}

	title := strings.TrimSpace(cmd.Title)
	if title == "" {
		return models.Notification{}, nil, fwusecase.E(fwusecase.CodeValidation, "notification title is required", nil)
	}

	payloadJSON, err := normalizeNotificationPayloadJSON(cmd.PayloadJSON)
	if err != nil {
		return models.Notification{}, nil, err
	}

	return models.Notification{
		NotificationType: notificationType,
		SourceType:       strings.TrimSpace(cmd.SourceType),
		SourceID:         strings.TrimSpace(cmd.SourceID),
		UserID:           userID,
		RecipientEmail:   strings.TrimSpace(cmd.RecipientEmail),
		RecipientPhone:   strings.TrimSpace(cmd.RecipientPhone),
		Title:            title,
		Summary:          strings.TrimSpace(cmd.Summary),
		PayloadJSON:      payloadJSON,
		Status:           models.NotificationStatusPending,
	}, labels, nil
}

func notificationTypeLabels(ctx fwusecase.Context) (map[string]string, error) {
	batch, err := GetDictionaries(ctx, DictionaryBatchQry{
		Types: []string{DictionaryTypeNotificationType},
	})
	if err != nil {
		return nil, err
	}

	labels := map[string]string{}
	for _, option := range batch.Dictionaries[DictionaryTypeNotificationType] {
		labels[option.Value] = option.Label
	}
	return labels, nil
}

func normalizeNotificationPayloadJSON(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "{}", nil
	}

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(value), &payload); err != nil {
		return "", fwusecase.E(fwusecase.CodeValidation, "payload JSON is invalid", err)
	}
	if payload == nil {
		return "", fwusecase.E(fwusecase.CodeValidation, "payload JSON must be an object", nil)
	}

	normalized, err := json.Marshal(payload)
	if err != nil {
		return "", fwusecase.E(fwusecase.CodeInternal, "failed to normalize payload JSON", err)
	}
	return string(normalized), nil
}

func notificationPayloadJSON(cmd SendNotificationCmd) (string, error) {
	if cmd.PayloadJSON != "" {
		return normalizeNotificationPayloadJSON(cmd.PayloadJSON)
	}
	if cmd.Payload == nil {
		return "{}", nil
	}

	encoded, err := json.Marshal(cmd.Payload)
	if err != nil {
		return "", fwusecase.E(fwusecase.CodeInternal, "failed to encode notification payload", err)
	}
	return normalizeNotificationPayloadJSON(string(encoded))
}

func publishTransientRealtimeMessage(cmd SendNotificationCmd) error {
	userID := strings.TrimSpace(cmd.UserID)
	if userID == "" {
		return fwusecase.E(fwusecase.CodeValidation, "user ID is required for realtime notification", nil)
	}

	messageType := strings.TrimSpace(cmd.MessageType)
	if messageType == "" {
		messageType = RealtimeMessageTypeNotification
	}

	payload, err := realtimePayload(cmd)
	if err != nil {
		return err
	}

	return publishRealtimeMessage(userID, messageType, strings.TrimSpace(cmd.Presentation), payload)
}

func publishStoredRealtimeMessage(notification models.Notification, cmd SendNotificationCmd) (string, error) {
	messageType := strings.TrimSpace(cmd.MessageType)
	if messageType == "" || messageType == RealtimeMessageTypeNotification {
		return publishRealtimeNotification(notification)
	}

	payload, err := realtimePayload(cmd)
	if err != nil {
		return "", err
	}
	sentAt := timefmt.NowSQLiteDateTime()
	if err := publishRealtimeMessage(notification.UserID, messageType, strings.TrimSpace(cmd.Presentation), payload); err != nil {
		return "", err
	}
	return sentAt, nil
}

func realtimePayload(cmd SendNotificationCmd) (interface{}, error) {
	if cmd.Payload != nil {
		return cmd.Payload, nil
	}
	payloadJSON := strings.TrimSpace(cmd.PayloadJSON)
	if payloadJSON == "" {
		return map[string]interface{}{}, nil
	}

	normalized, err := normalizeNotificationPayloadJSON(payloadJSON)
	if err != nil {
		return nil, err
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(normalized), &payload); err != nil {
		return nil, fwusecase.E(fwusecase.CodeInternal, "failed to decode notification payload", err)
	}
	return payload, nil
}

func publishRealtimeMessage(userID string, messageType string, presentation string, payload interface{}) error {
	return realtime.Publish(userID, realtime.NewMessage(
		realtime.MessageType(messageType),
		realtime.Presentation(presentation),
		payload,
	))
}

func publishRealtimeNotification(notification models.Notification) (string, error) {
	sentAt := timefmt.NowSQLiteDateTime()
	err := publishRealtimeMessage(notification.UserID, RealtimeMessageTypeNotification, "", realtime.NotificationPayload{
		ID:         notification.ID,
		Title:      notification.Title,
		Summary:    notification.Summary,
		SourceType: notification.SourceType,
		SourceID:   notification.SourceID,
		Status:     notificationPayloadStatus(notification.PayloadJSON),
	})
	if err != nil {
		return "", err
	}
	return sentAt, nil
}

func notificationPayloadStatus(payloadJSON string) string {
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(payloadJSON)), &payload); err != nil {
		return ""
	}
	status, _ := payload["status"].(string)
	return strings.TrimSpace(status)
}

func notificationCoFromModel(notification models.Notification, labels map[string]string) NotificationCo {
	label := labels[notification.NotificationType]
	if label == "" {
		label = notification.NotificationType
	}

	return NotificationCo{
		ID:                    notification.ID,
		NotificationType:      notification.NotificationType,
		NotificationTypeLabel: label,
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

func notificationCosFromModels(notifications []models.Notification, labels map[string]string) []NotificationCo {
	result := make([]NotificationCo, 0, len(notifications))
	for i := range notifications {
		result = append(result, notificationCoFromModel(notifications[i], labels))
	}
	return result
}

func containsLike(value string) string {
	if value == "" {
		return ""
	}
	return "%" + value + "%"
}

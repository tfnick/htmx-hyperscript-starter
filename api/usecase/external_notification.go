package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/tfnick/go-svelte-starter/api/framework/data/modelerror"
	fwevents "github.com/tfnick/go-svelte-starter/api/framework/events"
	"github.com/tfnick/go-svelte-starter/api/framework/queue"
	fwusecase "github.com/tfnick/go-svelte-starter/api/framework/usecase"
	"github.com/tfnick/go-svelte-starter/api/models"
	usecaseevents "github.com/tfnick/go-svelte-starter/api/usecase/events"
	"github.com/tfnick/go-svelte-starter/api/usecase/integrations/externalnotification"
)

const externalNotificationNoBotNote = "未配置bot"

var (
	externalNotificationAdaptersMu sync.RWMutex
	externalNotificationAdapters   = map[string]externalnotification.Adapter{}
)

type externalNotificationDeliveryMessage struct {
	DeliveryID string `json:"delivery_id"`
}

func RegisterExternalNotificationAdapter(adapterKey string, adapter externalnotification.Adapter) error {
	key := strings.TrimSpace(adapterKey)
	if key == "" {
		return fmt.Errorf("external notification adapter key is required")
	}
	if adapter == nil {
		return fmt.Errorf("external notification adapter is required")
	}

	externalNotificationAdaptersMu.Lock()
	defer externalNotificationAdaptersMu.Unlock()
	if _, exists := externalNotificationAdapters[key]; exists {
		return fmt.Errorf("external notification adapter already registered: %s", key)
	}
	externalNotificationAdapters[key] = adapter
	return nil
}

func RegisteredExternalNotificationAdapter(adapterKey string) (externalnotification.Adapter, bool) {
	externalNotificationAdaptersMu.RLock()
	defer externalNotificationAdaptersMu.RUnlock()
	adapter, ok := externalNotificationAdapters[strings.TrimSpace(adapterKey)]
	return adapter, ok
}

func RegisterExternalNotificationEventHandlers() error {
	err := fwevents.RegisterTransactional[usecaseevents.SupportLeadCreatedPayload](fwevents.Subscription{
		Topic:      usecaseevents.SupportLeadCreatedTopic,
		Subscriber: usecaseevents.SupportLeadCreatedSubscriber,
	}, handleSupportLeadCreatedExternalNotification)
	if errors.Is(err, fwevents.ErrDuplicateSubscription) {
		return nil
	}
	return err
}

func handleSupportLeadCreatedExternalNotification(txCtx fwusecase.Context, event fwevents.Event, payload usecaseevents.SupportLeadCreatedPayload) error {
	channel, err := models.GetEnabledPrimaryOrDefaultIntegrationChannelConfig(txCtx.Std(), models.IntegrationScenarioExternalNotification)
	if err != nil {
		if errors.Is(err, modelerror.ErrNotFound) {
			_, _, createErr := models.CreateExternalNotificationDelivery(txCtx.Std(), models.CreateExternalNotificationDeliveryCmd{
				EventID:        event.ID,
				IdempotencyKey: event.ID + ":external_notification:no_bot",
				Status:         models.ExternalNotificationDeliveryStatusSkipped,
				Note:           externalNotificationNoBotNote,
			})
			if createErr != nil {
				return createErr
			}
			return nil
		}
		return err
	}

	requestSnapshot, err := json.Marshal(map[string]string{
		"event_topic": event.Topic,
		"lead_id":     payload.LeadID,
		"channel_id":  channel.ID,
	})
	if err != nil {
		return err
	}
	delivery, created, err := models.CreateExternalNotificationDelivery(txCtx.Std(), models.CreateExternalNotificationDeliveryCmd{
		EventID:             event.ID,
		ChannelID:           channel.ID,
		ProviderCode:        channel.ProviderCode,
		AdapterKey:          channel.AdapterKey,
		IdempotencyKey:      event.ID + ":" + channel.ID,
		Status:              models.ExternalNotificationDeliveryStatusQueued,
		RequestSnapshotJSON: string(requestSnapshot),
	})
	if err != nil {
		return err
	}
	if !created {
		return nil
	}
	if DefaultQueueManager == nil {
		return fmt.Errorf("queue manager is not configured")
	}
	messageID, err := DefaultQueueManager.SendJSON(txCtx.Std(), queue.SendOptions{
		Queue: queue.QueueExternalNotifications,
	}, externalNotificationDeliveryMessage{DeliveryID: delivery.ID})
	if err != nil {
		return err
	}
	return models.MarkExternalNotificationDeliveryQueued(txCtx.Std(), delivery.ID, messageID)
}

func HandleExternalNotificationDeliveryJob(ctx context.Context, body []byte) error {
	var message externalNotificationDeliveryMessage
	if err := json.Unmarshal(body, &message); err != nil {
		return err
	}
	deliveryID := strings.TrimSpace(message.DeliveryID)
	if deliveryID == "" {
		return fmt.Errorf("external notification delivery ID is required")
	}

	ucCtx := fwusecase.NewContext(ctx, fwusecase.SurfaceSystem)
	delivery, err := models.GetExternalNotificationDeliveryByID(ucCtx.Std(), deliveryID)
	if err != nil {
		return err
	}
	if delivery.Status == models.ExternalNotificationDeliveryStatusSucceeded ||
		delivery.Status == models.ExternalNotificationDeliveryStatusSkipped {
		return nil
	}
	if err := models.MarkExternalNotificationDeliveryRunning(ucCtx.Std(), delivery.ID); err != nil {
		return err
	}

	event, err := models.GetDomainEvent(ucCtx.Std(), delivery.EventID)
	if err != nil {
		_ = models.MarkExternalNotificationDeliveryFailed(ucCtx.Std(), delivery.ID, "event_not_found", "domain event not found", "{}")
		return nil
	}
	payload, err := decodeSupportLeadCreatedPayload(event)
	if err != nil {
		_ = models.MarkExternalNotificationDeliveryFailed(ucCtx.Std(), delivery.ID, "payload_invalid", "support lead payload is invalid", "{}")
		return nil
	}
	channel, err := models.GetIntegrationChannelConfigByID(ucCtx.Std(), delivery.ChannelID)
	if err != nil {
		_ = models.MarkExternalNotificationDeliveryFailed(ucCtx.Std(), delivery.ID, "config_invalid", "bot channel config is invalid", "{}")
		return nil
	}
	adapter, ok := RegisteredExternalNotificationAdapter(channel.AdapterKey)
	if !ok {
		_ = models.MarkExternalNotificationDeliveryFailed(ucCtx.Std(), delivery.ID, "adapter_not_registered", "bot adapter is not registered", "{}")
		return nil
	}

	result, err := adapter.Send(ctx, externalnotification.ProviderConfig{
		ProviderCode: channel.ProviderCode,
		AdapterKey:   channel.AdapterKey,
		Credential:   channel.CredentialValue,
		ConfigJSON:   channel.ConfigJSON,
	}, externalNotificationMessageForLead(payload))
	if err != nil {
		code, message, retryable := externalNotificationProviderError(err)
		_ = models.MarkExternalNotificationDeliveryFailed(ucCtx.Std(), delivery.ID, code, message, "{}")
		if retryable {
			return err
		}
		return nil
	}
	responseSnapshot := strings.TrimSpace(result.ResponseSnapshot)
	if responseSnapshot == "" {
		responseSnapshot = "{}"
	}
	return models.MarkExternalNotificationDeliverySucceeded(ucCtx.Std(), delivery.ID, responseSnapshot)
}

func decodeSupportLeadCreatedPayload(event models.DomainEvent) (usecaseevents.SupportLeadCreatedPayload, error) {
	if event.Topic != usecaseevents.SupportLeadCreatedTopic {
		return usecaseevents.SupportLeadCreatedPayload{}, fmt.Errorf("unsupported external notification event topic: %s", event.Topic)
	}
	var payload usecaseevents.SupportLeadCreatedPayload
	if err := json.Unmarshal([]byte(event.PayloadJSON), &payload); err != nil {
		return payload, err
	}
	return payload, nil
}

func externalNotificationMessageForLead(payload usecaseevents.SupportLeadCreatedPayload) externalnotification.SendRequest {
	contact := strings.TrimSpace(payload.ContactEmail)
	if contact == "" {
		contact = strings.TrimSpace(payload.ContactPhone)
	} else if phone := strings.TrimSpace(payload.ContactPhone); phone != "" {
		contact += " / " + phone
	}
	return externalnotification.SendRequest{
		Title:      "新销售线索",
		Summary:    payload.NeedDescription,
		EventTopic: usecaseevents.SupportLeadCreatedTopic,
		Fields: []externalnotification.MessageField{
			{Label: "姓名", Value: payload.Name},
			{Label: "公司", Value: payload.Company},
			{Label: "联系方式", Value: contact},
			{Label: "需求", Value: payload.NeedDescription},
			{Label: "来源页面", Value: payload.SourcePage},
			{Label: "意图", Value: payload.DetectedIntent},
			{Label: "会话摘要", Value: payload.ConversationSummary},
		},
	}
}

func externalNotificationProviderError(err error) (string, string, bool) {
	var providerErr externalnotification.ProviderError
	if errors.As(err, &providerErr) {
		code := strings.TrimSpace(providerErr.Code)
		if code == "" {
			code = "provider_error"
		}
		message := strings.TrimSpace(providerErr.Message)
		if message == "" {
			message = "bot provider returned an error"
		}
		return code, message, providerErr.Retryable
	}
	return "provider_error", "bot provider returned an error", false
}

// --- Experiment Bot Test ---

// EnabledBotCo represents an enabled bot channel for the experiment tab.
type EnabledBotCo struct {
	ChannelCode  string
	ProviderCode string
	AdapterKey   string
}

// ListEnabledBots returns all enabled external notification channels.
func ListEnabledBots(ctx fwusecase.Context) ([]EnabledBotCo, error) {
	channels, err := models.ListIntegrationChannelConfigs(ctx.Std(), models.IntegrationScenarioExternalNotification)
	if err != nil {
		return nil, err
	}

	var items []EnabledBotCo
	for _, ch := range channels {
		if ch.Enabled != 1 {
			continue
		}
		items = append(items, EnabledBotCo{
			ChannelCode:  ch.ChannelCode,
			ProviderCode: ch.ProviderCode,
			AdapterKey:   ch.AdapterKey,
		})
	}
	return items, nil
}

// SendTestBotMessageCmd is the command for sending a test bot message.
type SendTestBotMessageCmd struct {
	ChannelCode string
	Title       string
	Content     string
}

// SendBotMessageResult is the result of sending a test bot message.
type SendBotMessageResult struct {
	ResponseSnapshot string
}

// SendTestBotMessage sends a test message via the selected bot channel.
func SendTestBotMessage(ctx fwusecase.Context, cmd SendTestBotMessageCmd) (SendBotMessageResult, error) {
	cmd.ChannelCode = strings.TrimSpace(cmd.ChannelCode)
	if cmd.ChannelCode == "" {
		return SendBotMessageResult{}, fwusecase.E(fwusecase.CodeValidation, "channel_code is required", nil)
	}

	channel, err := models.GetEnabledPrimaryOrDefaultIntegrationChannelConfig(ctx.Std(), models.IntegrationScenarioExternalNotification)
	if err != nil {
		// Try to find a channel by code specifically
		allChannels, listErr := models.ListIntegrationChannelConfigs(ctx.Std(), models.IntegrationScenarioExternalNotification)
		if listErr != nil {
			return SendBotMessageResult{}, fwusecase.E(fwusecase.CodeInternal, "failed to list bot channels", listErr)
		}
		var found bool
		for _, ch := range allChannels {
			if ch.ChannelCode == cmd.ChannelCode && ch.Enabled == 1 {
				channel = ch
				found = true
				break
			}
		}
		if !found {
			return SendBotMessageResult{}, fwusecase.E(fwusecase.CodeNotFound, "bot channel not found or not enabled: "+cmd.ChannelCode, nil)
		}
	} else if channel.ChannelCode != cmd.ChannelCode {
		// If the primary channel doesn't match, search by channel code
		allChannels, listErr := models.ListIntegrationChannelConfigs(ctx.Std(), models.IntegrationScenarioExternalNotification)
		if listErr != nil {
			return SendBotMessageResult{}, fwusecase.E(fwusecase.CodeInternal, "failed to list bot channels", listErr)
		}
		var found bool
		for _, ch := range allChannels {
			if ch.ChannelCode == cmd.ChannelCode && ch.Enabled == 1 {
				channel = ch
				found = true
				break
			}
		}
		if !found {
			return SendBotMessageResult{}, fwusecase.E(fwusecase.CodeNotFound, "bot channel not found or not enabled: "+cmd.ChannelCode, nil)
		}
	}

	adapter, ok := RegisteredExternalNotificationAdapter(channel.AdapterKey)
	if !ok {
		return SendBotMessageResult{}, fwusecase.E(fwusecase.CodeInternal, "bot adapter not registered: "+channel.AdapterKey, nil)
	}

	result, err := adapter.Send(ctx.Std(), externalnotification.ProviderConfig{
		ProviderCode: channel.ProviderCode,
		AdapterKey:   channel.AdapterKey,
		Credential:   channel.CredentialValue,
		ConfigJSON:   channel.ConfigJSON,
	}, externalnotification.SendRequest{
		Title: cmd.Title,
		Fields: []externalnotification.MessageField{
			{Label: "Content", Value: cmd.Content},
		},
	})
	if err != nil {
		return SendBotMessageResult{}, fwusecase.E(fwusecase.CodeInternal, "bot send failed: "+err.Error(), err)
	}

	responseSnapshot := strings.TrimSpace(result.ResponseSnapshot)
	if responseSnapshot == "" {
		responseSnapshot = "{}"
	}

	return SendBotMessageResult{
		ResponseSnapshot: responseSnapshot,
	}, nil
}

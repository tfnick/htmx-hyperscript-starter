package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/tfnick/go-svelte-starter/api/framework/cache"
	"github.com/tfnick/go-svelte-starter/api/framework/data/modelerror"
	"github.com/tfnick/go-svelte-starter/api/framework/integrations/credentials"
	"github.com/tfnick/go-svelte-starter/api/framework/integrations/providererror"
	"github.com/tfnick/go-svelte-starter/api/framework/queue"
	fwusecase "github.com/tfnick/go-svelte-starter/api/framework/usecase"
	"github.com/tfnick/go-svelte-starter/api/models"
	"github.com/tfnick/go-svelte-starter/api/usecase/integrations/payment"
)

type ReceivePaymentWebhookCmd struct {
	ChannelCode string
	Headers     map[string]string
	RawPayload  []byte
}

type PaymentWebhookJobPayload struct {
	ReceiptID string `json:"receipt_id"`
}

type PaymentWebhookReceiptCo struct {
	ID              string
	Status          string
	Duplicate       bool
	ProviderEventID string
	EventType       string
}

func ReceivePaymentWebhook(ctx fwusecase.Context, cmd ReceivePaymentWebhookCmd) (PaymentWebhookReceiptCo, error) {
	channelCode := strings.TrimSpace(cmd.ChannelCode)
	if channelCode == "" {
		return PaymentWebhookReceiptCo{}, fwusecase.E(fwusecase.CodeValidation, "payment channel code is required", nil)
	}
	if len(cmd.RawPayload) == 0 {
		return PaymentWebhookReceiptCo{}, fwusecase.E(fwusecase.CodeValidation, "payment webhook payload is required", nil)
	}
	if DefaultQueueManager == nil {
		return PaymentWebhookReceiptCo{}, fwusecase.E(fwusecase.CodeInternal, "queue manager is not configured", nil)
	}

	config, err := cache.Cached(ctx.Std(), "config.payment", "enabled:webhook_verify", 5*time.Minute, func() (models.IntegrationPaymentConfig, error) {
		return models.GetEnabledPaymentConfig(ctx.Std(), models.PaymentConfigQuery{
			Scenario:    models.IntegrationScenarioPayment,
			ChannelCode: channelCode,
		})
	})
	if err != nil {
		if errors.Is(err, modelerror.ErrNotFound) {
			return PaymentWebhookReceiptCo{}, fwusecase.E(fwusecase.CodeNotFound, "payment channel not found", err)
		}
		return PaymentWebhookReceiptCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to load payment configuration", err)
	}
	if config.Channel.WebhookEnabled != 1 {
		return PaymentWebhookReceiptCo{}, fwusecase.E(fwusecase.CodeForbidden, "payment webhook is not enabled", nil)
	}

	providerCfg, err := paymentProviderConfig(config)
	if err != nil {
		return PaymentWebhookReceiptCo{}, fwusecase.E(fwusecase.CodeInternal, "payment channel is not configured", err)
	}
	adapter, ok := registeredPaymentAdapter(config.Channel.AdapterKey)
	if !ok {
		cause := fmt.Errorf("payment adapter not registered: %s", config.Channel.AdapterKey)
		return PaymentWebhookReceiptCo{}, fwusecase.E(fwusecase.CodeInternal, "payment adapter is not configured", cause)
	}

	normalized, err := adapter.NormalizePaymentWebhook(ctx.Std(), providerCfg, payment.WebhookRequest{
		RawPayload:      cmd.RawPayload,
		Headers:         cmd.Headers,
		VerifySignature: true,
	})
	if err != nil {
		_ = persistFailedPaymentWebhookReceipt(ctx, config, cmd.RawPayload, cmd.Headers, "webhook_verification_failed")
		if providerErr, ok := providererror.From(err); ok && providerErr.Category == providererror.CategoryAuth {
			return PaymentWebhookReceiptCo{}, fwusecase.E(fwusecase.CodeUnauthorized, "payment webhook signature is invalid", err)
		}
		return PaymentWebhookReceiptCo{}, fwusecase.E(fwusecase.CodeValidation, "payment webhook payload is invalid", err)
	}

	receipt, created, err := persistAndEnqueuePaymentWebhookReceipt(ctx, config, normalized, cmd.RawPayload, cmd.Headers)
	if err != nil {
		return PaymentWebhookReceiptCo{}, err
	}
	status := receipt.Status
	if created {
		status = models.IntegrationWebhookReceiptStatusQueued
	}
	return PaymentWebhookReceiptCo{
		ID:              receipt.ID,
		Status:          status,
		Duplicate:       !created,
		ProviderEventID: normalized.ProviderEventID,
		EventType:       normalized.EventType,
	}, nil
}

func HandlePaymentWebhookJob(ctx context.Context, message []byte) error {
	var payload PaymentWebhookJobPayload
	if err := json.Unmarshal(message, &payload); err != nil {
		return err
	}
	payload.ReceiptID = strings.TrimSpace(payload.ReceiptID)
	if payload.ReceiptID == "" {
		return fwusecase.E(fwusecase.CodeValidation, "payment webhook receipt ID is required", nil)
	}

	receipt, err := models.GetIntegrationWebhookReceiptByID(ctx, payload.ReceiptID)
	if err != nil {
		return err
	}
	if receipt.Status == models.IntegrationWebhookReceiptStatusSucceeded || receipt.Status == models.IntegrationWebhookReceiptStatusIgnored {
		return nil
	}
	if err := models.MarkIntegrationWebhookReceiptProcessing(ctx, receipt.ID); err != nil {
		return err
	}

	config, err := cache.Cached(ctx, "config.payment", "enabled:webhook_verify", 5*time.Minute, func() (models.IntegrationPaymentConfig, error) {
		return models.GetEnabledPaymentConfig(ctx, models.PaymentConfigQuery{
			Scenario:    models.IntegrationScenarioPayment,
			ChannelCode: receipt.ChannelCode,
		})
	})
	if err != nil {
		_ = models.MarkIntegrationWebhookReceiptFailed(ctx, receipt.ID, "payment_config_unavailable")
		return err
	}
	providerCfg, err := paymentProviderConfig(config)
	if err != nil {
		_ = models.MarkIntegrationWebhookReceiptFailed(ctx, receipt.ID, "payment_config_invalid")
		return err
	}
	adapter, ok := registeredPaymentAdapter(config.Channel.AdapterKey)
	if !ok {
		err := fmt.Errorf("payment adapter not registered: %s", config.Channel.AdapterKey)
		_ = models.MarkIntegrationWebhookReceiptFailed(ctx, receipt.ID, "payment_adapter_unavailable")
		return err
	}

	rawPayload, err := credentials.DecryptString(receipt.PayloadCiphertext)
	if err != nil {
		_ = models.MarkIntegrationWebhookReceiptFailed(ctx, receipt.ID, "payment_payload_unavailable")
		return err
	}
	normalized, err := adapter.NormalizePaymentWebhook(ctx, providerCfg, payment.WebhookRequest{
		RawPayload: []byte(rawPayload),
	})
	if err != nil {
		_ = models.MarkIntegrationWebhookReceiptFailed(ctx, receipt.ID, "payment_webhook_normalize_failed")
		return nil
	}
	if normalized.BusinessEventType == payment.WebhookEventSubscriptionCanceled {
		ucCtx := fwusecase.NewContext(ctx, fwusecase.SurfaceSystem)
		if err := CancelOrderSubscription(ucCtx, CancelOrderSubscriptionCmd{
			OrderID:                normalized.OrderID,
			ProviderSubscriptionID: normalized.ProviderSubscriptionID,
		}); err != nil {
			if fwusecase.CodeOf(err) == fwusecase.CodeInternal {
				_ = models.MarkIntegrationWebhookReceiptFailed(ctx, receipt.ID, "payment_subscription_update_temporary_failed")
				return err
			}
			_ = models.MarkIntegrationWebhookReceiptFailed(ctx, receipt.ID, "payment_subscription_update_failed")
			return nil
		}
		return models.MarkIntegrationWebhookReceiptSucceeded(ctx, receipt.ID)
	}
	if normalized.BusinessEventType == payment.WebhookEventSubscriptionRenewed {
		subscriptionID := strings.TrimSpace(normalized.ProviderSubscriptionID)
		periodEnd := strings.TrimSpace(normalized.SubscriptionPeriodEnd)
		if subscriptionID == "" || periodEnd == "" {
			_ = models.MarkIntegrationWebhookReceiptFailed(ctx, receipt.ID, "payment_subscription_missing")
			return nil
		}
		order, err := models.GetOrderByProviderSubscriptionID(ctx, subscriptionID)
		if err != nil {
			_ = models.MarkIntegrationWebhookReceiptFailed(ctx, receipt.ID, "payment_subscription_order_missing")
			return nil
		}
		product, err := models.GetProductByID(ctx, order.ProductID)
		if err != nil {
			_ = models.MarkIntegrationWebhookReceiptFailed(ctx, receipt.ID, "payment_product_missing")
			return nil
		}
		if err := models.UpdateUserMembership(ctx, order.UserID, product.MembershipLevel, periodEnd); err != nil {
			_ = models.MarkIntegrationWebhookReceiptFailed(ctx, receipt.ID, "payment_membership_update_failed")
			return nil
		}
		return models.MarkIntegrationWebhookReceiptSucceeded(ctx, receipt.ID)
	}
	if normalized.BusinessEventType != payment.WebhookEventPaymentSucceeded {
		return models.MarkIntegrationWebhookReceiptIgnored(ctx, receipt.ID, "payment_webhook_ignored")
	}
	if strings.TrimSpace(normalized.OrderID) == "" {
		_ = models.MarkIntegrationWebhookReceiptFailed(ctx, receipt.ID, "payment_order_missing")
		return nil
	}

	ucCtx := fwusecase.NewContext(ctx, fwusecase.SurfaceSystem)
	if _, err := PayOrder(ucCtx, PayOrderCmd{
		OrderID:                normalized.OrderID,
		ProviderCheckoutID:     normalized.ProviderPaymentID,
		ProviderOrderID:        normalized.ProviderOrderID,
		ProviderCustomerID:     normalized.ProviderCustomerID,
		ProviderSubscriptionID: normalized.ProviderSubscriptionID,
		ProviderProductID:      normalized.ProviderProductID,
	}); err != nil {
		if fwusecase.CodeOf(err) == fwusecase.CodeInternal {
			_ = models.MarkIntegrationWebhookReceiptFailed(ctx, receipt.ID, "payment_fulfillment_temporary_failed")
			return err
		}
		_ = models.MarkIntegrationWebhookReceiptFailed(ctx, receipt.ID, "payment_fulfillment_failed")
		return nil
	}
	return models.MarkIntegrationWebhookReceiptSucceeded(ctx, receipt.ID)
}

func persistAndEnqueuePaymentWebhookReceipt(ctx fwusecase.Context, config models.IntegrationPaymentConfig, normalized payment.NormalizedWebhook, rawPayload []byte, headers map[string]string) (models.IntegrationWebhookReceipt, bool, error) {
	payloadCiphertext, err := credentials.EncryptString(string(rawPayload))
	if err != nil {
		return models.IntegrationWebhookReceipt{}, false, fwusecase.E(fwusecase.CodeInternal, "failed to protect payment webhook payload", err)
	}

	safeSnapshotJSON, err := safePaymentSnapshotJSON(normalized)
	if err != nil {
		return models.IntegrationWebhookReceipt{}, false, fwusecase.E(fwusecase.CodeInternal, "failed to record payment webhook", err)
	}

	idempotencyKey := paymentWebhookIdempotencyKey(normalized.ProviderEventID, rawPayload)
	var receipt models.IntegrationWebhookReceipt
	created := false
	err = fwusecase.WithAppTx(ctx, func(txCtx fwusecase.Context) error {
		var createErr error
		receipt, created, createErr = models.CreateIntegrationWebhookReceipt(txCtx.Std(), models.CreateIntegrationWebhookReceiptCmd{
			Scenario:          models.IntegrationScenarioPayment,
			ChannelID:         config.Channel.ID,
			ChannelCode:       config.Channel.ChannelCode,
			ProviderCode:      config.Channel.ProviderCode,
			ProviderEventID:   normalized.ProviderEventID,
			IdempotencyKey:    idempotencyKey,
			PayloadHash:       sha256Hex(rawPayload),
			PayloadCiphertext: payloadCiphertext,
			SafeSnapshotJSON:  safeSnapshotJSON,
			HeadersHash:       paymentWebhookHeadersHash(headers),
			Status:            models.IntegrationWebhookReceiptStatusReceived,
		})
		if createErr != nil {
			return createErr
		}
		if !created {
			return nil
		}

		messageID, err := DefaultQueueManager.SendJSON(txCtx.Std(), queue.SendOptions{
			Queue: queue.QueueIntegrationWebhooks,
		}, PaymentWebhookJobPayload{ReceiptID: receipt.ID})
		if err != nil {
			return err
		}
		return models.MarkIntegrationWebhookReceiptQueued(txCtx.Std(), receipt.ID, messageID)
	})
	if err != nil {
		return models.IntegrationWebhookReceipt{}, false, fwusecase.E(fwusecase.CodeInternal, "failed to queue payment webhook", err)
	}
	return receipt, created, nil
}

func persistFailedPaymentWebhookReceipt(ctx fwusecase.Context, config models.IntegrationPaymentConfig, rawPayload []byte, headers map[string]string, errorCode string) error {
	payloadCiphertext, err := credentials.EncryptString(string(rawPayload))
	if err != nil {
		return err
	}
	snapshotJSON, err := json.Marshal(map[string]interface{}{
		"error_code": errorCode,
	})
	if err != nil {
		return err
	}
	_, _, err = models.CreateIntegrationWebhookReceipt(ctx.Std(), models.CreateIntegrationWebhookReceiptCmd{
		Scenario:          models.IntegrationScenarioPayment,
		ChannelID:         config.Channel.ID,
		ChannelCode:       config.Channel.ChannelCode,
		ProviderCode:      config.Channel.ProviderCode,
		IdempotencyKey:    "invalid:" + sha256Hex(rawPayload),
		PayloadHash:       sha256Hex(rawPayload),
		PayloadCiphertext: payloadCiphertext,
		SafeSnapshotJSON:  string(snapshotJSON),
		HeadersHash:       paymentWebhookHeadersHash(headers),
		Status:            models.IntegrationWebhookReceiptStatusFailed,
	})
	return err
}

func safePaymentSnapshotJSON(normalized payment.NormalizedWebhook) (string, error) {
	snapshot := map[string]interface{}{}
	for key, value := range normalized.SafeSnapshot {
		snapshot[key] = value
	}
	snapshot["business_event_type"] = normalized.BusinessEventType
	body, err := json.Marshal(snapshot)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func paymentWebhookIdempotencyKey(providerEventID string, rawPayload []byte) string {
	providerEventID = strings.TrimSpace(providerEventID)
	if providerEventID != "" {
		return providerEventID
	}
	return "payload:" + sha256Hex(rawPayload)
}

func paymentWebhookHeadersHash(headers map[string]string) string {
	if len(headers) == 0 {
		return sha256Hex(nil)
	}
	normalized := map[string]string{}
	for key, value := range headers {
		normalized[strings.ToLower(strings.TrimSpace(key))] = strings.TrimSpace(value)
	}
	keys := make([]string, 0, len(normalized))
	for key := range normalized {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var builder strings.Builder
	for _, key := range keys {
		builder.WriteString(key)
		builder.WriteByte(':')
		builder.WriteString(normalized[key])
		builder.WriteByte('\n')
	}
	return sha256Hex([]byte(builder.String()))
}

func sha256Hex(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

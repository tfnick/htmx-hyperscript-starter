package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/tfnick/go-svelte-starter/api/framework/cache"
	"github.com/tfnick/go-svelte-starter/api/framework/data/modelerror"
	fwusecase "github.com/tfnick/go-svelte-starter/api/framework/usecase"
	"github.com/tfnick/go-svelte-starter/api/models"
	"github.com/tfnick/go-svelte-starter/api/usecase/integrations/payment"
)

type CreateOrderPaymentCheckoutCmd struct {
	OrderID string
}

type PaymentCheckoutCo struct {
	Order             OrderCo
	CheckoutURL       string
	ProviderPaymentID string
	ChannelCode       string
	InvocationID      string
	Status            string
}

type paymentChannelConfig struct {
	BaseURL    string `json:"base_url"`
	ProductID  string `json:"product_id"`
	SuccessURL string `json:"success_url"`
	Units      int    `json:"units"`
}

type paymentCredentialBundle struct {
	APIKey        string `json:"api_key"`
	WebhookSecret string `json:"webhook_secret"`
}

func CreateOrderPaymentCheckout(ctx fwusecase.Context, cmd CreateOrderPaymentCheckoutCmd) (PaymentCheckoutCo, error) {
	orderID := strings.TrimSpace(cmd.OrderID)
	if orderID == "" {
		return PaymentCheckoutCo{}, fwusecase.E(fwusecase.CodeValidation, "order ID is required", nil)
	}

	order, err := models.GetOrderByID(ctx.Std(), orderID)
	if err != nil {
		return PaymentCheckoutCo{}, orderLoadError(err)
	}
	if err := requireOrderAccess(ctx, order.UserID, "cannot create checkout for another user's order"); err != nil {
		return PaymentCheckoutCo{}, err
	}
	if order.Status == "paid" {
		return PaymentCheckoutCo{}, fwusecase.E(fwusecase.CodeConflict, "order is already paid", nil)
	}
	if order.Status != "pending" {
		return PaymentCheckoutCo{}, fwusecase.E(fwusecase.CodeConflict, "only pending orders can be paid", nil)
	}
	if strings.TrimSpace(order.ProductID) == "" {
		return PaymentCheckoutCo{}, fwusecase.E(fwusecase.CodeValidation, "order product is required", nil)
	}

	product, err := models.GetProductByID(ctx.Std(), order.ProductID)
	if err != nil {
		return PaymentCheckoutCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to load order product", err)
	}
	if product.Enabled != 1 {
		return PaymentCheckoutCo{}, fwusecase.E(fwusecase.CodeValidation, "product is disabled", nil)
	}
	if strings.TrimSpace(product.CreemProductID) == "" {
		return PaymentCheckoutCo{}, fwusecase.E(fwusecase.CodeValidation, "product Creem ID is required", nil)
	}

	user, err := models.GetUserByID(ctx.Std(), order.UserID)
	if err != nil {
		return PaymentCheckoutCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to load payment customer", err)
	}

	if product.MembershipLevel != "" && product.MembershipLevel != MembershipLevelBasic {
		effectiveLevel, _ := EffectiveMembership(user)
		if effectiveLevel != MembershipLevelBasic && membershipLevelRank(effectiveLevel) >= membershipLevelRank(product.MembershipLevel) {
			return PaymentCheckoutCo{}, fwusecase.E(fwusecase.CodeConflict, "cannot purchase this product while current membership is active", nil)
		}
	}

	config, err := cache.Cached(ctx.Std(), "config.payment", "enabled:checkout", 5*time.Minute, func() (models.IntegrationPaymentConfig, error) {
		return models.GetEnabledPaymentConfig(ctx.Std(), models.PaymentConfigQuery{
			Scenario:  models.IntegrationScenarioPayment,
			Operation: payment.OperationCreatePayment,
		})
	})
	if err != nil {
		if errors.Is(err, modelerror.ErrNotFound) {
			return PaymentCheckoutCo{}, fwusecase.E(fwusecase.CodeInternal, "payment channel is not configured", err)
		}
		return PaymentCheckoutCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to load payment configuration", err)
	}

	providerCfg, err := paymentProviderConfig(config)
	if err != nil {
		return PaymentCheckoutCo{}, fwusecase.E(fwusecase.CodeInternal, "payment channel is not configured", err)
	}

	adapter, ok := registeredPaymentAdapter(config.Channel.AdapterKey)
	if !ok {
		cause := fmt.Errorf("payment adapter not registered: %s", config.Channel.AdapterKey)
		return PaymentCheckoutCo{}, fwusecase.E(fwusecase.CodeInternal, "payment adapter is not configured", cause)
	}

	startedAt := time.Now()
	invocation, err := models.CreateIntegrationInvocation(ctx.Std(), models.CreateIntegrationInvocationCmd{
		Scenario:       models.IntegrationScenarioPayment,
		ChannelID:      config.Channel.ID,
		ChannelCode:    config.Channel.ChannelCode,
		ProviderCode:   config.Channel.ProviderCode,
		Operation:      payment.OperationCreatePayment,
		IdempotencyKey: order.ID,
	})
	if err != nil {
		return PaymentCheckoutCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to record payment invocation", err)
	}

	result, err := adapter.CreatePayment(ctx.Std(), providerCfg, payment.CreatePaymentRequest{
		OrderID:   order.ID,
		UserID:    order.UserID,
		Amount:    order.Amount,
		Currency:  "USD",
		ProductID: product.CreemProductID,
		Customer: payment.Customer{
			Email: user.Email,
		},
		Metadata: map[string]string{
			"order_id":         order.ID,
			"user_id":          order.UserID,
			"product_id":       order.ProductID,
			"creem_product_id": product.CreemProductID,
			"amount":           fmt.Sprintf("%d", order.Amount),
		},
	})
	if err != nil {
		category, retryable, providerRequestID := providerFailureMetadata(err)
		completeErr := models.CompleteIntegrationInvocation(ctx.Std(), models.CompleteIntegrationInvocationCmd{
			ID:                invocation.ID,
			Status:            models.IntegrationInvocationStatusFailed,
			ProviderRequestID: providerRequestID,
			ErrorCategory:     category,
			Retryable:         retryable,
			DurationMS:        time.Since(startedAt).Milliseconds(),
		})
		if completeErr != nil {
			err = fmt.Errorf("%w; complete payment invocation failed: %v", err, completeErr)
		}
		return PaymentCheckoutCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to create payment checkout", err)
	}

	usageJSON, err := json.Marshal(map[string]interface{}{
		"checkout_status": result.Status,
	})
	if err != nil {
		return PaymentCheckoutCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to record payment invocation", err)
	}
	if err := models.CompleteIntegrationInvocation(ctx.Std(), models.CompleteIntegrationInvocationCmd{
		ID:                invocation.ID,
		Status:            models.IntegrationInvocationStatusSucceeded,
		ProviderRequestID: result.ProviderRequestID,
		UsageJSON:         string(usageJSON),
		DurationMS:        time.Since(startedAt).Milliseconds(),
	}); err != nil {
		return PaymentCheckoutCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to record payment invocation", err)
	}
	if err := models.UpdateOrderProviderRefs(ctx.Std(), order.ID, models.OrderProviderRefs{
		ProviderCheckoutID: result.ProviderPaymentID,
		ProviderProductID:  product.CreemProductID,
	}); err != nil {
		return PaymentCheckoutCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to record payment checkout", err)
	}
	order.ProviderCheckoutID = result.ProviderPaymentID
	order.ProviderProductID = product.CreemProductID

	names, err := resolveOrderNames(ctx, []models.Order{*order}, nil)
	if err != nil {
		return PaymentCheckoutCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to load order display names", err)
	}
	return PaymentCheckoutCo{
		Order:             orderCoFromModel(order, names),
		CheckoutURL:       result.CheckoutURL,
		ProviderPaymentID: result.ProviderPaymentID,
		ChannelCode:       config.Channel.ChannelCode,
		InvocationID:      invocation.ID,
		Status:            result.Status,
	}, nil
}

func paymentProviderConfig(config models.IntegrationPaymentConfig) (payment.ProviderConfig, error) {
	channelConfig, err := parsePaymentConfigJSON(config.Channel.ConfigJSON)
	if err != nil {
		return payment.ProviderConfig{}, fmt.Errorf("parse payment channel config failed: %w", err)
	}
	operationConfig, err := parsePaymentConfigJSON(config.OperationConfig.ConfigJSON)
	if err != nil {
		return payment.ProviderConfig{}, fmt.Errorf("parse payment operation config failed: %w", err)
	}
	channelConfig = mergePaymentConfig(channelConfig, operationConfig)
	if strings.TrimSpace(channelConfig.BaseURL) == "" {
		return payment.ProviderConfig{}, fmt.Errorf("payment base_url is required")
	}

	secrets, err := paymentCredentialSecrets(config.Credential.ValueText)
	if err != nil {
		return payment.ProviderConfig{}, err
	}
	return payment.ProviderConfig{
		ChannelCode:   config.Channel.ChannelCode,
		ProviderCode:  config.Channel.ProviderCode,
		AdapterKey:    config.Channel.AdapterKey,
		BaseURL:       channelConfig.BaseURL,
		APIKey:        secrets.APIKey,
		WebhookSecret: secrets.WebhookSecret,
		ProductID:     channelConfig.ProductID,
		SuccessURL:    channelConfig.SuccessURL,
		Units:         channelConfig.Units,
	}, nil
}

func parsePaymentConfigJSON(raw string) (paymentChannelConfig, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return paymentChannelConfig{}, nil
	}
	var config paymentChannelConfig
	if err := json.Unmarshal([]byte(raw), &config); err != nil {
		return paymentChannelConfig{}, err
	}
	return config, nil
}

func mergePaymentConfig(base paymentChannelConfig, override paymentChannelConfig) paymentChannelConfig {
	if strings.TrimSpace(override.BaseURL) != "" {
		base.BaseURL = override.BaseURL
	}
	if strings.TrimSpace(override.ProductID) != "" {
		base.ProductID = override.ProductID
	}
	if strings.TrimSpace(override.SuccessURL) != "" {
		base.SuccessURL = override.SuccessURL
	}
	if override.Units > 0 {
		base.Units = override.Units
	}
	return base
}

func paymentCredentialSecrets(value string) (paymentCredentialBundle, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return paymentCredentialBundle{}, fmt.Errorf("payment credential is empty")
	}

	if strings.HasPrefix(value, "{") {
		var bundle paymentCredentialBundle
		if err := json.Unmarshal([]byte(value), &bundle); err != nil {
			return paymentCredentialBundle{}, fmt.Errorf("parse payment credential bundle failed: %w", err)
		}
		bundle.APIKey = strings.TrimSpace(bundle.APIKey)
		bundle.WebhookSecret = strings.TrimSpace(bundle.WebhookSecret)
		if bundle.APIKey == "" {
			return paymentCredentialBundle{}, fmt.Errorf("payment api_key is required")
		}
		return bundle, nil
	}

	return paymentCredentialBundle{APIKey: value}, nil
}

func orderLoadError(err error) error {
	if errors.Is(err, modelerror.ErrNotFound) {
		return fwusecase.E(fwusecase.CodeNotFound, "order not found", err)
	}
	if strings.Contains(err.Error(), "sql: no rows") {
		return fwusecase.E(fwusecase.CodeNotFound, "order not found", err)
	}
	return fwusecase.E(fwusecase.CodeInternal, "failed to load order", err)
}

func cancelCreemSubscriptionByID(ctx context.Context, subscriptionID string) error {
	config, err := cache.Cached(ctx, "config.payment", "enabled:cancel_subscription", 5*time.Minute, func() (models.IntegrationPaymentConfig, error) {
		return models.GetEnabledPaymentConfig(ctx, models.PaymentConfigQuery{
			Scenario:  models.IntegrationScenarioPayment,
			Operation: payment.OperationCreatePayment,
		})
	})
	if err != nil {
		return err
	}
	providerCfg, err := paymentProviderConfig(config)
	if err != nil {
		return err
	}
	adapter, ok := registeredPaymentAdapter(config.Channel.AdapterKey)
	if !ok {
		return fmt.Errorf("payment adapter not registered: %s", config.Channel.AdapterKey)
	}
	_, err = adapter.CancelSubscription(ctx, providerCfg, payment.CancelSubscriptionRequest{
		SubscriptionID: subscriptionID,
		Mode:           payment.CancelSubscriptionModeScheduled,
		OnExecute:      payment.CancelSubscriptionOnExecuteCancel,
	})
	return err
}

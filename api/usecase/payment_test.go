package usecase_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
	"testing"

	fwevents "github.com/tfnick/go-svelte-starter/api/framework/events"
	"github.com/tfnick/go-svelte-starter/api/framework/integrations/providererror"
	"github.com/tfnick/go-svelte-starter/api/framework/queue"
	fwusecase "github.com/tfnick/go-svelte-starter/api/framework/usecase"
	"github.com/tfnick/go-svelte-starter/api/models"
	creemadapter "github.com/tfnick/go-svelte-starter/api/providers/payment/creem"
	"github.com/tfnick/go-svelte-starter/api/usecase"
	usecaseevents "github.com/tfnick/go-svelte-starter/api/usecase/events"
	"github.com/tfnick/go-svelte-starter/api/usecase/integrations/payment"
	"github.com/tfnick/sqlx"
)

type fakePaymentAdapter struct {
	t *testing.T
}

func (a fakePaymentAdapter) CreatePayment(ctx context.Context, cfg payment.ProviderConfig, req payment.CreatePaymentRequest) (payment.CreatePaymentResult, error) {
	a.t.Helper()
	if cfg.ChannelCode != "usecase-creem" || cfg.APIKey != "payment-api-key" {
		a.t.Fatalf("unexpected provider config: %#v", cfg)
	}
	if req.OrderID != "o1" || req.ProductID != "prod_usecase" || req.Customer.Email != "ada@example.com" || req.Metadata["order_id"] != "o1" {
		a.t.Fatalf("unexpected payment request: %#v", req)
	}
	return payment.CreatePaymentResult{
		ProviderPaymentID: "ch_usecase",
		CheckoutURL:       "https://checkout.creem.io/ch_usecase",
		Status:            "pending",
		ProviderRequestID: "ch_usecase",
	}, nil
}

func (a fakePaymentAdapter) NormalizePaymentWebhook(ctx context.Context, cfg payment.ProviderConfig, req payment.WebhookRequest) (payment.NormalizedWebhook, error) {
	a.t.Helper()
	if cfg.WebhookSecret != "payment-webhook-secret" {
		a.t.Fatalf("unexpected webhook secret: %q", cfg.WebhookSecret)
	}
	if req.VerifySignature && webhookHeaderForTest(req.Headers, "creem-signature") != "signature" {
		a.t.Fatalf("expected signature header to be forwarded, got %#v", req.Headers)
	}
	return payment.NormalizedWebhook{
		ProviderEventID:        "evt_usecase",
		EventType:              "checkout.completed",
		BusinessEventType:      payment.WebhookEventPaymentSucceeded,
		ProviderPaymentID:      "ch_usecase",
		PaymentStatus:          "succeeded",
		OrderID:                "o1",
		ProviderOrderID:        "ord_usecase",
		ProviderCustomerID:     "cust_usecase",
		ProviderSubscriptionID: "sub_usecase",
		ProviderProductID:      "prod_usecase",
		SafeSnapshot: map[string]interface{}{
			"event_type":          "checkout.completed",
			"provider_event_id":   "evt_usecase",
			"provider_payment_id": "ch_usecase",
			"order_id":            "o1",
		},
	}, nil
}

func (a fakePaymentAdapter) CancelSubscription(ctx context.Context, cfg payment.ProviderConfig, req payment.CancelSubscriptionRequest) (payment.CancelSubscriptionResult, error) {
	a.t.Helper()
	return payment.CancelSubscriptionResult{Status: "canceled"}, nil
}

type authFailingPaymentAdapter struct{}

func (authFailingPaymentAdapter) CreatePayment(ctx context.Context, cfg payment.ProviderConfig, req payment.CreatePaymentRequest) (payment.CreatePaymentResult, error) {
	return payment.CreatePaymentResult{}, nil
}

func (authFailingPaymentAdapter) NormalizePaymentWebhook(ctx context.Context, cfg payment.ProviderConfig, req payment.WebhookRequest) (payment.NormalizedWebhook, error) {
	return payment.NormalizedWebhook{}, providererror.New(providererror.CategoryAuth, false, "payment webhook signature is invalid", nil)
}

func (authFailingPaymentAdapter) CancelSubscription(ctx context.Context, cfg payment.ProviderConfig, req payment.CancelSubscriptionRequest) (payment.CancelSubscriptionResult, error) {
	return payment.CancelSubscriptionResult{}, nil
}

type subscriptionCanceledPaymentAdapter struct {
	t *testing.T
}

func (a subscriptionCanceledPaymentAdapter) CreatePayment(ctx context.Context, cfg payment.ProviderConfig, req payment.CreatePaymentRequest) (payment.CreatePaymentResult, error) {
	a.t.Helper()
	return payment.CreatePaymentResult{}, nil
}

func (a subscriptionCanceledPaymentAdapter) NormalizePaymentWebhook(ctx context.Context, cfg payment.ProviderConfig, req payment.WebhookRequest) (payment.NormalizedWebhook, error) {
	a.t.Helper()
	return payment.NormalizedWebhook{
		ProviderEventID:        "evt_cancel",
		EventType:              "subscription.canceled",
		BusinessEventType:      payment.WebhookEventSubscriptionCanceled,
		PaymentStatus:          "canceled",
		OrderID:                "o1",
		ProviderSubscriptionID: "sub_usecase",
		ProviderProductID:      "prod_usecase",
		SafeSnapshot: map[string]interface{}{
			"event_type":               "subscription.canceled",
			"provider_event_id":        "evt_cancel",
			"provider_subscription_id": "sub_usecase",
			"order_id":                 "o1",
		},
	}, nil
}

func (a subscriptionCanceledPaymentAdapter) CancelSubscription(ctx context.Context, cfg payment.ProviderConfig, req payment.CancelSubscriptionRequest) (payment.CancelSubscriptionResult, error) {
	a.t.Helper()
	return payment.CancelSubscriptionResult{Status: "canceled"}, nil
}

func TestCreateOrderPaymentCheckoutUsesDBConfigAndRecordsInvocation(t *testing.T) {
	manager := setupUsecaseOrderTxDB(t)
	appDB, orderID := seedPayableOrder(t, manager)
	seedPaymentIntegrationConfig(t, appDB, "payment.creem.usecase-test")
	if err := usecase.RegisterPaymentAdapter("payment.creem.usecase-test", fakePaymentAdapter{t: t}); err != nil {
		t.Fatalf("register payment adapter: %v", err)
	}

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	ctx.Actor = fwusecase.ActorContext{Authenticated: true, UserID: "u1"}
	checkout, err := usecase.CreateOrderPaymentCheckout(ctx, usecase.CreateOrderPaymentCheckoutCmd{OrderID: orderID})
	if err != nil {
		t.Fatalf("create payment checkout: %v", err)
	}
	if checkout.CheckoutURL == "" || checkout.ProviderPaymentID != "ch_usecase" || checkout.Order.Status != "pending" {
		t.Fatalf("unexpected checkout: %#v", checkout)
	}

	var status string
	if err := appDB.Get(&status, `SELECT status FROM orders WHERE id = ?`, orderID); err != nil {
		t.Fatalf("get order status: %v", err)
	}
	if status != "pending" {
		t.Fatalf("checkout creation must not mark order paid, got %q", status)
	}
	var checkoutID string
	if err := appDB.Get(&checkoutID, `SELECT provider_checkout_id FROM orders WHERE id = ?`, orderID); err != nil {
		t.Fatalf("get checkout id: %v", err)
	}
	if checkoutID != "ch_usecase" {
		t.Fatalf("expected checkout id to be stored, got %q", checkoutID)
	}

	var invocation struct {
		Status            string `db:"status"`
		ProviderRequestID string `db:"provider_request_id"`
		IdempotencyKey    string `db:"idempotency_key"`
	}
	if err := appDB.Get(&invocation, `SELECT status, provider_request_id, idempotency_key FROM integration_invocations WHERE id = ?`, checkout.InvocationID); err != nil {
		t.Fatalf("load invocation: %v", err)
	}
	if invocation.Status != models.IntegrationInvocationStatusSucceeded || invocation.ProviderRequestID != "ch_usecase" || invocation.IdempotencyKey != orderID {
		t.Fatalf("unexpected invocation: %#v", invocation)
	}
}

func TestCreateOrderPaymentCheckoutRejectsCrossUser(t *testing.T) {
	manager := setupUsecaseOrderTxDB(t)
	appDB, orderID := seedPayableOrder(t, manager)
	seedPaymentIntegrationConfig(t, appDB, "payment.creem.cross-user-test")

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	ctx.Actor = fwusecase.ActorContext{Authenticated: true, UserID: "other-user"}
	_, err := usecase.CreateOrderPaymentCheckout(ctx, usecase.CreateOrderPaymentCheckoutCmd{OrderID: orderID})
	if fwusecase.CodeOf(err) != fwusecase.CodeForbidden {
		t.Fatalf("expected forbidden for cross-user checkout, got %v", err)
	}
}

func TestCreateOrderPaymentCheckoutFallsBackToSingleEnabledChannelWithoutOperationConfig(t *testing.T) {
	manager := setupUsecaseOrderTxDB(t)
	appDB, orderID := seedPayableOrder(t, manager)
	seedPaymentIntegrationConfig(t, appDB, "payment.creem.usecase-fallback-test")
	if _, err := appDB.Exec(`DELETE FROM integration_operation_configs WHERE scenario = 'payment'`); err != nil {
		t.Fatalf("delete payment operation config: %v", err)
	}
	if err := usecase.RegisterPaymentAdapter("payment.creem.usecase-fallback-test", fakePaymentAdapter{t: t}); err != nil {
		t.Fatalf("register payment adapter: %v", err)
	}

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	ctx.Actor = fwusecase.ActorContext{Authenticated: true, UserID: "u1"}
	checkout, err := usecase.CreateOrderPaymentCheckout(ctx, usecase.CreateOrderPaymentCheckoutCmd{OrderID: orderID})
	if err != nil {
		t.Fatalf("create payment checkout without operation config: %v", err)
	}
	if checkout.CheckoutURL == "" || checkout.ChannelCode != "usecase-creem" {
		t.Fatalf("unexpected checkout fallback result: %#v", checkout)
	}
}

func TestReceivePaymentWebhookPersistsReceiptAndQueuesOnce(t *testing.T) {
	manager := setupUsecaseOrderTxDB(t)
	appDB, _ := seedPayableOrder(t, manager)
	seedPaymentIntegrationConfig(t, appDB, "payment.creem.webhook-test")
	if err := usecase.RegisterPaymentAdapter("payment.creem.webhook-test", fakePaymentAdapter{t: t}); err != nil {
		t.Fatalf("register payment adapter: %v", err)
	}
	configurePaymentTestQueues(t)

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	receipt, err := usecase.ReceivePaymentWebhook(ctx, usecase.ReceivePaymentWebhookCmd{
		ChannelCode: "usecase-creem",
		Headers:     map[string]string{"creem-signature": "signature"},
		RawPayload:  []byte(`{"id":"evt_usecase"}`),
	})
	if err != nil {
		t.Fatalf("receive payment webhook: %v", err)
	}
	if receipt.ID == "" || receipt.Duplicate {
		t.Fatalf("unexpected first receipt: %#v", receipt)
	}
	if queueCount := countRows(t, appDB, `SELECT COUNT(*) FROM goqite WHERE queue = ?`, queue.QueueIntegrationWebhooks); queueCount != 1 {
		t.Fatalf("expected one webhook queue message, got %d", queueCount)
	}

	duplicate, err := usecase.ReceivePaymentWebhook(ctx, usecase.ReceivePaymentWebhookCmd{
		ChannelCode: "usecase-creem",
		Headers:     map[string]string{"creem-signature": "signature"},
		RawPayload:  []byte(`{"id":"evt_usecase"}`),
	})
	if err != nil {
		t.Fatalf("receive duplicate webhook: %v", err)
	}
	if !duplicate.Duplicate || duplicate.ID != receipt.ID {
		t.Fatalf("expected duplicate receipt, got %#v", duplicate)
	}
	if queueCount := countRows(t, appDB, `SELECT COUNT(*) FROM goqite WHERE queue = ?`, queue.QueueIntegrationWebhooks); queueCount != 1 {
		t.Fatalf("expected duplicate webhook not to enqueue again, got %d messages", queueCount)
	}

	var row struct {
		Status            string `db:"status"`
		PayloadCiphertext string `db:"payload_ciphertext"`
		MessageID         string `db:"message_id"`
	}
	if err := appDB.Get(&row, `SELECT status, payload_ciphertext, message_id FROM integration_webhook_receipts WHERE id = ?`, receipt.ID); err != nil {
		t.Fatalf("load receipt: %v", err)
	}
	if row.Status != models.IntegrationWebhookReceiptStatusQueued || row.MessageID == "" {
		t.Fatalf("unexpected receipt row: %#v", row)
	}
	if row.PayloadCiphertext == `{"id":"evt_usecase"}` {
		t.Fatal("expected encrypted webhook payload, got plaintext")
	}
}

func TestReceivePaymentWebhookRecordsHeaderHashForInvalidSignature(t *testing.T) {
	manager := setupUsecaseOrderTxDB(t)
	appDB, _ := seedPayableOrder(t, manager)
	seedPaymentIntegrationConfig(t, appDB, "payment.creem.invalid-signature-test")
	if err := usecase.RegisterPaymentAdapter("payment.creem.invalid-signature-test", authFailingPaymentAdapter{}); err != nil {
		t.Fatalf("register payment adapter: %v", err)
	}
	configurePaymentTestQueues(t)

	payload := []byte(`{"id":"evt_invalid"}`)
	signature := "bad-signature"
	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	_, err := usecase.ReceivePaymentWebhook(ctx, usecase.ReceivePaymentWebhookCmd{
		ChannelCode: "usecase-creem",
		Headers:     map[string]string{"creem-signature": signature},
		RawPayload:  payload,
	})
	if fwusecase.CodeOf(err) != fwusecase.CodeUnauthorized {
		t.Fatalf("expected unauthorized error, got %v", err)
	}

	var row struct {
		Status            string `db:"status"`
		PayloadCiphertext string `db:"payload_ciphertext"`
		HeadersHash       string `db:"headers_hash"`
		LastErrorCode     string `db:"last_error_code"`
	}
	if err := appDB.Get(&row, `
		SELECT status, payload_ciphertext, headers_hash, last_error_code
		FROM integration_webhook_receipts
		WHERE idempotency_key = ?
	`, "invalid:"+sha256HexForTest(payload)); err != nil {
		t.Fatalf("load failed receipt: %v", err)
	}
	if row.Status != models.IntegrationWebhookReceiptStatusFailed {
		t.Fatalf("expected failed receipt, got %#v", row)
	}
	if row.PayloadCiphertext == string(payload) {
		t.Fatal("expected encrypted webhook payload, got plaintext")
	}
	if row.HeadersHash != webhookHeadersHashForTest(map[string]string{"creem-signature": signature}) {
		t.Fatalf("unexpected headers hash: %#v", row)
	}
}

func TestReceivePaymentWebhookRequiresCreemSignatureAndDoesNotEnqueue(t *testing.T) {
	manager := setupUsecaseOrderTxDB(t)
	appDB, _ := seedPayableOrder(t, manager)
	seedPaymentIntegrationConfig(t, appDB, "payment.creem.missing-signature-test")
	if err := usecase.RegisterPaymentAdapter("payment.creem.missing-signature-test", creemadapter.NewAdapter(nil)); err != nil {
		t.Fatalf("register payment adapter: %v", err)
	}
	configurePaymentTestQueues(t)

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	_, err := usecase.ReceivePaymentWebhook(ctx, usecase.ReceivePaymentWebhookCmd{
		ChannelCode: "usecase-creem",
		RawPayload:  []byte(`{"id":"evt_missing_signature"}`),
	})
	if fwusecase.CodeOf(err) != fwusecase.CodeUnauthorized {
		t.Fatalf("expected unauthorized error, got %v", err)
	}

	if queueCount := countRows(t, appDB, `SELECT COUNT(*) FROM goqite WHERE queue = ?`, queue.QueueIntegrationWebhooks); queueCount != 0 {
		t.Fatalf("expected missing signature not to enqueue webhook work, got %d messages", queueCount)
	}
	if failedCount := countRows(t, appDB, `
		SELECT COUNT(*)
		FROM integration_webhook_receipts
		WHERE idempotency_key = ? AND status = ?
	`, "invalid:"+sha256HexForTest([]byte(`{"id":"evt_missing_signature"}`)), models.IntegrationWebhookReceiptStatusFailed); failedCount != 1 {
		t.Fatalf("expected one failed receipt for missing signature, got %d", failedCount)
	}
}

func TestReceivePaymentWebhookRejectsDisabledWebhookChannel(t *testing.T) {
	manager := setupUsecaseOrderTxDB(t)
	appDB, _ := seedPayableOrder(t, manager)
	seedPaymentIntegrationConfig(t, appDB, "payment.creem.webhook-disabled-test", 0)
	if err := usecase.RegisterPaymentAdapter("payment.creem.webhook-disabled-test", fakePaymentAdapter{t: t}); err != nil {
		t.Fatalf("register payment adapter: %v", err)
	}
	configurePaymentTestQueues(t)

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	_, err := usecase.ReceivePaymentWebhook(ctx, usecase.ReceivePaymentWebhookCmd{
		ChannelCode: "usecase-creem",
		Headers:     map[string]string{"creem-signature": "signature"},
		RawPayload:  []byte(`{"id":"evt_usecase"}`),
	})
	if fwusecase.CodeOf(err) != fwusecase.CodeForbidden {
		t.Fatalf("expected forbidden error, got %v", err)
	}
	if queueCount := countRows(t, appDB, `SELECT COUNT(*) FROM goqite WHERE queue = ?`, queue.QueueIntegrationWebhooks); queueCount != 0 {
		t.Fatalf("expected no webhook queue message, got %d", queueCount)
	}
}

func sha256HexForTest(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func webhookHeaderForTest(headers map[string]string, name string) string {
	for key, value := range headers {
		if strings.EqualFold(key, name) {
			return value
		}
	}
	return ""
}

func webhookHeadersHashForTest(headers map[string]string) string {
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
	return sha256HexForTest([]byte(builder.String()))
}

func TestHandlePaymentWebhookJobCompletesOrderThroughWorker(t *testing.T) {
	manager := setupUsecaseOrderTxDB(t)
	appDB, orderID := seedPayableOrder(t, manager)
	seedPaymentIntegrationConfig(t, appDB, "payment.creem.worker-test")
	if err := usecase.RegisterPaymentAdapter("payment.creem.worker-test", fakePaymentAdapter{t: t}); err != nil {
		t.Fatalf("register payment adapter: %v", err)
	}
	configurePaymentTestQueues(t)

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	receipt, err := usecase.ReceivePaymentWebhook(ctx, usecase.ReceivePaymentWebhookCmd{
		ChannelCode: "usecase-creem",
		Headers:     map[string]string{"creem-signature": "signature"},
		RawPayload:  []byte(`{"id":"evt_usecase"}`),
	})
	if err != nil {
		t.Fatalf("receive payment webhook: %v", err)
	}

	body, _ := json.Marshal(usecase.PaymentWebhookJobPayload{ReceiptID: receipt.ID})
	if err := usecase.HandlePaymentWebhookJob(t.Context(), body); err != nil {
		t.Fatalf("handle webhook job: %v", err)
	}

	var orderStatus string
	if err := appDB.Get(&orderStatus, `SELECT status FROM orders WHERE id = ?`, orderID); err != nil {
		t.Fatalf("get order status: %v", err)
	}
	if orderStatus != "paid" {
		t.Fatalf("expected worker to mark order paid, got %q", orderStatus)
	}
	var receiptStatus string
	if err := appDB.Get(&receiptStatus, `SELECT status FROM integration_webhook_receipts WHERE id = ?`, receipt.ID); err != nil {
		t.Fatalf("get receipt status: %v", err)
	}
	if receiptStatus != models.IntegrationWebhookReceiptStatusSucceeded {
		t.Fatalf("expected succeeded receipt, got %q", receiptStatus)
	}
	if eventCount := countRows(t, appDB, `SELECT COUNT(*) FROM domain_events WHERE topic = ?`, usecaseevents.OrderPaidTopic); eventCount != 1 {
		t.Fatalf("expected order paid domain event, got %d", eventCount)
	}
	var refs struct {
		ProviderCheckoutID     string `db:"provider_checkout_id"`
		ProviderOrderID        string `db:"provider_order_id"`
		ProviderCustomerID     string `db:"provider_customer_id"`
		ProviderSubscriptionID string `db:"provider_subscription_id"`
		ProviderProductID      string `db:"provider_product_id"`
		SubscriptionStatus     string `db:"subscription_status"`
	}
	if err := appDB.Get(&refs, `
		SELECT provider_checkout_id, provider_order_id, provider_customer_id, provider_subscription_id, provider_product_id, subscription_status
		FROM orders
		WHERE id = ?
	`, orderID); err != nil {
		t.Fatalf("get provider refs: %v", err)
	}
	if refs.ProviderCheckoutID != "ch_usecase" || refs.ProviderOrderID != "ord_usecase" || refs.ProviderCustomerID != "cust_usecase" || refs.ProviderSubscriptionID != "sub_usecase" || refs.ProviderProductID != "prod_usecase" || refs.SubscriptionStatus != usecase.OrderSubscriptionStatusActive {
		t.Fatalf("unexpected provider refs: %#v", refs)
	}
}

func TestHandlePaymentWebhookJobCancelsSubscriptionWithoutChangingMembership(t *testing.T) {
	manager := setupUsecaseOrderTxDB(t)
	appDB, orderID := seedPayableOrder(t, manager)
	if _, err := appDB.Exec(`
		UPDATE orders
		SET status = 'paid', provider_subscription_id = 'sub_usecase', subscription_status = 'active', membership_applied_at = '2026-01-01 00:00:00'
		WHERE id = ?
	`, orderID); err != nil {
		t.Fatalf("prepare paid subscription order: %v", err)
	}
	if _, err := appDB.Exec(`UPDATE users SET membership_level = 'premium', membership_expires_at = '2099-02-28 00:00:00' WHERE id = 'u1'`); err != nil {
		t.Fatalf("prepare membership: %v", err)
	}
	seedPaymentIntegrationConfig(t, appDB, "payment.creem.subscription-cancel-test")
	if err := usecase.RegisterPaymentAdapter("payment.creem.subscription-cancel-test", subscriptionCanceledPaymentAdapter{t: t}); err != nil {
		t.Fatalf("register payment adapter: %v", err)
	}
	configurePaymentTestQueues(t)

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	receipt, err := usecase.ReceivePaymentWebhook(ctx, usecase.ReceivePaymentWebhookCmd{
		ChannelCode: "usecase-creem",
		Headers:     map[string]string{"creem-signature": "signature"},
		RawPayload:  []byte(`{"id":"evt_cancel"}`),
	})
	if err != nil {
		t.Fatalf("receive payment webhook: %v", err)
	}

	body, _ := json.Marshal(usecase.PaymentWebhookJobPayload{ReceiptID: receipt.ID})
	if err := usecase.HandlePaymentWebhookJob(t.Context(), body); err != nil {
		t.Fatalf("handle webhook job: %v", err)
	}

	var order struct {
		Status             string `db:"status"`
		SubscriptionStatus string `db:"subscription_status"`
	}
	if err := appDB.Get(&order, `SELECT status, subscription_status FROM orders WHERE id = ?`, orderID); err != nil {
		t.Fatalf("load order: %v", err)
	}
	if order.Status != "paid" || order.SubscriptionStatus != usecase.OrderSubscriptionStatusCanceled {
		t.Fatalf("unexpected order state after cancel webhook: %#v", order)
	}
	var user struct {
		MembershipLevel     string `db:"membership_level"`
		MembershipExpiresAt string `db:"membership_expires_at"`
	}
	if err := appDB.Get(&user, `SELECT membership_level, membership_expires_at FROM users WHERE id = 'u1'`); err != nil {
		t.Fatalf("load user: %v", err)
	}
	if user.MembershipLevel != "premium" || !sameSQLiteInstantForPaymentTest(user.MembershipExpiresAt, "2099-02-28 00:00:00") {
		t.Fatalf("membership should not change after cancellation: %#v", user)
	}
}

func sameSQLiteInstantForPaymentTest(actual string, expected string) bool {
	if actual == expected {
		return true
	}
	return actual == expected[:10]+"T"+expected[11:]+"Z"
}

func seedPaymentIntegrationConfig(t *testing.T, appDB *sqlx.DB, adapterKey string, webhookEnabledOverride ...int) {
	t.Helper()
	webhookEnabled := 1
	if len(webhookEnabledOverride) > 0 {
		webhookEnabled = webhookEnabledOverride[0]
	}

	credentialValue, err := credentialsForTest(`{"api_key":"payment-api-key","webhook_secret":"payment-webhook-secret"}`)
	if err != nil {
		t.Fatalf("prepare payment credential: %v", err)
	}
	if _, err := appDB.Exec(`
		INSERT INTO integration_credentials (id, credential_type, ciphertext, key_version, masked_value, value_text, enabled)
		VALUES (?, 'payment_bundle', ?, '', '', ?, 1)
	`, "payment-credential", credentialValue, credentialValue); err != nil {
		t.Fatalf("insert payment credential: %v", err)
	}
	if _, err := appDB.Exec(`
		INSERT INTO integration_channels (
			id, scenario, channel_code, provider_code, adapter_key, environment, enabled, priority, credential_id, webhook_enabled, config_json
		) VALUES (?, 'payment', 'usecase-creem', 'creem', ?, 'test', 1, 1, ?, ?, '{"base_url":"https://test-api.creem.io/v1","product_id":"prod_usecase","success_url":"https://example.com/orders"}')
	`, "payment-channel", adapterKey, "payment-credential", webhookEnabled); err != nil {
		t.Fatalf("insert payment channel: %v", err)
	}
	if _, err := appDB.Exec(`
		INSERT INTO integration_operation_configs (
			id, scenario, operation, channel_code, enabled
		) VALUES (?, 'payment', 'create_payment', 'usecase-creem', 1)
	`, "payment-create-config"); err != nil {
		t.Fatalf("insert payment operation config: %v", err)
	}
}

func configurePaymentTestQueues(t *testing.T) {
	t.Helper()

	queueManager, err := queue.NewManager()
	if err != nil {
		t.Fatalf("new queue manager: %v", err)
	}
	previousQueue := usecase.DefaultQueueManager
	usecase.DefaultQueueManager = queueManager
	fwevents.Configure(usecaseevents.DurableStore{}, queueManager)
	t.Cleanup(func() {
		usecase.DefaultQueueManager = previousQueue
	})

	if err := usecaseevents.RegisterEventHandlers(func(ctx fwusecase.Context, cmd usecaseevents.AwardOrderPaidPointsCmd) (usecaseevents.PointsResult, bool, error) {
		points, awarded, err := usecase.AwardOrderPaidPoints(ctx, usecase.AwardOrderPaidPointsCmd{
			UserID:  cmd.UserID,
			OrderID: cmd.OrderID,
			Points:  cmd.Points,
		})
		return usecaseevents.PointsResult{
			UserID:  points.UserID,
			Balance: points.Balance,
		}, awarded, err
	}, sendRealtimeNotificationForUsecaseTest); err != nil {
		t.Fatalf("register event handlers: %v", err)
	}
	if err := usecaseevents.RegisterMembershipEventHandlers(func(ctx fwusecase.Context, cmd usecaseevents.ApplyOrderMembershipCmd) (usecaseevents.MembershipResult, bool, error) {
		membership, applied, err := usecase.ApplyOrderMembership(ctx, usecase.ApplyOrderMembershipCmd{
			OrderID: cmd.OrderID,
		})
		return usecaseevents.MembershipResult{
			UserID:              membership.UserID,
			MembershipLevel:     membership.MembershipLevel,
			MembershipExpiresAt: membership.MembershipExpiresAt,
		}, applied, err
	}); err != nil {
		t.Fatalf("register membership event handlers: %v", err)
	}
}

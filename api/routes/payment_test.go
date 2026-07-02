package routes_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/tfnick/go-svelte-starter/api/db"
	fwcontext "github.com/tfnick/go-svelte-starter/api/framework/http/context"
	"github.com/tfnick/go-svelte-starter/api/framework/queue"
	"github.com/tfnick/go-svelte-starter/api/models"
	"github.com/tfnick/go-svelte-starter/api/routes"
	"github.com/tfnick/go-svelte-starter/api/usecase"
	"github.com/tfnick/go-svelte-starter/api/usecase/integrations/payment"
	"github.com/tfnick/sqlx"
)

type routeFakePaymentAdapter struct{}

func (routeFakePaymentAdapter) CreatePayment(ctx context.Context, cfg payment.ProviderConfig, req payment.CreatePaymentRequest) (payment.CreatePaymentResult, error) {
	if req.ProductID != "prod_route" {
		panic("unexpected route payment product id: " + req.ProductID)
	}
	return payment.CreatePaymentResult{
		ProviderPaymentID: "route-ch",
		CheckoutURL:       "https://checkout.creem.io/route-ch",
		Status:            "pending",
		ProviderRequestID: "route-ch",
	}, nil
}

func (routeFakePaymentAdapter) NormalizePaymentWebhook(ctx context.Context, cfg payment.ProviderConfig, req payment.WebhookRequest) (payment.NormalizedWebhook, error) {
	if webhookHeaderForRouteTest(req.Headers, "creem-signature") != "signature" {
		panic("expected route to forward webhook headers")
	}
	return payment.NormalizedWebhook{
		ProviderEventID:   "route-evt",
		EventType:         "checkout.completed",
		BusinessEventType: payment.WebhookEventPaymentSucceeded,
		ProviderPaymentID: "route-ch",
		PaymentStatus:     "succeeded",
		OrderID:           "route-order",
		SafeSnapshot: map[string]interface{}{
			"event_type":          "checkout.completed",
			"provider_event_id":   "route-evt",
			"provider_payment_id": "route-ch",
			"order_id":            "route-order",
		},
	}, nil
}

func webhookHeaderForRouteTest(headers map[string]string, name string) string {
	for key, value := range headers {
		if strings.EqualFold(key, name) {
			return value
		}
	}
	return ""
}

func (routeFakePaymentAdapter) CancelSubscription(ctx context.Context, cfg payment.ProviderConfig, req payment.CancelSubscriptionRequest) (payment.CancelSubscriptionResult, error) {
	return payment.CancelSubscriptionResult{Status: "canceled"}, nil
}

func TestCreateOrderPaymentCheckoutUsesInternalEnvelope(t *testing.T) {
	setupRouteTestDBs(t)
	appDB, err := db.DefaultManager.GetEngine("app")
	if err != nil {
		t.Fatalf("get app engine: %v", err)
	}
	seedRoutePaymentOrder(t, appDB)
	seedRoutePaymentIntegrationConfig(t, appDB, "payment.creem.route-checkout-test")
	if err := usecase.RegisterPaymentAdapter("payment.creem.route-checkout-test", routeFakePaymentAdapter{}); err != nil {
		t.Fatalf("register payment adapter: %v", err)
	}

	router := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/orders/route-order/payment-checkout", nil)
	rec := httptest.NewRecorder()
	c := router.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("route-order")
	fwcontext.SetCurrentUser(c, &models.User{ID: "route-user", Name: "Ada", IsAdmin: 0})

	if err := routes.CreateOrderPaymentCheckout(c); err != nil {
		t.Fatalf("create checkout route: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var envelope struct {
		Success bool                           `json:"success"`
		Data    routes.PaymentCheckoutResponse `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !envelope.Success || envelope.Data.CheckoutURL == "" || envelope.Data.Order.Status != "pending" {
		t.Fatalf("unexpected envelope: %#v body=%s", envelope, rec.Body.String())
	}
}

func TestCreateOrderPaymentCheckoutRejectsCrossUserRoute(t *testing.T) {
	setupRouteTestDBs(t)
	appDB, err := db.DefaultManager.GetEngine("app")
	if err != nil {
		t.Fatalf("get app engine: %v", err)
	}
	seedRoutePaymentOrder(t, appDB)
	seedRoutePaymentIntegrationConfig(t, appDB, "payment.creem.route-cross-user-test")

	router := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/orders/route-order/payment-checkout", nil)
	rec := httptest.NewRecorder()
	c := router.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("route-order")
	fwcontext.SetCurrentUser(c, &models.User{ID: "other-user", Name: "Mallory", IsAdmin: 0})

	if err := routes.CreateOrderPaymentCheckout(c); err != nil {
		t.Fatalf("create checkout route: %v", err)
	}
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d: %s", http.StatusForbidden, rec.Code, rec.Body.String())
	}
}

func TestReceivePaymentWebhookReturnsProviderAckAndQueuesReceipt(t *testing.T) {
	setupRouteTestDBs(t)
	appDB, err := db.DefaultManager.GetEngine("app")
	if err != nil {
		t.Fatalf("get app engine: %v", err)
	}
	seedRoutePaymentOrder(t, appDB)
	seedRoutePaymentIntegrationConfig(t, appDB, "payment.creem.route-webhook-test")
	if err := usecase.RegisterPaymentAdapter("payment.creem.route-webhook-test", routeFakePaymentAdapter{}); err != nil {
		t.Fatalf("register payment adapter: %v", err)
	}
	configureRoutePaymentQueue(t)

	router := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/integrations/payment/route-creem/webhooks/creem", strings.NewReader(`{"id":"route-evt"}`))
	req.Header.Set("creem-signature", "signature")
	rec := httptest.NewRecorder()
	c := router.NewContext(req, rec)
	c.SetParamNames("channel_code")
	c.SetParamValues("route-creem")

	if err := routes.ReceivePaymentWebhook(c); err != nil {
		t.Fatalf("receive webhook route: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}
	if rec.Body.String() != "" {
		t.Fatalf("expected empty provider ack body, got %s", rec.Body.String())
	}
	if queueCount := routeCountRows(t, appDB, `SELECT COUNT(*) FROM goqite WHERE queue = ?`, queue.QueueIntegrationWebhooks); queueCount != 1 {
		t.Fatalf("expected webhook queue message, got %d", queueCount)
	}
}

func seedRoutePaymentOrder(t *testing.T, appDB *sqlx.Engine) {
	t.Helper()

	if _, err := appDB.ExecP(`INSERT INTO users (id, name, email, password_hash, email_verified, is_active) VALUES ('route-user', 'Ada', 'ada@example.com', '', 1, 1)`); err != nil {
		t.Fatalf("insert route user: %v", err)
	}
	seedRouteCheckoutProduct(t, appDB, "route-product", "prod_route")
	if _, err := appDB.ExecP(`INSERT INTO orders (id, user_id, product_id, amount, status) VALUES ('route-order', 'route-user', 'route-product', 1000, 'pending')`); err != nil {
		t.Fatalf("insert route order: %v", err)
	}
}

func seedRoutePaymentIntegrationConfig(t *testing.T, appDB *sqlx.Engine, adapterKey string) {
	t.Helper()

	credentialValue := `{"api_key":"route-payment-api-key","webhook_secret":"route-payment-webhook-secret"}`
	if _, err := appDB.ExecP(`
		INSERT INTO integration_credentials (id, credential_type, ciphertext, key_version, masked_value, value_text, enabled)
		VALUES (?, 'payment_bundle', ?, '', '', ?, 1)
	`, "route-payment-credential", credentialValue, credentialValue); err != nil {
		t.Fatalf("insert payment credential: %v", err)
	}
	if _, err := appDB.ExecP(`
		INSERT INTO integration_channels (
			id, scenario, channel_code, provider_code, adapter_key, environment, enabled, priority, credential_id, webhook_enabled, config_json
		) VALUES (?, 'payment', 'route-creem', 'creem', ?, 'test', 1, 1, ?, 1, '{"base_url":"https://test-api.creem.io/v1","product_id":"prod_route"}')
	`, "route-payment-channel", adapterKey, "route-payment-credential"); err != nil {
		t.Fatalf("insert payment channel: %v", err)
	}
	if _, err := appDB.ExecP(`
		INSERT INTO integration_operation_configs (
			id, scenario, operation, channel_code, enabled
		) VALUES (?, 'payment', 'create_payment', 'route-creem', 1)
	`, "route-payment-create-config"); err != nil {
		t.Fatalf("insert payment operation config: %v", err)
	}
}

func configureRoutePaymentQueue(t *testing.T) {
	t.Helper()

	queueManager, err := queue.NewManager()
	if err != nil {
		t.Fatalf("new queue manager: %v", err)
	}
	previousQueue := usecase.DefaultQueueManager
	usecase.DefaultQueueManager = queueManager
	t.Cleanup(func() {
		usecase.DefaultQueueManager = previousQueue
	})
}

func routeCountRows(t *testing.T, appDB *sqlx.Engine, query string, args ...interface{}) int {
	t.Helper()

	var count int
	if err := appDB.GetP(&count, query, args...); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	return count
}

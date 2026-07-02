package payment

import "context"

const (
	Scenario               = "payment"
	OperationCreatePayment = "create_payment"

	WebhookEventPaymentSucceeded     = "payment.succeeded"
	WebhookEventSubscriptionCanceled = "subscription.canceled"
	WebhookEventSubscriptionRenewed  = "subscription.renewed"

	CancelSubscriptionModeImmediate   = "immediate"
	CancelSubscriptionModeScheduled   = "scheduled"
	CancelSubscriptionOnExecuteCancel = "cancel"
	CancelSubscriptionOnExecutePause  = "pause"
)

type ProviderConfig struct {
	ChannelCode   string
	ProviderCode  string
	AdapterKey    string
	BaseURL       string
	APIKey        string
	WebhookSecret string
	ProductID     string
	SuccessURL    string
	Units         int
}

type Customer struct {
	Email string
}

type CreatePaymentRequest struct {
	OrderID    string
	UserID     string
	Amount     int64
	Currency   string
	Customer   Customer
	Metadata   map[string]string
	ProductID  string
	SuccessURL string
	Units      int
}

type CreatePaymentResult struct {
	ProviderPaymentID string
	CheckoutURL       string
	Status            string
	ProviderRequestID string
}

type WebhookRequest struct {
	RawPayload      []byte
	Headers         map[string]string
	VerifySignature bool
}

type CancelSubscriptionRequest struct {
	SubscriptionID string
	Mode           string
	OnExecute      string
}

type CancelSubscriptionResult struct {
	Status string
}

type NormalizedWebhook struct {
	ProviderEventID        string
	EventType              string
	BusinessEventType      string
	ProviderPaymentID      string
	PaymentStatus          string
	OrderID                string
	ProviderOrderID        string
	ProviderCustomerID     string
	ProviderSubscriptionID string
	ProviderProductID      string
	SubscriptionPeriodEnd  string
	SafeSnapshot           map[string]interface{}
}

type Adapter interface {
	CreatePayment(ctx context.Context, cfg ProviderConfig, req CreatePaymentRequest) (CreatePaymentResult, error)
	NormalizePaymentWebhook(ctx context.Context, cfg ProviderConfig, req WebhookRequest) (NormalizedWebhook, error)
	CancelSubscription(ctx context.Context, cfg ProviderConfig, req CancelSubscriptionRequest) (CancelSubscriptionResult, error)
}

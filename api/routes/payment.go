package routes

import (
	"io"
	"net/http"

	"github.com/labstack/echo/v4"
	fwcontext "github.com/tfnick/go-svelte-starter/api/framework/http/context"
	httpresponse "github.com/tfnick/go-svelte-starter/api/framework/http/response"
	"github.com/tfnick/go-svelte-starter/api/usecase"
)

const maxPaymentWebhookPayloadBytes = 1 << 20

type PaymentCheckoutResponse struct {
	Message           string        `json:"message"`
	Order             OrderResponse `json:"order"`
	CheckoutURL       string        `json:"checkout_url"`
	ProviderPaymentID string        `json:"provider_payment_id"`
	ChannelCode       string        `json:"channel_code"`
	InvocationID      string        `json:"invocation_id"`
	Status            string        `json:"status"`
}

func ToPaymentCheckoutResponse(checkout usecase.PaymentCheckoutCo) PaymentCheckoutResponse {
	return PaymentCheckoutResponse{
		Message:           "payment checkout created",
		Order:             ToOrderResponse(checkout.Order),
		CheckoutURL:       checkout.CheckoutURL,
		ProviderPaymentID: checkout.ProviderPaymentID,
		ChannelCode:       checkout.ChannelCode,
		InvocationID:      checkout.InvocationID,
		Status:            checkout.Status,
	}
}

func CreateOrderPaymentCheckout(c echo.Context) error {
	orderID := c.Param("id")
	ctx := fwcontext.InternalUsecaseContext(c)
	checkout, err := usecase.CreateOrderPaymentCheckout(ctx, usecase.CreateOrderPaymentCheckoutCmd{
		OrderID: orderID,
	})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}
	return httpresponse.OK(c, ToPaymentCheckoutResponse(checkout))
}

func ReceivePaymentWebhook(c echo.Context) error {
	body, err := io.ReadAll(io.LimitReader(c.Request().Body, maxPaymentWebhookPayloadBytes+1))
	if err != nil {
		return httpresponse.BadRequest(c, "invalid request data")
	}
	if len(body) > maxPaymentWebhookPayloadBytes {
		return httpresponse.ErrorWithCode(c, http.StatusRequestEntityTooLarge, "validation", "payment webhook payload is too large")
	}

	ctx := fwcontext.InternalUsecaseContext(c)
	_, err = usecase.ReceivePaymentWebhook(ctx, usecase.ReceivePaymentWebhookCmd{
		ChannelCode: c.Param("channel_code"),
		Headers:     webhookHeaders(c.Request().Header),
		RawPayload:  body,
	})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}
	return c.NoContent(http.StatusOK)
}

func webhookHeaders(headers http.Header) map[string]string {
	result := make(map[string]string, len(headers))
	for key, values := range headers {
		if len(values) == 0 {
			continue
		}
		result[key] = values[0]
	}
	return result
}

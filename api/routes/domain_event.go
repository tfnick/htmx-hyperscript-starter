package routes

import (
	"github.com/labstack/echo/v4"
	fwcontext "github.com/tfnick/go-svelte-starter/api/framework/http/context"
	fwrequest "github.com/tfnick/go-svelte-starter/api/framework/http/request"
	httpresponse "github.com/tfnick/go-svelte-starter/api/framework/http/response"
	"github.com/tfnick/go-svelte-starter/api/usecase"
)

type DomainEventResponse struct {
	ID            string `json:"id"`
	Topic         string `json:"topic"`
	AggregateType string `json:"aggregate_type"`
	AggregateID   string `json:"aggregate_id"`
	PayloadJSON   string `json:"payload_json"`
	MetadataJSON  string `json:"metadata_json"`
	OccurredAt    string `json:"occurred_at"`
	CreatedAt     string `json:"created_at"`
}

type DomainEventDeliveryResponse struct {
	ID         string `json:"id"`
	EventID    string `json:"event_id"`
	Subscriber string `json:"subscriber"`
	MessageID  string `json:"message_id"`
	Status     string `json:"status"`
	Attempts   int    `json:"attempts"`
	LastError  string `json:"last_error"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

type DomainEventsResponse struct {
	Items      []DomainEventResponse `json:"items"`
	Pagination PaginationResponse    `json:"pagination"`
}

func ListDomainEvents(c echo.Context) error {
	page := fwrequest.PageQuery(c)
	ctx := fwcontext.InternalUsecaseContext(c)
	events, err := usecase.ListDomainEvents(ctx, usecase.DomainEventsQry{
		Page:     page.Page,
		PageSize: page.PageSize,
	})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}

	return httpresponse.OK(c, ToDomainEventsResponse(events))
}

func ListDomainEventDeliveries(c echo.Context) error {
	ctx := fwcontext.InternalUsecaseContext(c)
	deliveries, err := usecase.ListDomainEventDeliveries(ctx, usecase.DomainEventDeliveriesQry{
		EventID: c.Param("id"),
	})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}

	return httpresponse.OK(c, ToDomainEventDeliveryResponses(deliveries))
}

func ToDomainEventResponse(event usecase.DomainEventCo) DomainEventResponse {
	return DomainEventResponse{
		ID:            event.ID,
		Topic:         event.Topic,
		AggregateType: event.AggregateType,
		AggregateID:   event.AggregateID,
		PayloadJSON:   event.PayloadJSON,
		MetadataJSON:  event.MetadataJSON,
		OccurredAt:    event.OccurredAt,
		CreatedAt:     event.CreatedAt,
	}
}

func ToDomainEventResponses(events []usecase.DomainEventCo) []DomainEventResponse {
	responses := make([]DomainEventResponse, 0, len(events))
	for i := range events {
		responses = append(responses, ToDomainEventResponse(events[i]))
	}
	return responses
}

func ToDomainEventDeliveryResponse(delivery usecase.DomainEventDeliveryCo) DomainEventDeliveryResponse {
	return DomainEventDeliveryResponse{
		ID:         delivery.ID,
		EventID:    delivery.EventID,
		Subscriber: delivery.Subscriber,
		MessageID:  delivery.MessageID,
		Status:     delivery.Status,
		Attempts:   delivery.Attempts,
		LastError:  delivery.LastError,
		CreatedAt:  delivery.CreatedAt,
		UpdatedAt:  delivery.UpdatedAt,
	}
}

func ToDomainEventDeliveryResponses(deliveries []usecase.DomainEventDeliveryCo) []DomainEventDeliveryResponse {
	responses := make([]DomainEventDeliveryResponse, 0, len(deliveries))
	for i := range deliveries {
		responses = append(responses, ToDomainEventDeliveryResponse(deliveries[i]))
	}
	return responses
}

func ToDomainEventsResponse(events usecase.DomainEventsCo) DomainEventsResponse {
	return DomainEventsResponse{
		Items:      ToDomainEventResponses(events.Items),
		Pagination: ToPaginationResponse(events.Pagination),
	}
}

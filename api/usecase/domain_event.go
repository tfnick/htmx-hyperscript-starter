package usecase

import (
	"strings"

	fwusecase "github.com/tfnick/go-svelte-starter/api/framework/usecase"
	"github.com/tfnick/go-svelte-starter/api/models"
)

type DomainEventsQry struct {
	Page     int
	PageSize int
}

type DomainEventDeliveriesQry struct {
	EventID string
}

type DomainEventCo struct {
	ID            string
	Topic         string
	AggregateType string
	AggregateID   string
	PayloadJSON   string
	MetadataJSON  string
	OccurredAt    string
	CreatedAt     string
}

type DomainEventDeliveryCo struct {
	ID         string
	EventID    string
	Subscriber string
	MessageID  string
	Status     string
	Attempts   int
	LastError  string
	CreatedAt  string
	UpdatedAt  string
}

type DomainEventsCo struct {
	Items      []DomainEventCo
	Pagination fwusecase.PageResult
}

func ListDomainEvents(ctx fwusecase.Context, qry DomainEventsQry) (DomainEventsCo, error) {
	pageQuery, err := fwusecase.NormalizePageQuery(fwusecase.PageQuery{
		Page:     qry.Page,
		PageSize: qry.PageSize,
	})
	if err != nil {
		return DomainEventsCo{}, err
	}

	totalItems, err := models.CountDomainEvents(ctx.Std())
	if err != nil {
		return DomainEventsCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to count domain events", err)
	}

	events, err := models.ListDomainEvents(ctx.Std(), pageQuery.Limit(), pageQuery.Offset())
	if err != nil {
		return DomainEventsCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to load domain events", err)
	}

	return DomainEventsCo{
		Items:      domainEventCosFromModels(events),
		Pagination: fwusecase.NewPageResult(pageQuery, totalItems),
	}, nil
}

func ListDomainEventDeliveries(ctx fwusecase.Context, qry DomainEventDeliveriesQry) ([]DomainEventDeliveryCo, error) {
	eventID := strings.TrimSpace(qry.EventID)
	if eventID == "" {
		return nil, fwusecase.E(fwusecase.CodeValidation, "event ID is required", nil)
	}

	deliveries, err := models.ListDomainEventDeliveries(ctx.Std(), eventID)
	if err != nil {
		return nil, fwusecase.E(fwusecase.CodeInternal, "failed to load domain event deliveries", err)
	}
	return domainEventDeliveryCosFromModels(deliveries), nil
}

func domainEventCoFromModel(event models.DomainEvent) DomainEventCo {
	return DomainEventCo{
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

func domainEventCosFromModels(events []models.DomainEvent) []DomainEventCo {
	result := make([]DomainEventCo, 0, len(events))
	for i := range events {
		result = append(result, domainEventCoFromModel(events[i]))
	}
	return result
}

func domainEventDeliveryCoFromModel(delivery models.DomainEventDelivery) DomainEventDeliveryCo {
	return DomainEventDeliveryCo{
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

func domainEventDeliveryCosFromModels(deliveries []models.DomainEventDelivery) []DomainEventDeliveryCo {
	result := make([]DomainEventDeliveryCo, 0, len(deliveries))
	for i := range deliveries {
		result = append(result, domainEventDeliveryCoFromModel(deliveries[i]))
	}
	return result
}

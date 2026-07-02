package usecase_test

import (
	"fmt"
	"testing"

	fwusecase "github.com/tfnick/go-svelte-starter/api/framework/usecase"
	"github.com/tfnick/go-svelte-starter/api/usecase"
	"github.com/tfnick/sqlx"
)

func TestListDomainEventsReturnsRequestedPageAndMetadata(t *testing.T) {
	manager := setupUsecaseOrderTxDB(t)
	appDB, err := manager.GetEngine("app")
	if err != nil {
		t.Fatalf("get app engine: %v", err)
	}
	seedDomainEventsForPagination(t, appDB, 5)

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	result, err := usecase.ListDomainEvents(ctx, usecase.DomainEventsQry{
		Page:     2,
		PageSize: 2,
	})
	if err != nil {
		t.Fatalf("list domain events: %v", err)
	}

	if len(result.Items) != 2 {
		t.Fatalf("expected two events on page 2, got %d", len(result.Items))
	}
	if result.Items[0].ID != "event-03" || result.Items[1].ID != "event-02" {
		t.Fatalf("expected stable created_at desc page order, got %#v", result.Items)
	}

	page := result.Pagination
	if page.Page != 2 || page.PageSize != 2 || page.TotalItems != 5 || page.TotalPages != 3 {
		t.Fatalf("unexpected pagination metadata: %#v", page)
	}
	if !page.HasPrevious || !page.HasNext {
		t.Fatalf("expected page 2 of 3 to have previous and next: %#v", page)
	}
}

func TestListDomainEventDeliveriesReturnsSelectedEventRecords(t *testing.T) {
	manager := setupUsecaseOrderTxDB(t)
	appDB, err := manager.GetEngine("app")
	if err != nil {
		t.Fatalf("get app engine: %v", err)
	}
	seedDomainEventsForPagination(t, appDB, 1)
	seedDomainEventDeliveries(t, appDB, "event-01")

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	deliveries, err := usecase.ListDomainEventDeliveries(ctx, usecase.DomainEventDeliveriesQry{
		EventID: "event-01",
	})
	if err != nil {
		t.Fatalf("list domain event deliveries: %v", err)
	}

	if len(deliveries) != 2 {
		t.Fatalf("expected two deliveries, got %#v", deliveries)
	}
	if deliveries[0].Subscriber != "audit.record_order_paid" || deliveries[1].Subscriber != "points.award_on_order_paid" {
		t.Fatalf("expected stable delivery order, got %#v", deliveries)
	}
	if deliveries[1].LastError != "boom" || deliveries[1].Attempts != 2 {
		t.Fatalf("expected failed delivery metadata, got %#v", deliveries[1])
	}
}

func TestListDomainEventDeliveriesRejectsEmptyEventID(t *testing.T) {
	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	_, err := usecase.ListDomainEventDeliveries(ctx, usecase.DomainEventDeliveriesQry{})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func seedDomainEventsForPagination(t *testing.T, appDB *sqlx.Engine, count int) {
	t.Helper()

	query := `
		INSERT INTO domain_events (
			id, topic, aggregate_type, aggregate_id, payload_json, metadata_json, occurred_at, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`
	for i := 1; i <= count; i++ {
		_, err := appDB.ExecP(query,
			fmt.Sprintf("event-%02d", i),
			"order.paid",
			"order",
			fmt.Sprintf("order-%02d", i),
			fmt.Sprintf(`{"order_id":"order-%02d"}`, i),
			"{}",
			fmt.Sprintf("2026-01-01T00:00:%02dZ", i),
			fmt.Sprintf("2026-01-01 00:00:%02d", i),
		)
		if err != nil {
			t.Fatalf("insert domain event %d: %v", i, err)
		}
	}
}

func seedDomainEventDeliveries(t *testing.T, appDB *sqlx.Engine, eventID string) {
	t.Helper()

	query := `
		INSERT INTO domain_event_deliveries (
			id, event_id, subscriber, message_id, status, attempts, last_error, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	rows := []struct {
		id         string
		subscriber string
		status     string
		attempts   int
		lastError  string
		createdAt  string
	}{
		{"delivery-01", "audit.record_order_paid", "succeeded", 1, "", "2026-01-01 00:00:01"},
		{"delivery-02", "points.award_on_order_paid", "failed", 2, "boom", "2026-01-01 00:00:02"},
	}
	for _, row := range rows {
		_, err := appDB.ExecP(query,
			row.id,
			eventID,
			row.subscriber,
			row.id+"-message",
			row.status,
			row.attempts,
			row.lastError,
			row.createdAt,
			row.createdAt,
		)
		if err != nil {
			t.Fatalf("insert domain event delivery %s: %v", row.id, err)
		}
	}
}

package routes_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/tfnick/go-svelte-starter/api/db"
	"github.com/tfnick/go-svelte-starter/api/routes"
	"github.com/tfnick/sqlx"
)

func TestListDomainEventsReturnsPaginatedEnvelope(t *testing.T) {
	setupRouteTestDBs(t)
	appDB, err := db.DefaultManager.GetEngine("app")
	if err != nil {
		t.Fatalf("get app engine: %v", err)
	}
	seedRouteDomainEventsForPagination(t, appDB, 5)

	router := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/events?page=2&page_size=2", nil)
	rec := httptest.NewRecorder()
	c := router.NewContext(req, rec)

	if err := routes.ListDomainEvents(c); err != nil {
		t.Fatalf("list domain events: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var envelope struct {
		Success bool                        `json:"success"`
		Data    routes.DomainEventsResponse `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !envelope.Success {
		t.Fatalf("expected success envelope, got %s", rec.Body.String())
	}
	if len(envelope.Data.Items) != 2 {
		t.Fatalf("expected two event items, got %#v", envelope.Data.Items)
	}
	if envelope.Data.Items[0].ID != "route-event-03" || envelope.Data.Items[1].ID != "route-event-02" {
		t.Fatalf("expected stable page items, got %#v", envelope.Data.Items)
	}
	if envelope.Data.Pagination.TotalItems != 5 || envelope.Data.Pagination.TotalPages != 3 {
		t.Fatalf("unexpected pagination metadata: %#v", envelope.Data.Pagination)
	}
}

func TestListDomainEventsRejectsInvalidPageQuery(t *testing.T) {
	setupRouteTestDBs(t)

	router := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/events?page=0&page_size=10", nil)
	rec := httptest.NewRecorder()
	c := router.NewContext(req, rec)

	if err := routes.ListDomainEvents(c); err != nil {
		t.Fatalf("list domain events: %v", err)
	}

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	body := strings.TrimSpace(rec.Body.String())
	if !strings.Contains(body, `"success":false`) || !strings.Contains(body, `"code":"validation"`) {
		t.Fatalf("expected validation envelope, got %s", body)
	}
}

func TestListDomainEventDeliveriesReturnsSelectedEventRecords(t *testing.T) {
	setupRouteTestDBs(t)
	appDB, err := db.DefaultManager.GetEngine("app")
	if err != nil {
		t.Fatalf("get app engine: %v", err)
	}
	seedRouteDomainEventsForPagination(t, appDB, 1)
	seedRouteDomainEventDeliveries(t, appDB, "route-event-01")

	router := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/events/route-event-01/deliveries", nil)
	rec := httptest.NewRecorder()
	c := router.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("route-event-01")

	if err := routes.ListDomainEventDeliveries(c); err != nil {
		t.Fatalf("list domain event deliveries: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var envelope struct {
		Success bool                                 `json:"success"`
		Data    []routes.DomainEventDeliveryResponse `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !envelope.Success {
		t.Fatalf("expected success envelope, got %s", rec.Body.String())
	}
	if len(envelope.Data) != 2 {
		t.Fatalf("expected two delivery items, got %#v", envelope.Data)
	}
	if envelope.Data[0].Subscriber != "audit.record_order_paid" || envelope.Data[1].Status != "failed" {
		t.Fatalf("unexpected delivery records: %#v", envelope.Data)
	}
}

func seedRouteDomainEventsForPagination(t *testing.T, appDB *sqlx.Engine, count int) {
	t.Helper()

	query := `
		INSERT INTO domain_events (
			id, topic, aggregate_type, aggregate_id, payload_json, metadata_json, occurred_at, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`
	for i := 1; i <= count; i++ {
		_, err := appDB.ExecP(query,
			fmt.Sprintf("route-event-%02d", i),
			"order.paid",
			"order",
			fmt.Sprintf("route-order-%02d", i),
			fmt.Sprintf(`{"order_id":"route-order-%02d"}`, i),
			"{}",
			fmt.Sprintf("2026-01-01T00:00:%02dZ", i),
			fmt.Sprintf("2026-01-01 00:00:%02d", i),
		)
		if err != nil {
			t.Fatalf("insert route domain event %d: %v", i, err)
		}
	}
}

func seedRouteDomainEventDeliveries(t *testing.T, appDB *sqlx.Engine, eventID string) {
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
		{"route-delivery-01", "audit.record_order_paid", "succeeded", 1, "", "2026-01-01 00:00:01"},
		{"route-delivery-02", "points.award_on_order_paid", "failed", 2, "boom", "2026-01-01 00:00:02"},
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
			t.Fatalf("insert route domain event delivery %s: %v", row.id, err)
		}
	}
}

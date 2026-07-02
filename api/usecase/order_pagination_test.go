package usecase_test

import (
	"fmt"
	"testing"

	fwusecase "github.com/tfnick/go-svelte-starter/api/framework/usecase"
	"github.com/tfnick/go-svelte-starter/api/usecase"
	"github.com/tfnick/sqlx"
)

func TestGetUserOrdersReturnsRequestedPageAndMetadata(t *testing.T) {
	manager := setupUsecaseOrderTxDB(t)
	appDB, err := manager.GetEngine("app")
	if err != nil {
		t.Fatalf("get app engine: %v", err)
	}
	const seedUserID = "019ea0c1-0001-7000-8000-000000000001"
	seedUserOrdersForPagination(t, appDB, seedUserID, 5)

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	ctx.Actor.Authenticated = true
	ctx.Actor.UserID = seedUserID
	result, err := usecase.GetUserOrders(ctx, usecase.UserOrdersQry{
		UserID:   seedUserID,
		Page:     2,
		PageSize: 2,
	})
	if err != nil {
		t.Fatalf("get user orders: %v", err)
	}

	if len(result.Items) != 2 {
		t.Fatalf("expected two orders on page 2, got %d", len(result.Items))
	}
	if result.Items[0].ID != "order-03" || result.Items[1].ID != "order-02" {
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

func TestListMyOrdersUsesAuthenticatedActorAsOwner(t *testing.T) {
	manager := setupUsecaseOrderTxDB(t)
	appDB, err := manager.GetEngine("app")
	if err != nil {
		t.Fatalf("get app engine: %v", err)
	}
	const actorUserID = "019ea0c1-0001-7000-8000-000000000001"
	const otherUserID = "019ea0c1-0002-7000-8000-000000000002"
	ensureUsecaseTestUser(t, appDB, otherUserID)
	seedUserOrdersForPagination(t, appDB, actorUserID, 2)
	seedUserOrdersForPaginationWithPrefix(t, appDB, otherUserID, "other-order", 1)

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	ctx.Actor.Authenticated = true
	ctx.Actor.UserID = actorUserID
	result, err := usecase.ListMyOrders(ctx, usecase.ListMyOrdersQry{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("list my orders: %v", err)
	}

	if len(result.Items) != 2 {
		t.Fatalf("expected actor-owned orders only, got %#v", result.Items)
	}
	for _, order := range result.Items {
		if order.UserID != actorUserID {
			t.Fatalf("expected only actor orders, got %#v", order)
		}
	}
}

func TestListAdminOrdersRequiresAdminAndAllowsFilters(t *testing.T) {
	manager := setupUsecaseOrderTxDB(t)
	appDB, err := manager.GetEngine("app")
	if err != nil {
		t.Fatalf("get app engine: %v", err)
	}
	const seedUserID = "019ea0c1-0001-7000-8000-000000000001"
	seedUserOrdersForPagination(t, appDB, seedUserID, 3)
	if _, err := appDB.ExecP(`UPDATE orders SET status = ? WHERE id = ?`, "paid", "order-02"); err != nil {
		t.Fatalf("mark paid order: %v", err)
	}

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	ctx.Actor.Authenticated = true
	if _, err := usecase.ListAdminOrders(ctx, usecase.ListAdminOrdersQry{}); fwusecase.CodeOf(err) != fwusecase.CodeForbidden {
		t.Fatalf("expected forbidden for non-admin, got %v", err)
	}

	ctx.Actor.IsAdmin = true
	result, err := usecase.ListAdminOrders(ctx, usecase.ListAdminOrdersQry{
		UserID:   seedUserID,
		Status:   "paid",
		Page:     1,
		PageSize: 10,
	})
	if err != nil {
		t.Fatalf("list admin orders: %v", err)
	}
	if len(result.Items) != 1 || result.Items[0].ID != "order-02" {
		t.Fatalf("expected filtered paid order, got %#v", result.Items)
	}
}

func TestGetUserOrdersRejectsCrossUserForNonAdmin(t *testing.T) {
	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	ctx.Actor.Authenticated = true
	ctx.Actor.UserID = "user-1"

	_, err := usecase.GetUserOrders(ctx, usecase.UserOrdersQry{
		UserID:   "user-2",
		Page:     1,
		PageSize: 10,
	})
	if fwusecase.CodeOf(err) != fwusecase.CodeForbidden {
		t.Fatalf("expected forbidden for cross-user legacy query, got %v", err)
	}
}

func seedUserOrdersForPagination(t *testing.T, appDB *sqlx.Engine, userID string, count int) {
	seedUserOrdersForPaginationWithPrefix(t, appDB, userID, "order", count)
}

func seedUserOrdersForPaginationWithPrefix(t *testing.T, appDB *sqlx.Engine, userID string, idPrefix string, count int) {
	t.Helper()

	query := `INSERT INTO orders (id, user_id, amount, status, created_at) VALUES (?, ?, ?, ?, ?)`
	for i := 1; i <= count; i++ {
		_, err := appDB.ExecP(query,
			fmt.Sprintf("%s-%02d", idPrefix, i),
			userID,
			int64(i*100),
			"pending",
			fmt.Sprintf("2026-01-01 00:00:%02d", i),
		)
		if err != nil {
			t.Fatalf("insert order %d: %v", i, err)
		}
	}
}

func ensureUsecaseTestUser(t *testing.T, appDB *sqlx.Engine, userID string) {
	t.Helper()

	if _, err := appDB.ExecP(`
		INSERT OR IGNORE INTO users (id, name, email, password_hash, email_verified, is_active)
		VALUES (?, ?, ?, '', 1, 1)
	`, userID, "Order Test User", userID+"@example.com"); err != nil {
		t.Fatalf("insert test user: %v", err)
	}
}

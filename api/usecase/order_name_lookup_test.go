package usecase_test

import (
	"context"
	"testing"

	fwusecase "github.com/tfnick/go-svelte-starter/api/framework/usecase"
	"github.com/tfnick/go-svelte-starter/api/models"
	"github.com/tfnick/go-svelte-starter/api/usecase"
)

func TestOrderUsecaseResolvesDisplayNamesWithFrameworkLookup(t *testing.T) {
	manager := setupUsecaseOrderTxDB(t)

	const seedUserID = "019ea0c1-0001-7000-8000-000000000001"
	const seedProductID = "019ea0c1-0004-7000-8000-000000000001"

	expectedUser, err := models.GetUserByID(context.Background(), seedUserID)
	if err != nil {
		t.Fatalf("load expected user: %v", err)
	}
	expectedProduct, err := models.GetProductByID(context.Background(), seedProductID)
	if err != nil {
		t.Fatalf("load expected product: %v", err)
	}
	appDB, err := manager.GetEngine("app")
	if err != nil {
		t.Fatalf("get app engine: %v", err)
	}
	if _, err := appDB.ExecP(`UPDATE products SET creem_product_id = ?, enabled = 1 WHERE id = ?`, "prod_lookup", seedProductID); err != nil {
		t.Fatalf("update checkout product: %v", err)
	}

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	ctx.Actor = fwusecase.ActorContext{Authenticated: true, UserID: seedUserID}
	created, err := usecase.CreateOrder(ctx, usecase.CreateOrderCmd{
		UserID:    seedUserID,
		ProductID: seedProductID,
	})
	if err != nil {
		t.Fatalf("create order: %v", err)
	}
	if created.UserName != expectedUser.Name {
		t.Fatalf("expected created order user name %q, got %q", expectedUser.Name, created.UserName)
	}

	detail, err := usecase.GetOrderDetail(ctx, usecase.OrderDetailQry{OrderID: created.ID})
	if err != nil {
		t.Fatalf("get order detail: %v", err)
	}
	if detail.Order.UserName != expectedUser.Name {
		t.Fatalf("expected detail order user name %q, got %q", expectedUser.Name, detail.Order.UserName)
	}
	if len(detail.Items) != 0 {
		t.Fatalf("expected new Creem checkout ledger to have no local items, got %d", len(detail.Items))
	}

	crossUserCtx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	crossUserCtx.Actor = fwusecase.ActorContext{Authenticated: true, UserID: "019ea0c1-9999-7000-8000-000000000999"}
	if _, err := usecase.GetOrderDetail(crossUserCtx, usecase.OrderDetailQry{OrderID: created.ID}); fwusecase.CodeOf(err) != fwusecase.CodeForbidden {
		t.Fatalf("expected forbidden for cross-user order detail, got %v", err)
	}

	adminCtx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	adminCtx.Actor = fwusecase.ActorContext{Authenticated: true, UserID: "admin-user", IsAdmin: true}
	if _, err := usecase.GetOrderDetail(adminCtx, usecase.OrderDetailQry{OrderID: created.ID}); err != nil {
		t.Fatalf("admin should read order detail: %v", err)
	}

	if _, err := appDB.ExecP(`
		INSERT INTO orders (id, user_id, amount, status, created_at)
		VALUES (?, ?, ?, 'pending', '2026-01-01 00:00:00')
	`, "lookup-order-with-item", seedUserID, int64(699900)); err != nil {
		t.Fatalf("insert legacy order: %v", err)
	}
	if _, err := appDB.ExecP(`
		INSERT INTO order_items (id, order_id, product_id, quantity, price)
		VALUES (?, ?, ?, 1, ?)
	`, "lookup-order-item", "lookup-order-with-item", seedProductID, int64(699900)); err != nil {
		t.Fatalf("insert legacy order item: %v", err)
	}

	legacyDetail, err := usecase.GetOrderDetail(ctx, usecase.OrderDetailQry{OrderID: "lookup-order-with-item"})
	if err != nil {
		t.Fatalf("get legacy order detail: %v", err)
	}
	if legacyDetail.Order.UserName != expectedUser.Name {
		t.Fatalf("expected legacy detail order user name %q, got %q", expectedUser.Name, legacyDetail.Order.UserName)
	}
	if len(legacyDetail.Items) != 1 {
		t.Fatalf("expected one legacy detail item, got %d", len(legacyDetail.Items))
	}
	if legacyDetail.Items[0].ProductName != expectedProduct.Name {
		t.Fatalf("expected legacy detail item product name %q, got %q", expectedProduct.Name, legacyDetail.Items[0].ProductName)
	}
}

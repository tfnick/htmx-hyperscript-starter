package usecase_test

import (
	"path/filepath"
	"testing"

	"github.com/tfnick/go-svelte-starter/api/db"
	fwusecase "github.com/tfnick/go-svelte-starter/api/framework/usecase"
	"github.com/tfnick/go-svelte-starter/api/usecase"
)

func setupUsecaseOrderTxDB(t testing.TB) *db.DBManager {
	t.Helper()

	previous := db.DefaultManager
	manager := db.NewDBManager()
	db.DefaultManager = manager

	dir := t.TempDir()
	t.Cleanup(func() {
		_ = manager.Close()
		db.DefaultManager = previous
	})

	if err := manager.Open("app", "sqlite", filepath.Join(dir, "app.db")); err != nil {
		t.Fatalf("open app db: %v", err)
	}
	if err := manager.AutoMigrate("app"); err != nil {
		t.Fatalf("migrate app db: %v", err)
	}
	if err := manager.Open("shared", "sqlite", filepath.Join(dir, "shared.db")); err != nil {
		t.Fatalf("open shared db: %v", err)
	}
	if err := manager.AutoMigrate("shared"); err != nil {
		t.Fatalf("migrate shared db: %v", err)
	}

	return manager
}

func TestCreateOrderCreatesPendingLedgerForSelectedProduct(t *testing.T) {
	manager := setupUsecaseOrderTxDB(t)

	appDB, err := manager.GetDB("app")
	if err != nil {
		t.Fatalf("get app db: %v", err)
	}

	if _, err := appDB.Exec(`INSERT INTO users (id, name, email, password_hash, email_verified, is_active) VALUES ('u1', 'Ada', 'ada@example.com', '', 1, 1)`); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	seedUsecaseCheckoutProduct(t, appDB, "p1", "prod_usecase")

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	order, err := usecase.CreateOrder(ctx, usecase.CreateOrderCmd{UserID: "u1", ProductID: "p1"})
	if err != nil {
		t.Fatalf("create order: %v", err)
	}
	if order.ID == "" {
		t.Fatalf("expected generated order id")
	}
	if order.Status != "pending" {
		t.Fatalf("expected pending order, got %q", order.Status)
	}
	if order.Amount != 0 {
		t.Fatalf("expected Creem-priced ledger amount 0, got %d", order.Amount)
	}
	if order.UserName != "Ada" {
		t.Fatalf("expected resolved user name Ada, got %q", order.UserName)
	}
	if order.ProductID != "p1" || order.ProductName != "Premium Month" {
		t.Fatalf("expected resolved product, got %#v", order)
	}

	var itemCount int
	if err := appDB.Get(&itemCount, `SELECT COUNT(*) FROM order_items WHERE order_id = ?`, order.ID); err != nil {
		t.Fatalf("count order items: %v", err)
	}
	if itemCount != 0 {
		t.Fatalf("expected no local order items for Creem checkout ledger, got %d", itemCount)
	}
}

func TestCreateOrderIgnoresLegacyItemsForCreemCheckoutLedger(t *testing.T) {
	manager := setupUsecaseOrderTxDB(t)

	appDB, err := manager.GetDB("app")
	if err != nil {
		t.Fatalf("get app db: %v", err)
	}

	if _, err := appDB.Exec(`INSERT INTO users (id, name, email, password_hash, email_verified, is_active) VALUES ('u1', 'Ada', 'ada@example.com', '', 1, 1)`); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	seedUsecaseCheckoutProduct(t, appDB, "p1", "prod_usecase")

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	order, err := usecase.CreateOrder(ctx, usecase.CreateOrderCmd{
		UserID:    "u1",
		ProductID: "p1",
		Items: []usecase.CreateOrderItemCmd{
			{ProductID: "missing-product", Quantity: 0},
		},
	})
	if err != nil {
		t.Fatalf("create order with ignored legacy items: %v", err)
	}
	if order.Amount != 0 {
		t.Fatalf("expected legacy item payload not to set local amount, got %d", order.Amount)
	}

	var itemCount int
	if err := appDB.Get(&itemCount, `SELECT COUNT(*) FROM order_items WHERE order_id = ?`, order.ID); err != nil {
		t.Fatalf("count order items: %v", err)
	}
	if itemCount != 0 {
		t.Fatalf("expected ignored legacy items not to persist, got %d", itemCount)
	}
}

func TestCreateOrderRequiresExistingUser(t *testing.T) {
	manager := setupUsecaseOrderTxDB(t)

	appDB, err := manager.GetDB("app")
	if err != nil {
		t.Fatalf("get app db: %v", err)
	}

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	_, err = usecase.CreateOrder(ctx, usecase.CreateOrderCmd{
		UserID:    "missing-user",
		ProductID: "p1",
	})
	if err == nil {
		t.Fatalf("expected missing user failure")
	}
	if fwusecase.CodeOf(err) != fwusecase.CodeValidation {
		t.Fatalf("expected validation error code, got %q: %v", fwusecase.CodeOf(err), err)
	}

	var orderCount int
	if err := appDB.Get(&orderCount, `SELECT COUNT(*) FROM orders`); err != nil {
		t.Fatalf("count orders: %v", err)
	}
	if orderCount != 0 {
		t.Fatalf("expected no order for missing user, found %d", orderCount)
	}
}

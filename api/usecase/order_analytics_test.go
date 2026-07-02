package usecase_test

import (
	"testing"
	"time"

	"github.com/tfnick/go-svelte-starter/api/framework/timefmt"
	fwusecase "github.com/tfnick/go-svelte-starter/api/framework/usecase"
	"github.com/tfnick/go-svelte-starter/api/usecase"
	"github.com/tfnick/sqlx"
)

func TestListAdminOrdersFiltersByUserQueryAndCreatedTime(t *testing.T) {
	manager := setupUsecaseOrderTxDB(t)
	appDB, err := manager.GetEngine("app")
	if err != nil {
		t.Fatalf("get app engine: %v", err)
	}
	seedAdminOrderManagementFixture(t, appDB)

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	ctx.Actor.Authenticated = true
	if _, err := usecase.ListAdminOrders(ctx, usecase.ListAdminOrdersQry{Page: 1, PageSize: 10}); fwusecase.CodeOf(err) != fwusecase.CodeForbidden {
		t.Fatalf("expected forbidden for non-admin, got %v", err)
	}

	ctx.Actor.IsAdmin = true
	result, err := usecase.ListAdminOrders(ctx, usecase.ListAdminOrdersQry{
		UserQuery: "bea@example.com",
		Status:    "paid",
		StartTime: "2030-01-01T00:00:02Z",
		EndTime:   "2030-01-01T00:00:04Z",
		Page:      1,
		PageSize:  1,
	})
	if err != nil {
		t.Fatalf("list admin orders: %v", err)
	}

	if len(result.Items) != 1 || result.Items[0].ID != "admin-order-04" {
		t.Fatalf("expected newest filtered order on page 1, got %#v", result.Items)
	}
	if result.Pagination.TotalItems != 2 || result.Pagination.TotalPages != 2 || !result.Pagination.HasNext {
		t.Fatalf("unexpected pagination: %#v", result.Pagination)
	}
}

func TestListAdminOrdersRejectsInvalidTimeRange(t *testing.T) {
	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	ctx.Actor.Authenticated = true
	ctx.Actor.IsAdmin = true

	_, err := usecase.ListAdminOrders(ctx, usecase.ListAdminOrdersQry{
		StartTime: "2030-01-02T00:00:00Z",
		EndTime:   "2030-01-01T00:00:00Z",
		Page:      1,
		PageSize:  10,
	})
	if fwusecase.CodeOf(err) != fwusecase.CodeValidation {
		t.Fatalf("expected validation for inverted time range, got %v", err)
	}

	_, err = usecase.ListAdminOrders(ctx, usecase.ListAdminOrdersQry{
		StartTime: "not-a-date",
		Page:      1,
		PageSize:  10,
	})
	if fwusecase.CodeOf(err) != fwusecase.CodeValidation {
		t.Fatalf("expected validation for invalid start_time, got %v", err)
	}
}

func seedAdminOrderManagementFixture(t *testing.T, appDB *sqlx.Engine) {
	t.Helper()

	users := []struct {
		id    string
		name  string
		email string
	}{
		{"admin-order-user-a", "Ada Lovelace", "ada@example.com"},
		{"admin-order-user-b", "Bea Hopper", "bea@example.com"},
	}
	for _, user := range users {
		if _, err := appDB.ExecP(`
			INSERT INTO users (id, name, email, password_hash, email_verified, is_active)
			VALUES (?, ?, ?, '', 1, 1)
		`, user.id, user.name, user.email); err != nil {
			t.Fatalf("insert admin order user: %v", err)
		}
	}

	orders := []struct {
		id        string
		userID    string
		status    string
		createdAt string
	}{
		{"admin-order-01", "admin-order-user-a", "paid", "2030-01-01 00:00:01"},
		{"admin-order-02", "admin-order-user-b", "pending", "2030-01-01 00:00:02"},
		{"admin-order-03", "admin-order-user-b", "paid", "2030-01-01 00:00:03"},
		{"admin-order-04", "admin-order-user-b", "paid", "2030-01-01 00:00:04"},
		{"admin-order-05", "admin-order-user-b", "paid", "2030-01-01 00:00:05"},
	}
	for _, order := range orders {
		if _, err := appDB.ExecP(`
			INSERT INTO orders (id, user_id, amount, status, created_at)
			VALUES (?, ?, 1000, ?, ?)
		`, order.id, order.userID, order.status, order.createdAt); err != nil {
			t.Fatalf("insert admin order: %v", err)
		}
	}
}

func sqliteTime(value time.Time) string {
	return timefmt.SQLiteDateTime(value.UTC().Truncate(time.Second))
}

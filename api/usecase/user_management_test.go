package usecase_test

import (
	"fmt"
	"testing"

	fwusecase "github.com/tfnick/go-svelte-starter/api/framework/usecase"
	"github.com/tfnick/go-svelte-starter/api/usecase"
	"github.com/tfnick/sqlx"
)

func TestListUsersReturnsRequestedPageAndMetadata(t *testing.T) {
	manager := setupUsecaseOrderTxDB(t)
	appDB, err := manager.GetEngine("app")
	if err != nil {
		t.Fatalf("get app engine: %v", err)
	}
	seedUsersForManagement(t, appDB, 5)

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	result, err := usecase.ListUsers(ctx, usecase.ListUsersQry{
		Page:     2,
		PageSize: 2,
	})
	if err != nil {
		t.Fatalf("list users: %v", err)
	}

	if len(result.Items) != 2 {
		t.Fatalf("expected two users on page 2, got %d", len(result.Items))
	}
	if result.Items[0].ID != "user-03" || result.Items[1].ID != "user-02" {
		t.Fatalf("expected stable created_at desc page order, got %#v", result.Items)
	}

	page := result.Pagination
	if page.Page != 2 || page.PageSize != 2 || page.TotalItems != 8 || page.TotalPages != 4 {
		t.Fatalf("unexpected pagination metadata: %#v", page)
	}
	if !page.HasPrevious || !page.HasNext {
		t.Fatalf("expected page 2 of 4 to have previous and next: %#v", page)
	}
}

func TestListUsersIncludesRegistrationMetadataAndCreatedAtFilters(t *testing.T) {
	manager := setupUsecaseOrderTxDB(t)
	appDB, err := manager.GetEngine("app")
	if err != nil {
		t.Fatalf("get app engine: %v", err)
	}
	seedUsersForManagement(t, appDB, 5)
	if _, err := appDB.ExecP(`
		INSERT INTO user_registration_profiles (
			id, user_id, registration_ip, registration_country, registration_region, registration_user_agent, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, "profile-user-04", "user-04", "203.0.113.40", "China", "Shanghai", "Mozilla/5.0 list-test", "2030-01-02 00:00:00", "2030-01-02 00:00:00"); err != nil {
		t.Fatalf("insert registration profile: %v", err)
	}

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	result, err := usecase.ListUsers(ctx, usecase.ListUsersQry{
		StartTime: "2030-01-01T00:00:02Z",
		EndTime:   "2030-01-01T00:00:04Z",
		Page:      1,
		PageSize:  10,
	})
	if err != nil {
		t.Fatalf("list users: %v", err)
	}

	if len(result.Items) != 3 {
		t.Fatalf("expected three filtered users, got %#v", result.Items)
	}
	if result.Pagination.TotalItems != 3 {
		t.Fatalf("expected filtered total 3, got %#v", result.Pagination)
	}
	if result.Items[0].ID != "user-04" {
		t.Fatalf("expected newest filtered user first, got %#v", result.Items)
	}
	if result.Items[0].RegistrationIP != "203.0.113.40" ||
		result.Items[0].RegistrationCountry != "China" ||
		result.Items[0].RegistrationRegion != "Shanghai" ||
		result.Items[0].RegistrationUserAgent != "Mozilla/5.0 list-test" {
		t.Fatalf("expected registration metadata, got %#v", result.Items[0])
	}
	if result.Items[1].RegistrationIP != "" {
		t.Fatalf("expected missing registration metadata to stay empty, got %#v", result.Items[1])
	}
}

func TestListUsersRejectsInvalidCreatedAtFilters(t *testing.T) {
	setupUsecaseOrderTxDB(t)

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	_, err := usecase.ListUsers(ctx, usecase.ListUsersQry{
		StartTime: "not-a-time",
		Page:      1,
		PageSize:  10,
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
	if fwusecase.CodeOf(err) != fwusecase.CodeValidation {
		t.Fatalf("expected validation error, got %v", err)
	}

	_, err = usecase.ListUsers(ctx, usecase.ListUsersQry{
		StartTime: "2030-01-02T00:00:00Z",
		EndTime:   "2030-01-01T00:00:00Z",
		Page:      1,
		PageSize:  10,
	})
	if err == nil {
		t.Fatal("expected validation error for reversed time range")
	}
	if fwusecase.CodeOf(err) != fwusecase.CodeValidation {
		t.Fatalf("expected validation error for reversed time range, got %v", err)
	}
}

func TestSetUserActiveDisablesAndEnablesUser(t *testing.T) {
	manager := setupUsecaseOrderTxDB(t)
	appDB, err := manager.GetEngine("app")
	if err != nil {
		t.Fatalf("get app engine: %v", err)
	}
	seedUsersForManagement(t, appDB, 1)

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	disabled, err := usecase.SetUserActive(ctx, usecase.SetUserActiveCmd{
		ID:     "user-01",
		Active: false,
	})
	if err != nil {
		t.Fatalf("disable user: %v", err)
	}
	if disabled.IsActive {
		t.Fatalf("expected disabled user, got %#v", disabled)
	}

	enabled, err := usecase.SetUserActive(ctx, usecase.SetUserActiveCmd{
		ID:     "user-01",
		Active: true,
	})
	if err != nil {
		t.Fatalf("enable user: %v", err)
	}
	if !enabled.IsActive {
		t.Fatalf("expected enabled user, got %#v", enabled)
	}
}

func TestSetUserActiveRejectsDisablingCurrentUser(t *testing.T) {
	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	ctx.Actor = fwusecase.ActorContext{
		Authenticated: true,
		UserID:        "user-01",
	}

	_, err := usecase.SetUserActive(ctx, usecase.SetUserActiveCmd{
		ID:     "user-01",
		Active: false,
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
	if fwusecase.CodeOf(err) != fwusecase.CodeValidation {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestSetUserActiveReturnsNotFoundForMissingUser(t *testing.T) {
	setupUsecaseOrderTxDB(t)
	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)

	_, err := usecase.SetUserActive(ctx, usecase.SetUserActiveCmd{
		ID:     "missing-user",
		Active: false,
	})
	if err == nil {
		t.Fatal("expected not found error")
	}
	if fwusecase.CodeOf(err) != fwusecase.CodeNotFound {
		t.Fatalf("expected not found error, got %v", err)
	}
}

func seedUsersForManagement(t *testing.T, appDB *sqlx.Engine, count int) {
	t.Helper()

	query := `
		INSERT INTO users (
			id, name, email, password_hash, email_verified, is_active, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`
	for i := 1; i <= count; i++ {
		createdAt := fmt.Sprintf("2030-01-01 00:00:%02d", i)
		_, err := appDB.ExecP(query,
			fmt.Sprintf("user-%02d", i),
			fmt.Sprintf("User %02d", i),
			fmt.Sprintf("user%02d@example.com", i),
			"",
			1,
			1,
			createdAt,
			createdAt,
		)
		if err != nil {
			t.Fatalf("insert user %d: %v", i, err)
		}
	}
}

package routes_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/tfnick/go-svelte-starter/api/db"
	fwcontext "github.com/tfnick/go-svelte-starter/api/framework/http/context"
	"github.com/tfnick/go-svelte-starter/api/models"
	"github.com/tfnick/go-svelte-starter/api/routes"
	"github.com/tfnick/sqlx"
)

const routeSeedAdminUserID = "019ea0c1-0001-7000-8000-000000000001"

func TestGetAllUsersReturnsPaginatedEnvelope(t *testing.T) {
	setupRouteTestDBs(t)
	appDB, err := db.DefaultManager.GetEngine("app")
	if err != nil {
		t.Fatalf("get app engine: %v", err)
	}
	seedRouteUsersForManagement(t, appDB, 5)

	router := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/users?page=2&page_size=2", nil)
	rec := httptest.NewRecorder()
	c := router.NewContext(req, rec)

	if err := routes.GetAllUsers(c); err != nil {
		t.Fatalf("get all users: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var envelope struct {
		Success bool                 `json:"success"`
		Data    routes.UsersResponse `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !envelope.Success {
		t.Fatalf("expected success envelope, got %s", rec.Body.String())
	}
	if len(envelope.Data.Items) != 2 {
		t.Fatalf("expected two user items, got %#v", envelope.Data.Items)
	}
	if envelope.Data.Items[0].ID != "route-user-03" || envelope.Data.Items[1].ID != "route-user-02" {
		t.Fatalf("expected stable page items, got %#v", envelope.Data.Items)
	}
	if envelope.Data.Pagination.TotalItems != 8 || envelope.Data.Pagination.TotalPages != 4 {
		t.Fatalf("unexpected pagination metadata: %#v", envelope.Data.Pagination)
	}
}

func TestGetAllUsersReturnsRegistrationMetadataAndCreatedAtFilters(t *testing.T) {
	setupRouteTestDBs(t)
	appDB, err := db.DefaultManager.GetEngine("app")
	if err != nil {
		t.Fatalf("get app engine: %v", err)
	}
	seedRouteUsersForManagement(t, appDB, 5)
	if _, err := appDB.ExecP(`
		INSERT INTO user_registration_profiles (
			id, user_id, registration_ip, registration_country, registration_region, registration_user_agent, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, "route-profile-user-04", "route-user-04", "198.51.100.40", "China", "Shanghai", "route-agent", "2030-01-02 00:00:00", "2030-01-02 00:00:00"); err != nil {
		t.Fatalf("insert registration profile: %v", err)
	}

	router := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/users?page=1&page_size=10&start_time=2030-01-01T00%3A00%3A03Z&end_time=2030-01-01T00%3A00%3A04Z", nil)
	rec := httptest.NewRecorder()
	c := router.NewContext(req, rec)

	if err := routes.GetAllUsers(c); err != nil {
		t.Fatalf("get all users: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var envelope struct {
		Success bool                 `json:"success"`
		Data    routes.UsersResponse `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(envelope.Data.Items) != 2 || envelope.Data.Pagination.TotalItems != 2 {
		t.Fatalf("expected two filtered users, got %#v", envelope.Data)
	}
	if envelope.Data.Items[0].ID != "route-user-04" || envelope.Data.Items[1].ID != "route-user-03" {
		t.Fatalf("expected filtered created_at order, got %#v", envelope.Data.Items)
	}
	if envelope.Data.Items[0].RegistrationIP != "198.51.100.40" ||
		envelope.Data.Items[0].RegistrationCountry != "China" ||
		envelope.Data.Items[0].RegistrationRegion != "Shanghai" ||
		envelope.Data.Items[0].RegistrationUserAgent != "route-agent" {
		t.Fatalf("expected registration metadata in DTO, got %#v", envelope.Data.Items[0])
	}
}

func TestGetAllUsersRejectsInvalidPageQuery(t *testing.T) {
	setupRouteTestDBs(t)

	router := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/users?page=0&page_size=10", nil)
	rec := httptest.NewRecorder()
	c := router.NewContext(req, rec)

	if err := routes.GetAllUsers(c); err != nil {
		t.Fatalf("get all users: %v", err)
	}

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	body := strings.TrimSpace(rec.Body.String())
	if !strings.Contains(body, `"success":false`) || !strings.Contains(body, `"code":"validation"`) {
		t.Fatalf("expected validation envelope, got %s", body)
	}
}

func TestSetUserActiveReturnsUpdatedUser(t *testing.T) {
	setupRouteTestDBs(t)
	appDB, err := db.DefaultManager.GetEngine("app")
	if err != nil {
		t.Fatalf("get app engine: %v", err)
	}
	seedRouteUsersForManagement(t, appDB, 1)

	router := echo.New()
	req := httptest.NewRequest(http.MethodPatch, "/api/users/route-user-01/active", bytes.NewBufferString(`{"active":false}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := router.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("route-user-01")
	fwcontext.SetCurrentUser(c, &models.User{ID: routeSeedAdminUserID, Name: "Operator"})

	if err := routes.SetUserActive(c); err != nil {
		t.Fatalf("set user active: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var envelope struct {
		Success bool                `json:"success"`
		Data    routes.UserResponse `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !envelope.Success || envelope.Data.IsActive {
		t.Fatalf("expected disabled user response, got %s", rec.Body.String())
	}
}

func TestSetUserActiveRejectsCurrentUserDisable(t *testing.T) {
	setupRouteTestDBs(t)

	router := echo.New()
	req := httptest.NewRequest(http.MethodPatch, "/api/users/"+routeSeedAdminUserID+"/active", bytes.NewBufferString(`{"active":false}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := router.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(routeSeedAdminUserID)
	fwcontext.SetCurrentUser(c, &models.User{ID: routeSeedAdminUserID, Name: "Operator"})

	if err := routes.SetUserActive(c); err != nil {
		t.Fatalf("set user active: %v", err)
	}

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	body := strings.TrimSpace(rec.Body.String())
	if !strings.Contains(body, `"code":"validation"`) || !strings.Contains(body, "cannot disable current user") {
		t.Fatalf("expected current user validation envelope, got %s", body)
	}
}

func TestSetUserActiveReturnsNotFoundForMissingUser(t *testing.T) {
	setupRouteTestDBs(t)

	router := echo.New()
	req := httptest.NewRequest(http.MethodPatch, "/api/users/missing-user/active", bytes.NewBufferString(`{"active":false}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := router.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("missing-user")
	fwcontext.SetCurrentUser(c, &models.User{ID: routeSeedAdminUserID, Name: "Operator"})

	if err := routes.SetUserActive(c); err != nil {
		t.Fatalf("set user active: %v", err)
	}

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
	body := strings.TrimSpace(rec.Body.String())
	if !strings.Contains(body, `"code":"not_found"`) {
		t.Fatalf("expected not found envelope, got %s", body)
	}
}

func seedRouteUsersForManagement(t *testing.T, appDB *sqlx.Engine, count int) {
	t.Helper()

	query := `
		INSERT INTO users (
			id, name, email, password_hash, email_verified, is_active, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`
	for i := 1; i <= count; i++ {
		createdAt := fmt.Sprintf("2030-01-01 00:00:%02d", i)
		_, err := appDB.ExecP(query,
			fmt.Sprintf("route-user-%02d", i),
			fmt.Sprintf("Route User %02d", i),
			fmt.Sprintf("route-user%02d@example.com", i),
			"",
			1,
			1,
			createdAt,
			createdAt,
		)
		if err != nil {
			t.Fatalf("insert route user %d: %v", i, err)
		}
	}
}

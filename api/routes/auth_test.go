package routes_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/tfnick/go-svelte-starter/api/db"
	"github.com/tfnick/go-svelte-starter/api/framework/authz"
	authmiddleware "github.com/tfnick/go-svelte-starter/api/framework/http/middleware"
	"github.com/tfnick/go-svelte-starter/api/routes"
	"github.com/tfnick/go-svelte-starter/api/usecase"
)

func TestCurrentUserResponseIncludesAdminFlag(t *testing.T) {
	response := routes.ToCurrentUserResponse(usecase.UserCo{
		ID:            "019ea0c1-0001-7000-8000-000000000001",
		Name:          "Admin",
		Email:         "admin@example.com",
		EmailVerified: true,
		IsAdmin:       true,
		Role:          authz.RolePlatformAdmin,
		Permissions:   []string{authz.PermissionSettingsManage},
		DataScope:     authz.DataScopePlatform,
	})

	if !response.IsAdmin {
		t.Fatalf("expected admin flag in current user response: %#v", response)
	}
	if response.Role != authz.RolePlatformAdmin || response.DataScope != authz.DataScopePlatform {
		t.Fatalf("expected role and data scope in current user response: %#v", response)
	}
	if len(response.Permissions) != 1 || response.Permissions[0] != authz.PermissionSettingsManage {
		t.Fatalf("expected permissions in current user response: %#v", response)
	}
}

func TestAuthStatusUserResponseIncludesAdminFlag(t *testing.T) {
	response := routes.ToAuthStatusUserResponse(usecase.UserCo{
		ID:          "019ea0c1-0001-7000-8000-000000000001",
		Name:        "Admin",
		IsAdmin:     true,
		Role:        authz.RolePlatformAdmin,
		Permissions: []string{authz.PermissionSettingsManage},
		DataScope:   authz.DataScopePlatform,
	})

	if !response.IsAdmin {
		t.Fatalf("expected admin flag in auth status response: %#v", response)
	}
	if response.Role != authz.RolePlatformAdmin || response.DataScope != authz.DataScopePlatform {
		t.Fatalf("expected role and data scope in auth status response: %#v", response)
	}
	if len(response.Permissions) != 1 || response.Permissions[0] != authz.PermissionSettingsManage {
		t.Fatalf("expected permissions in auth status response: %#v", response)
	}
}

func TestAuthRouteRateLimitReturnsInternalEnvelope(t *testing.T) {
	now := time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC)
	limiter := authmiddleware.NewMemoryRateLimiter()
	rateLimit := limiter.Middleware(authmiddleware.RateLimitConfig{
		MaxRequests: 1,
		Window:      time.Minute,
		KeyPrefix:   "auth-test",
		Now: func() time.Time {
			return now
		},
	})

	router := echo.New()
	router.POST("/api/auth/login", func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	}, rateLimit)

	first := httptest.NewRecorder()
	firstReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"email":"ada@example.com","password":"secret"}`))
	firstReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	firstReq.Header.Set("X-Forwarded-For", "203.0.113.10")
	router.ServeHTTP(first, firstReq)
	if first.Code != http.StatusOK {
		t.Fatalf("expected first request to pass, got %d: %s", first.Code, first.Body.String())
	}

	second := httptest.NewRecorder()
	secondReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"email":"ada@example.com","password":"secret"}`))
	secondReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	secondReq.Header.Set("X-Forwarded-For", "203.0.113.10")
	router.ServeHTTP(second, secondReq)
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("expected rate limited request, got %d: %s", second.Code, second.Body.String())
	}
	if !strings.Contains(second.Body.String(), `"code":"rate_limited"`) {
		t.Fatalf("expected rate_limited envelope, got %s", second.Body.String())
	}
}

func TestRegisterRoutePersistsRegistrationRequestContext(t *testing.T) {
	setupRouteTestDBs(t)
	router := echo.New()
	router.POST("/api/auth/register", routes.Register)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", strings.NewReader(`{"name":"Ada","email":"ada.route@example.com","password":"secret123","utm_source":"xiaohongshu","utm_medium":"social","utm_campaign":"launch"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.Header.Set("X-Forwarded-For", "203.0.113.44")
	req.Header.Set("User-Agent", "route-registration-test")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected register to succeed, got %d: %s", rec.Code, rec.Body.String())
	}

	var response struct {
		Success bool `json:"success"`
		Data    struct {
			User struct {
				ID string `json:"id"`
			} `json:"user"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode register response: %v", err)
	}
	if !response.Success || response.Data.User.ID == "" {
		t.Fatalf("unexpected register response: %s", rec.Body.String())
	}

	appDB, err := db.DefaultManager.GetDB("app")
	if err != nil {
		t.Fatalf("get app db: %v", err)
	}
	var profile struct {
		RegistrationIP        string `db:"registration_ip"`
		RegistrationUserAgent string `db:"registration_user_agent"`
		UtmSource             string `db:"utm_source"`
		UtmMedium             string `db:"utm_medium"`
		UtmCampaign           string `db:"utm_campaign"`
	}
	if err := appDB.Get(&profile, `SELECT registration_ip, registration_user_agent, utm_source, utm_medium, utm_campaign FROM user_registration_profiles WHERE user_id = ?`, response.Data.User.ID); err != nil {
		t.Fatalf("get registration profile: %v", err)
	}
	if profile.RegistrationIP != "203.0.113.44" || profile.RegistrationUserAgent != "route-registration-test" {
		t.Fatalf("unexpected route registration metadata: %#v", profile)
	}
	if profile.UtmSource != "xiaohongshu" || profile.UtmMedium != "social" || profile.UtmCampaign != "launch" {
		t.Fatalf("unexpected route registration UTM metadata: %#v", profile)
	}
}

package middleware

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/tfnick/go-svelte-starter/api/db"
	"github.com/tfnick/go-svelte-starter/api/framework/authz"
	fwauth "github.com/tfnick/go-svelte-starter/api/framework/http/auth"
	fwcontext "github.com/tfnick/go-svelte-starter/api/framework/http/context"
	"github.com/tfnick/go-svelte-starter/api/models"
)

const (
	authSeedUserID     = "019ea0c1-0001-7000-8000-000000000001"
	authSeedOperatorID = "019ea0c1-0001-7000-8000-000000000002"
)

func TestRequireAuthRejectsMissingTokenWithInternalEnvelope(t *testing.T) {
	router := echo.New()
	router.Use(RequireAuth())
	router.GET("/api/protected", func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/protected", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	assertInternalErrorEnvelope(t, rec.Body.String(), "unauthorized", "not logged in")
}

func TestRequireAuthAuthenticatesBearerToken(t *testing.T) {
	setupAuthMiddlewareDB(t)
	t.Setenv(fwauth.EnvJWTSecret, "test-secret")

	token, err := fwauth.IssueUserToken(authSeedUserID)
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}

	router := echo.New()
	router.Use(RequireAuth())
	router.GET("/api/protected", func(c echo.Context) error {
		user := GetCurrentUser(c)
		if user == nil || user.ID != authSeedUserID {
			t.Fatalf("expected current user %s, got %#v", authSeedUserID, user)
		}
		return c.NoContent(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/protected", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer "+token.AccessToken)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestRequireAuthRejectsQueryTokenForNormalAPI(t *testing.T) {
	setupAuthMiddlewareDB(t)
	t.Setenv(fwauth.EnvJWTSecret, "test-secret")

	token, err := fwauth.IssueUserToken(authSeedUserID)
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}

	router := echo.New()
	router.Use(RequireAuth())
	router.GET("/api/protected", func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/protected?access_token="+token.AccessToken, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
	}
	assertInternalErrorEnvelope(t, rec.Body.String(), "unauthorized", "not logged in")
}

func TestRequireAuthAllowsQueryTokenWhenExplicitlyConfigured(t *testing.T) {
	setupAuthMiddlewareDB(t)
	t.Setenv(fwauth.EnvJWTSecret, "test-secret")

	token, err := fwauth.IssueUserToken(authSeedUserID)
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}

	router := echo.New()
	router.Use(RequireAuthWithConfig(AuthConfig{AllowQueryToken: true}))
	router.GET("/api/user/realtime/ws", func(c echo.Context) error {
		user := fwcontext.GetCurrentUser(c)
		if user == nil || user.ID != authSeedUserID {
			t.Fatalf("expected current user %s, got %#v", authSeedUserID, user)
		}
		return c.NoContent(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/user/realtime/ws?access_token="+token.AccessToken, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestRequireAuthDoesNotAcceptLegacySessionCookie(t *testing.T) {
	setupAuthMiddlewareDB(t)

	router := echo.New()
	router.Use(RequireAuth())
	router.GET("/api/protected", func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/protected", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "legacy-session"})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestRequireAdminRejectsNonAdminUser(t *testing.T) {
	router := echo.New()
	router.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			fwcontext.SetCurrentUser(c, &models.User{ID: authSeedOperatorID, Name: "Operator", IsAdmin: 0})
			return next(c)
		}
	})
	router.Use(RequireAdmin())
	router.GET("/api/parameters/integration-channels", func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/parameters/integration-channels", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
	assertInternalErrorEnvelope(t, rec.Body.String(), "forbidden", "admin access is required")
}

func TestRequireAdminAllowsAdminUser(t *testing.T) {
	router := echo.New()
	router.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			fwcontext.SetCurrentUser(c, &models.User{ID: authSeedUserID, Name: "Admin", IsAdmin: 1})
			return next(c)
		}
	})
	router.Use(RequireAdmin())
	router.GET("/api/parameters/integration-channels", func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/parameters/integration-channels", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestRequireAdminAllowsPlatformAdminRole(t *testing.T) {
	router := echo.New()
	router.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			fwcontext.SetCurrentUser(c, &models.User{ID: authSeedUserID, Name: "Admin", Role: authz.RolePlatformAdmin})
			return next(c)
		}
	})
	router.Use(RequireAdmin())
	router.GET("/api/parameters/integration-channels", func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/parameters/integration-channels", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestRequirePermissionAllowsMappedPermission(t *testing.T) {
	router := echo.New()
	router.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			fwcontext.SetCurrentUser(c, &models.User{ID: authSeedUserID, Name: "Admin", Role: authz.RolePlatformAdmin})
			return next(c)
		}
	})
	router.Use(RequirePermission(authz.PermissionSettingsManage))
	router.GET("/api/admin/settings/clarity", func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/admin/settings/clarity", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestRequirePermissionRejectsMissingPermission(t *testing.T) {
	router := echo.New()
	router.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			fwcontext.SetCurrentUser(c, &models.User{ID: authSeedOperatorID, Name: "Operator", Role: authz.RoleUser})
			return next(c)
		}
	})
	router.Use(RequirePermission(authz.PermissionSettingsManage))
	router.GET("/api/admin/settings/clarity", func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/admin/settings/clarity", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
	assertInternalErrorEnvelope(t, rec.Body.String(), "forbidden", "permission is required")
}

func setupAuthMiddlewareDB(t *testing.T) {
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
}

func assertInternalErrorEnvelope(t *testing.T, body string, code string, message string) {
	t.Helper()

	expected := []string{
		`"success":false`,
		`"error":`,
		`"code":"` + code + `"`,
		`"message":"` + message + `"`,
	}
	for _, value := range expected {
		if !strings.Contains(body, value) {
			t.Fatalf("expected %s in body, got %s", value, body)
		}
	}
}

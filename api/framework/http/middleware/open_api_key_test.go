package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/tfnick/go-svelte-starter/api/db"
)

func TestRequireOpenAPIKeyReturnsEnvelopeForMissingKey(t *testing.T) {
	router := echo.New()
	router.Use(RequireOpenAPIKey())
	router.GET("/open-api/v1/account/me", func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/open-api/v1/account/me", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	assertOpenAPIErrorEnvelope(t, rec.Body.String(), "unauthorized", "missing api key")
}

func TestRequireOpenAPIKeyReturnsEnvelopeForInvalidKey(t *testing.T) {
	previous := db.DefaultManager
	db.DefaultManager = db.NewDBManager()
	t.Cleanup(func() {
		db.DefaultManager = previous
	})

	router := echo.New()
	router.Use(RequireOpenAPIKey())
	router.GET("/open-api/v1/account/me", func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/open-api/v1/account/me", nil)
	req.Header.Set("X-API-Key", "invalid")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	assertOpenAPIErrorEnvelope(t, rec.Body.String(), "unauthorized", "invalid api key")
}

func assertOpenAPIErrorEnvelope(t *testing.T, body string, code string, message string) {
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

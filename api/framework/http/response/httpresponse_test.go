package httpresponse_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	httpresponse "github.com/tfnick/go-svelte-starter/api/framework/http/response"
)

func TestErrorUsesInternalAPIEnvelope(t *testing.T) {
	router := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/example", nil)
	rec := httptest.NewRecorder()
	c := router.NewContext(req, rec)

	if err := httpresponse.Error(c, http.StatusTeapot, "short and safe"); err != nil {
		t.Fatalf("error response: %v", err)
	}

	if rec.Code != http.StatusTeapot {
		t.Fatalf("expected status %d, got %d", http.StatusTeapot, rec.Code)
	}
	if strings.TrimSpace(rec.Body.String()) != `{"success":false,"error":{"code":"internal_error","message":"short and safe"}}` {
		t.Fatalf("expected error envelope body, got %s", rec.Body.String())
	}
}

func TestMessageUsesInternalAPIEnvelope(t *testing.T) {
	router := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/example", nil)
	rec := httptest.NewRecorder()
	c := router.NewContext(req, rec)

	if err := httpresponse.Message(c, http.StatusCreated, "created"); err != nil {
		t.Fatalf("message response: %v", err)
	}

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, rec.Code)
	}
	if strings.TrimSpace(rec.Body.String()) != `{"success":true,"data":{"message":"created"}}` {
		t.Fatalf("expected success message envelope, got %s", rec.Body.String())
	}
}

func TestSuccessUsesInternalAPIEnvelope(t *testing.T) {
	router := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/example", nil)
	rec := httptest.NewRecorder()
	c := router.NewContext(req, rec)

	if err := httpresponse.OK(c, map[string]string{"id": "u1"}); err != nil {
		t.Fatalf("success response: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if strings.TrimSpace(rec.Body.String()) != `{"success":true,"data":{"id":"u1"}}` {
		t.Fatalf("expected success envelope, got %s", rec.Body.String())
	}
}

func TestBadRequestUsesValidationCode(t *testing.T) {
	router := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/example", nil)
	rec := httptest.NewRecorder()
	c := router.NewContext(req, rec)

	if err := httpresponse.BadRequest(c, "invalid request data"); err != nil {
		t.Fatalf("bad request response: %v", err)
	}

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if strings.TrimSpace(rec.Body.String()) != `{"success":false,"error":{"code":"validation","message":"invalid request data"}}` {
		t.Fatalf("expected validation error envelope, got %s", rec.Body.String())
	}
}

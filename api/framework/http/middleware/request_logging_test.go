package middleware

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	fwcontext "github.com/tfnick/go-svelte-starter/api/framework/http/context"
	"github.com/tfnick/go-svelte-starter/api/framework/logging"
)

func TestRequestLoggerAddsRequestIDAndSurface(t *testing.T) {
	cleanupRequestLoggingTest(t)

	router := echo.New()
	router.Use(RequestLogger("api"))
	router.GET("/api/ping", func(c echo.Context) error {
		if fwcontext.GetRequestID(c) != "req-123" {
			t.Fatalf("expected request id in context")
		}
		return c.JSON(http.StatusOK, map[string]string{"ok": "true"})
	})

	req := httptest.NewRequest(http.MethodGet, "/api/ping", nil)
	req.Header.Set(fwcontext.RequestIDHeader, "req-123")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if rec.Header().Get(fwcontext.RequestIDHeader) != "req-123" {
		t.Fatalf("expected response request id header")
	}

	logs := readTestLogs(t)
	if !strings.Contains(logs, `"surface":"api"`) {
		t.Fatalf("expected api surface in logs, got %s", logs)
	}
	if !strings.Contains(logs, `"request_id":"req-123"`) {
		t.Fatalf("expected request id in logs, got %s", logs)
	}
	if !strings.Contains(logs, `"route":"/api/ping"`) {
		t.Fatalf("expected route in logs, got %s", logs)
	}
}

func TestRequestLoggerAddsOpenAPIConsumerFields(t *testing.T) {
	cleanupRequestLoggingTest(t)

	router := echo.New()
	router.Use(RequestLogger("open-api"))
	router.GET("/open-api/v1/me", func(c echo.Context) error {
		fwcontext.SetOpenAPIConsumer(c, &fwcontext.OpenAPIConsumerContext{
			KeyID:       "secret-key-id",
			PartnerID:   "partner-1",
			AccountID:   "account-1",
			Environment: "sandbox",
		})
		return c.JSON(http.StatusOK, map[string]string{"ok": "true"})
	})

	req := httptest.NewRequest(http.MethodGet, "/open-api/v1/me", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if rec.Header().Get(fwcontext.RequestIDHeader) == "" {
		t.Fatalf("expected generated response request id header")
	}

	logs := readTestLogs(t)
	if !strings.Contains(logs, `"surface":"open-api"`) {
		t.Fatalf("expected open-api surface in logs, got %s", logs)
	}
	if !strings.Contains(logs, `"partner_id":"partner-1"`) {
		t.Fatalf("expected partner id in logs, got %s", logs)
	}
	if strings.Contains(logs, "secret-key-id") {
		t.Fatalf("did not expect key id in logs, got %s", logs)
	}
}

func cleanupRequestLoggingTest(t *testing.T) {
	t.Helper()

	if err := logging.Close(); err != nil {
		t.Fatalf("close logging before test: %v", err)
	}
	if err := os.RemoveAll("logs"); err != nil {
		t.Fatalf("remove logs before test: %v", err)
	}
	if err := logging.Init(true); err != nil {
		t.Fatalf("init logging: %v", err)
	}

	t.Cleanup(func() {
		if err := logging.Close(); err != nil {
			t.Fatalf("close logging after test: %v", err)
		}
		if err := os.RemoveAll("logs"); err != nil {
			t.Fatalf("remove logs after test: %v", err)
		}
	})
}

func readTestLogs(t *testing.T) string {
	t.Helper()

	if err := logging.Close(); err != nil {
		t.Fatalf("close logging before reading: %v", err)
	}

	content, err := os.ReadFile(logging.DefaultLogPath)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	return string(content)
}

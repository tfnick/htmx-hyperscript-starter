package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
)

func TestMemoryRateLimitRejectsAfterWindowQuota(t *testing.T) {
	now := time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC)
	limiter := NewMemoryRateLimiter()

	router := echo.New()
	router.Use(limiter.Middleware(RateLimitConfig{
		MaxRequests: 2,
		Window:      time.Minute,
		KeyPrefix:   "test",
		Now: func() time.Time {
			return now
		},
	}))
	router.GET("/api/auth/login", func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	})

	for i := 0; i < 2; i++ {
		rec := serveRateLimitTestRequest(router, "203.0.113.10")
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d expected 200, got %d: %s", i+1, rec.Code, rec.Body.String())
		}
	}

	rec := serveRateLimitTestRequest(router, "203.0.113.10")
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"code":"rate_limited"`) {
		t.Fatalf("expected rate_limited envelope, got %s", rec.Body.String())
	}

	now = now.Add(time.Minute)
	rec = serveRateLimitTestRequest(router, "203.0.113.10")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected quota reset after window, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestMemoryRateLimitUsesClientIPKey(t *testing.T) {
	now := time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC)
	limiter := NewMemoryRateLimiter()

	router := echo.New()
	router.Use(limiter.Middleware(RateLimitConfig{
		MaxRequests: 1,
		Window:      time.Minute,
		KeyPrefix:   "test",
		Now: func() time.Time {
			return now
		},
	}))
	router.GET("/api/auth/login", func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	})

	first := serveRateLimitTestRequest(router, "203.0.113.10")
	if first.Code != http.StatusOK {
		t.Fatalf("expected first IP to pass, got %d", first.Code)
	}
	secondSameIP := serveRateLimitTestRequest(router, "203.0.113.10")
	if secondSameIP.Code != http.StatusTooManyRequests {
		t.Fatalf("expected same IP to be limited, got %d", secondSameIP.Code)
	}
	otherIP := serveRateLimitTestRequest(router, "203.0.113.11")
	if otherIP.Code != http.StatusOK {
		t.Fatalf("expected different IP to pass, got %d: %s", otherIP.Code, otherIP.Body.String())
	}
}

func serveRateLimitTestRequest(router *echo.Echo, ip string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/auth/login", nil)
	req.Header.Set("X-Forwarded-For", ip)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

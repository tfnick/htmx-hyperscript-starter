package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	httpresponse "github.com/tfnick/go-svelte-starter/api/framework/http/response"
)

type RateLimitConfig struct {
	MaxRequests int
	Window      time.Duration
	KeyPrefix   string
	KeyFunc     func(echo.Context) string
	Now         func() time.Time
}

type rateLimitBucket struct {
	count       int
	windowStart time.Time
	lastSeen    time.Time
}

type MemoryRateLimiter struct {
	mu      sync.Mutex
	buckets map[string]rateLimitBucket
}

func NewMemoryRateLimiter() *MemoryRateLimiter {
	return &MemoryRateLimiter{
		buckets: make(map[string]rateLimitBucket),
	}
}

func MemoryRateLimit(config RateLimitConfig) echo.MiddlewareFunc {
	return NewMemoryRateLimiter().Middleware(config)
}

func (l *MemoryRateLimiter) Middleware(config RateLimitConfig) echo.MiddlewareFunc {
	if l == nil {
		l = NewMemoryRateLimiter()
	}
	if config.MaxRequests <= 0 {
		config.MaxRequests = 10
	}
	if config.Window <= 0 {
		config.Window = time.Minute
	}
	if config.Now == nil {
		config.Now = time.Now
	}
	if config.KeyFunc == nil {
		config.KeyFunc = ClientIPRateLimitKey
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			rawKey := strings.TrimSpace(config.KeyFunc(c))
			if rawKey == "" {
				rawKey = "unknown"
			}
			key := hashRateLimitKey(config.KeyPrefix + ":" + rawKey)
			if !l.allow(key, config.MaxRequests, config.Window, config.Now()) {
				return httpresponse.ErrorWithCode(c, http.StatusTooManyRequests, "rate_limited", "too many requests, please try again later")
			}
			return next(c)
		}
	}
}

func ClientIPRateLimitKey(c echo.Context) string {
	req := c.Request()
	forwardedFor := strings.TrimSpace(req.Header.Get("X-Forwarded-For"))
	if forwardedFor != "" {
		first, _, _ := strings.Cut(forwardedFor, ",")
		if ip := strings.TrimSpace(first); ip != "" {
			return ip
		}
	}
	if forwarded := strings.TrimSpace(req.Header.Get("X-Real-IP")); forwarded != "" {
		return forwarded
	}
	host, _, err := net.SplitHostPort(req.RemoteAddr)
	if err == nil && strings.TrimSpace(host) != "" {
		return strings.TrimSpace(host)
	}
	return strings.TrimSpace(req.RemoteAddr)
}

func (l *MemoryRateLimiter) allow(key string, maxRequests int, window time.Duration, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.prune(now, window)

	bucket := l.buckets[key]
	if bucket.windowStart.IsZero() || now.Sub(bucket.windowStart) >= window {
		l.buckets[key] = rateLimitBucket{
			count:       1,
			windowStart: now,
			lastSeen:    now,
		}
		return true
	}

	bucket.count++
	bucket.lastSeen = now
	l.buckets[key] = bucket
	return bucket.count <= maxRequests
}

func (l *MemoryRateLimiter) prune(now time.Time, window time.Duration) {
	for key, bucket := range l.buckets {
		if now.Sub(bucket.lastSeen) > 2*window {
			delete(l.buckets, key)
		}
	}
}

func hashRateLimitKey(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

package middleware

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/tfnick/go-svelte-starter/api/framework/cache"
	"github.com/tfnick/go-svelte-starter/api/framework/data/modelerror"
	"github.com/tfnick/go-svelte-starter/api/models"
)

const pageViewCookieName = "pv_sid"

func PageViewTracker() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if !isPageViewPath(c.Request().URL.Path) {
				return next(c)
			}

			// Check if page view tracking is enabled
			enabled, err := pageViewEnabled(c)
			if err != nil || !enabled {
				return next(c)
			}
			// sqlite wal model do not use goroutine
			recordPageView(c)

			return next(c)
		}
	}
}

func isPageViewPath(path string) bool {
	if path == "/" || strings.HasPrefix(path, "/app") || strings.HasPrefix(path, "/pricing") || strings.HasPrefix(path, "/features") {
		return true
	}
	return false
}

func pageViewEnabled(c echo.Context) (bool, error) {
	return cache.Cached(c.Request().Context(), "app_setting", "site.page_view_enabled", 365*24*time.Hour, func() (bool, error) {
		setting, err := models.GetAppSetting(c.Request().Context(), "site.page_view_enabled")
		if err != nil {
			if errors.Is(err, modelerror.ErrNotFound) {
				return true, nil // default: enabled
			}
			return true, err
		}
		var enabled bool
		if err := json.Unmarshal([]byte(setting.ValueJSON), &enabled); err != nil {
			return true, nil
		}
		return enabled, nil
	})
}

func recordPageView(c echo.Context) {
	pv := models.PageView{}

	user := GetCurrentUser(c)
	if user != nil {
		pv.UserID = user.ID
	}

	pv.Path = c.Request().URL.Path
	pv.Referrer = c.Request().Referer()
	pv.Country = c.Request().Header.Get("X-Geo-Country")
	pv.Region = c.Request().Header.Get("X-Geo-Region")

	// Session cookie
	sid := sessionFromCookie(c)
	if sid == "" {
		sid = uuid.Must(uuid.NewV7()).String()
		c.SetCookie(&http.Cookie{
			Name:     pageViewCookieName,
			Value:    sid,
			Path:     "/",
			MaxAge:   86400 * 30,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		})
	}
	pv.SessionID = sid

	// UTM from cookies
	pv.UtmSource = cookieValue(c, "utm_source")
	pv.UtmMedium = cookieValue(c, "utm_medium")
	pv.UtmCampaign = cookieValue(c, "utm_campaign")

	_ = models.InsertPageView(c.Request().Context(), &pv)
}

func sessionFromCookie(c echo.Context) string {
	cookie, err := c.Cookie(pageViewCookieName)
	if err != nil || cookie.Value == "" {
		return ""
	}
	return strings.TrimSpace(cookie.Value)
}

func cookieValue(c echo.Context, name string) string {
	cookie, err := c.Cookie(name)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(cookie.Value)
}

package middleware

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/tfnick/go-svelte-starter/api/framework/authz"
	fwauth "github.com/tfnick/go-svelte-starter/api/framework/http/auth"
	fwcontext "github.com/tfnick/go-svelte-starter/api/framework/http/context"
	httpresponse "github.com/tfnick/go-svelte-starter/api/framework/http/response"
	"github.com/tfnick/go-svelte-starter/api/models"
)

const UserContextKey = fwcontext.UserContextKey

const AccessTokenQueryParam = "access_token"

type AuthConfig struct {
	SkipPaths       []string
	AllowQueryToken bool
}

var DefaultAuthConfig = AuthConfig{
	SkipPaths: []string{
		"/api/auth/login",
		"/api/auth/register",
		"/api/auth/forgot-password",
		"/api/auth/reset-password",
	},
}

func RequireAuth() echo.MiddlewareFunc {
	return RequireAuthWithConfig(DefaultAuthConfig)
}

func RequireAuthWithConfig(config AuthConfig) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			path := c.Path()
			for _, skipPath := range config.SkipPaths {
				if strings.HasPrefix(path, skipPath) {
					return next(c)
				}
			}

			token := tokenFromRequest(c, config.AllowQueryToken)
			if token == "" {
				return httpresponse.Unauthorized(c, "not logged in")
			}

			claims, err := fwauth.ParseUserToken(token)
			if err != nil {
				return httpresponse.ErrorWithCode(c, http.StatusUnauthorized, "unauthorized", "login token is invalid or expired")
			}

			user, err := models.GetUserByID(c.Request().Context(), claims.Subject)
			if err != nil {
				return httpresponse.Unauthorized(c, "user does not exist")
			}

			if user.IsActive == 0 {
				return httpresponse.Forbidden(c, "account is disabled")
			}

			fwcontext.SetCurrentUser(c, user)
			return next(c)
		}
	}
}

func GetCurrentUser(c echo.Context) *models.User {
	return fwcontext.GetCurrentUser(c)
}

func RequireAdmin() echo.MiddlewareFunc {
	return RequireRole(authz.RolePlatformAdmin)
}

func RequireRole(roles ...string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			user := GetCurrentUser(c)
			if user == nil {
				return httpresponse.Unauthorized(c, "not logged in")
			}
			role := authz.NormalizeRole(user.Role, user.IsAdmin == 1)
			if !authz.HasRole(role, roles...) {
				return httpresponse.Forbidden(c, "admin access is required")
			}
			return next(c)
		}
	}
}

func RequirePermission(permission string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			user := GetCurrentUser(c)
			if user == nil {
				return httpresponse.Unauthorized(c, "not logged in")
			}
			role := authz.NormalizeRole(user.Role, user.IsAdmin == 1)
			if !authz.HasPermission(authz.PermissionsForRole(role), permission) {
				return httpresponse.Forbidden(c, "permission is required")
			}
			return next(c)
		}
	}
}

func OptionalAuth() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			token := tokenFromRequest(c, false)
			if token == "" {
				return next(c)
			}

			claims, err := fwauth.ParseUserToken(token)
			if err != nil {
				return next(c)
			}

			user, err := models.GetUserByID(c.Request().Context(), claims.Subject)
			if err == nil && user.IsActive == 1 {
				fwcontext.SetCurrentUser(c, user)
			}

			return next(c)
		}
	}
}

func tokenFromRequest(c echo.Context, allowQueryToken bool) string {
	authHeader := strings.TrimSpace(c.Request().Header.Get(echo.HeaderAuthorization))
	if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
		return strings.TrimSpace(authHeader[len("Bearer "):])
	}

	if !allowQueryToken {
		return ""
	}
	return strings.TrimSpace(c.QueryParam(AccessTokenQueryParam))
}

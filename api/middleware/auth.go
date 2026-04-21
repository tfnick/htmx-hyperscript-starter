package middleware

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/zachatrocity/htmx-hyperscript-starter/api/models"
)

// Context key for storing user
const UserContextKey = "user"

// Session cookie name
const SessionCookieName = "session_id"

// AuthConfig 认证中间件配置
type AuthConfig struct {
	// SkipPaths 不需要认证的路径前缀
	SkipPaths []string
}

// DefaultAuthConfig 默认配置
var DefaultAuthConfig = AuthConfig{
	SkipPaths: []string{
		"/api/auth/login",
		"/api/auth/register",
		"/api/auth/forgot-password",
		"/api/auth/reset-password",
		"/api/components",
	},
}

// RequireAuth 认证中间件
func RequireAuth() echo.MiddlewareFunc {
	return RequireAuthWithConfig(DefaultAuthConfig)
}

// RequireAuthWithConfig 带配置的认证中间件
func RequireAuthWithConfig(config AuthConfig) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// 检查是否跳过认证
			path := c.Path()
			for _, skipPath := range config.SkipPaths {
				if strings.HasPrefix(path, skipPath) {
					return next(c)
				}
			}

			// 从 Cookie 获取 Session ID
			cookie, err := c.Cookie(SessionCookieName)
			if err != nil || cookie.Value == "" {
				return c.JSON(http.StatusUnauthorized, map[string]string{
					"error": "未登录",
				})
			}

			sessionID := cookie.Value

			// 验证 Session
			session, err := models.GetSessionByID(sessionID)
			if err != nil {
				// 清除无效 Cookie
				clearSessionCookie(c)
				return c.JSON(http.StatusUnauthorized, map[string]string{
					"error": "会话已过期，请重新登录",
				})
			}

			// 获取用户信息
			user, err := models.GetUserByID(session.UserID)
			if err != nil {
				clearSessionCookie(c)
				return c.JSON(http.StatusUnauthorized, map[string]string{
					"error": "用户不存在",
				})
			}

			// 检查用户状态
			if user.IsActive == 0 {
				clearSessionCookie(c)
				return c.JSON(http.StatusForbidden, map[string]string{
					"error": "账户已被禁用",
				})
			}

			// 将用户信息存入上下文
			c.Set(UserContextKey, user)

			return next(c)
		}
	}
}

// GetCurrentUser 从上下文获取当前用户
func GetCurrentUser(c echo.Context) *models.User {
	user, ok := c.Get(UserContextKey).(*models.User)
	if !ok {
		return nil
	}
	return user
}

// SetSessionCookie 设置 Session Cookie
func SetSessionCookie(c echo.Context, sessionID string, maxAge int) {
	c.SetCookie(&http.Cookie{
		Name:     SessionCookieName,
		Value:    sessionID,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   false, // 开发环境设为 false，生产环境应设为 true
		SameSite: http.SameSiteLaxMode,
	})
}

// clearSessionCookie 清除 Session Cookie
func clearSessionCookie(c echo.Context) {
	c.SetCookie(&http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})
}

// OptionalAuth 可选认证中间件（不强制登录，但会尝试获取用户信息）
func OptionalAuth() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			cookie, err := c.Cookie(SessionCookieName)
			if err != nil || cookie.Value == "" {
				return next(c)
			}

			session, err := models.GetSessionByID(cookie.Value)
			if err != nil {
				return next(c)
			}

			user, err := models.GetUserByID(session.UserID)
			if err == nil && user.IsActive == 1 {
				c.Set(UserContextKey, user)
			}

			return next(c)
		}
	}
}

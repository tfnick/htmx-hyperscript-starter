package routes

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/zachatrocity/htmx-hyperscript-starter/api/middleware"
	"github.com/zachatrocity/htmx-hyperscript-starter/api/models"
)

// RegisterRequest 注册请求
type RegisterRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginRequest 登录请求
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// ForgotPasswordRequest 忘记密码请求
type ForgotPasswordRequest struct {
	Email string `json:"email"`
}

// ResetPasswordRequest 重置密码请求
type ResetPasswordRequest struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

// Register 用户注册
func Register(c echo.Context) error {
	// 支持JSON和表单两种提交方式
	var req RegisterRequest
	contentType := c.Request().Header.Get("Content-Type")

	if contentType == "application/json" {
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": "无效的请求数据",
			})
		}
	} else {
		// 表单提交
		req.Name = c.FormValue("name")
		req.Email = c.FormValue("email")
		req.Password = c.FormValue("password")
	}

	// 验证必填字段
	if req.Name == "" || req.Email == "" || req.Password == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "姓名、邮箱和密码不能为空",
		})
	}

	// 验证密码长度
	if len(req.Password) < 6 {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "密码长度至少6位",
		})
	}

	// 检查邮箱是否已注册
	exists, err := models.UserExistsByEmail(req.Email)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "检查邮箱失败",
		})
	}
	if exists {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "该邮箱已注册",
		})
	}

	// 创建用户
	user := &models.User{
		Name:  req.Name,
		Email: req.Email,
	}
	if err := user.SetPassword(req.Password); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "密码处理失败",
		})
	}

	if err := models.CreateUser(user); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
	}

	// 创建会话
	session, err := models.CreateSession(user.ID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "创建会话失败",
		})
	}

	// 设置 Cookie（7天有效期）
	middleware.SetSessionCookie(c, session.ID, 7*24*3600)

	// HTMX 重定向到首页
	c.Response().Header().Set("HX-Redirect", "/")
	return c.HTML(http.StatusOK, "")
}

// Login 用户登录
func Login(c echo.Context) error {
	// 支持JSON和表单两种提交方式
	var req LoginRequest
	contentType := c.Request().Header.Get("Content-Type")

	if contentType == "application/json" {
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": "无效的请求数据",
			})
		}
	} else {
		// 表单提交
		req.Email = c.FormValue("email")
		req.Password = c.FormValue("password")
	}

	if req.Email == "" || req.Password == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "邮箱和密码不能为空",
		})
	}

	// 获取用户
	user, err := models.GetUserWithPasswordByEmail(req.Email)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "登录失败",
		})
	}
	if user == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{
			"error": "邮箱或密码错误",
		})
	}

	// 验证密码
	if !user.CheckPassword(req.Password) {
		return c.JSON(http.StatusUnauthorized, map[string]string{
			"error": "邮箱或密码错误",
		})
	}

	// 检查用户状态
	if user.IsActive == 0 {
		return c.JSON(http.StatusForbidden, map[string]string{
			"error": "账户已被禁用",
		})
	}

	// 创建会话
	session, err := models.CreateSession(user.ID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "创建会话失败",
		})
	}

	// 设置 Cookie
	middleware.SetSessionCookie(c, session.ID, 7*24*3600)

	// HTMX 重定向到首页
	c.Response().Header().Set("HX-Redirect", "/")
	return c.HTML(http.StatusOK, "")
}

// Logout 用户登出
func Logout(c echo.Context) error {
	cookie, err := c.Cookie(middleware.SessionCookieName)
	if err == nil && cookie.Value != "" {
		// 删除会话
		models.DeleteSession(cookie.Value)
	}

	// 清除 Cookie
	middleware.SetSessionCookie(c, "", -1)

	return c.JSON(http.StatusOK, map[string]string{
		"message": "已登出",
	})
}

// GetCurrentUser 获取当前登录用户
func GetCurrentUser(c echo.Context) error {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{
			"error": "未登录",
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"user": map[string]interface{}{
			"id":            user.ID,
			"name":          user.Name,
			"email":         user.Email,
			"email_verified": user.EmailVerified == 1,
		},
	})
}

// ForgotPassword 忘记密码
func ForgotPassword(c echo.Context) error {
	// 支持JSON和表单两种提交方式
	var req ForgotPasswordRequest
	contentType := c.Request().Header.Get("Content-Type")

	if contentType == "application/json" {
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": "无效的请求数据",
			})
		}
	} else {
		req.Email = c.FormValue("email")
	}

	if req.Email == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "邮箱不能为空",
		})
	}

	// 检查用户是否存在
	user, err := models.GetUserWithPasswordByEmail(req.Email)
	if err != nil || user == nil {
		// 为了安全，不暴露用户是否存在
		return c.JSON(http.StatusOK, map[string]string{
			"message": "如果该邮箱已注册，重置链接已发送",
		})
	}

	// 创建重置 Token
	token, err := models.CreatePasswordReset(user.ID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "创建重置链接失败",
		})
	}

	// 开发环境：打印重置链接到控制台
	resetURL := fmt.Sprintf("http://localhost:3000/reset-password?token=%s", token)
	fmt.Printf("\n========================================\n")
	fmt.Printf("📧 密码重置链接（开发模式）\n")
	fmt.Printf("用户: %s (%s)\n", user.Name, user.Email)
	fmt.Printf("链接: %s\n", resetURL)
	fmt.Printf("========================================\n\n")

	return c.JSON(http.StatusOK, map[string]string{
		"message": "如果该邮箱已注册，重置链接已发送",
	})
}

// ResetPassword 重置密码
func ResetPassword(c echo.Context) error {
	// 支持JSON和表单两种提交方式
	var req ResetPasswordRequest
	contentType := c.Request().Header.Get("Content-Type")

	if contentType == "application/json" {
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": "无效的请求数据",
			})
		}
	} else {
		req.Token = c.FormValue("token")
		req.Password = c.FormValue("password")
	}

	if req.Token == "" || req.Password == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Token 和密码不能为空",
		})
	}

	if len(req.Password) < 6 {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "密码长度至少6位",
		})
	}

	// 验证 Token
	reset, err := models.VerifyPasswordResetToken(req.Token)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "重置链接无效或已过期",
		})
	}

	// 更新密码
	if err := models.UpdateUserPassword(reset.UserID, req.Password); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "更新密码失败",
		})
	}

	// 标记 Token 已使用
	models.MarkPasswordResetUsed(reset.ID)

	// 删除用户所有会话（强制重新登录）
	models.DeleteUserSessions(reset.UserID)

	return c.JSON(http.StatusOK, map[string]string{
		"message": "密码重置成功，请重新登录",
	})
}

// GetAuthStatus 获取认证状态（公开API，不强制登录）
func GetAuthStatus(c echo.Context) error {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"logged_in": false,
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"logged_in": true,
		"user": map[string]interface{}{
			"id":   user.ID,
			"name": user.Name,
		},
	})
}

// GetAuthStatusComponent 获取认证状态组件（返回HTML）
func GetAuthStatusComponent(c echo.Context) error {
	user := middleware.GetCurrentUser(c)

	html := `<div class="header-auth-inner">`
	if user != nil {
		html += `
			<span class="user-name">你好, ` + user.Name + `</span>
			<button class="btn-logout secondary" hx-post="/api/auth/logout" hx-swap="none" _="on htmx:afterRequest call window.location.reload()">登出</button>
		`
	} else {
		html += `<a href="/login.html" role="button" class="secondary">登录</a>`
	}
	html += `</div>`

	return c.HTML(http.StatusOK, html)
}

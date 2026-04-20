package routes

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/zachatrocity/htmx-hyperscript-starter/api/models"
)

// GetUserMock 模拟获取用户（旧方法，用于演示）
func GetUserMock(c echo.Context) error {
	// simulate network request
	time.Sleep(3 * time.Second)
	u := &models.User{
		ID:    c.Param("id"),
		Name:  "Zach",
		Email: "email@email.com",
	}
	return c.JSON(http.StatusOK, u)
}

// CreateUser 创建用户处理函数
func CreateUser(c echo.Context) error {
	var user models.User
	if err := c.Bind(&user); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "无效的请求数据",
		})
	}

	if user.Name == "" || user.Email == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "姓名和邮箱不能为空",
		})
	}

	if err := models.CreateUser(&user); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
	}

	return c.JSON(http.StatusCreated, user)
}

// GetAllUsers 获取所有用户
func GetAllUsers(c echo.Context) error {
	users, err := models.GetAllUsers()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
	}
	return c.JSON(http.StatusOK, users)
}

// GetUser 根据 ID 获取单个用户
func GetUser(c echo.Context) error {
	// 支持路径参数 /api/users/:id 和查询参数 /api/users?id=xxx
	id := c.Param("id")
	if id == "" {
		id = c.QueryParam("id")
	}
	if id == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "缺少用户 ID",
		})
	}
	user, err := models.GetUserByID(id)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "用户不存在",
		})
	}
	return c.JSON(http.StatusOK, user)
}

// UpdateUser 更新用户
func UpdateUser(c echo.Context) error {
	id := c.Param("id")
	var user models.User
	if err := c.Bind(&user); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "无效的请求数据",
		})
	}
	user.ID = id

	if err := models.UpdateUser(&user); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
	}

	return c.JSON(http.StatusOK, user)
}

// DeleteUser 删除用户
func DeleteUser(c echo.Context) error {
	id := c.Param("id")
	if err := models.DeleteUser(id); err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": err.Error(),
		})
	}
	return c.NoContent(http.StatusNoContent)
}

package main

import (
	"flag"
	"fmt"
	"net/http"
	"strings"

	"github.com/aarol/reload"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	user "github.com/zachatrocity/htmx-hyperscript-starter/api/routes"
	"github.com/zachatrocity/htmx-hyperscript-starter/api/db"
	authMiddleware "github.com/zachatrocity/htmx-hyperscript-starter/api/middleware"
)

// 程序启动以及路由注册
func main() {
	isDevelopment := flag.Bool("dev", true, "Development mode")
	port := flag.String("port", "3000", "Port to serve the app")
	dbPath := flag.String("db", "data/app.db", "SQLite database path")
	flag.Parse()

	router := echo.New()

	// ========== 数据库初始化 ==========
	// 确保数据目录存在
	if err := db.EnsureDataDir(); err != nil {
		router.Logger.Fatal("创建数据目录失败: ", err)
	}

	// 初始化数据库连接
	if err := db.InitDB(*dbPath); err != nil {
		router.Logger.Fatal("数据库初始化失败: ", err)
	}
	defer db.CloseDB()

	// 执行数据库迁移
	if err := db.AutoMigrate(); err != nil {
		router.Logger.Fatal("数据库迁移失败: ", err)
	}
	// ========== 数据库初始化结束 ==========

	// Add Middlewares Here
	// e.Use(middleware.Logger())
	router.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"http://localhost:3000", "http://localhost:4000"},
		AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept},
	}))

	// server the public folder as static
	router.Static("/", "public/")

	api := router.Group("/api")
	{
		// htmx components
		api.GET("/components/*", func(c echo.Context) error {
			if *isDevelopment {
				fmt.Println("Component Requested: " + c.Request().URL.Path)
			}
			component := strings.ReplaceAll(c.Request().URL.Path, "/api/components/", "")
			// yet the cache for dev
			c.Response().Header().Set("Cache-Control", "no-store")
			return c.File("public/components/" + component + ".html")
		})

		// 认证 API（无需登录）
		api.POST("/auth/register", user.Register)
		api.POST("/auth/login", user.Login)
		api.POST("/auth/forgot-password", user.ForgotPassword)
		api.POST("/auth/reset-password", user.ResetPassword)

		// 认证状态（可选登录，用于前端显示）
		api.GET("/auth/status", user.GetAuthStatus, authMiddleware.OptionalAuth())
		api.GET("/auth/status-component", user.GetAuthStatusComponent, authMiddleware.OptionalAuth())

		// 需要认证的 API
		protected := api.Group("")
		protected.Use(authMiddleware.RequireAuth())
		{
			// 认证相关（需要登录）
			protected.POST("/auth/logout", user.Logout)
			protected.GET("/auth/me", user.GetCurrentUser)

			// 用户 API
			protected.GET("/users", user.GetAllUsers)
			protected.GET("/users/:id", user.GetUser)
			protected.POST("/users", user.CreateUser)
			protected.PUT("/users/:id", user.UpdateUser)
			protected.DELETE("/users/:id", user.DeleteUser)

			// 订单 API
			protected.POST("/orders", user.CreateOrder)
			protected.GET("/orders/user/:user_id", user.GetUserOrders)
			protected.GET("/orders/:id", user.GetOrderDetail)
			protected.PATCH("/orders/:id/status", user.UpdateOrderStatus)
		}
	}

	// hot reload from aarol/reload
	if *isDevelopment {
		// Watch for HTML changes in the public folder to trigger browser reload
		reload := reload.New("public/")

		// reload.OnReload = func() {
		// build templates if that's your thing
		// }
		router.GET("/reload_ws", echo.WrapHandler(reload.Handle(http.DefaultServeMux)))

		fmt.Println("Hot Reload Enabled...")
	}

	fmt.Printf("Listening on port %s\n", *port)
	router.Logger.Fatal(router.Start(":" + *port))
}

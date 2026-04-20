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
)

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

		// 用户 API
		api.GET("/users", user.GetAllUsers)
		api.GET("/users/:id", user.GetUser)
		api.POST("/users", user.CreateUser)
		api.PUT("/users/:id", user.UpdateUser)
		api.DELETE("/users/:id", user.DeleteUser)

		// 订单 API
		api.POST("/orders", user.CreateOrder)
		api.GET("/orders/user/:user_id", user.GetUserOrders)
		api.GET("/orders/:id", user.GetOrderDetail)
		api.PATCH("/orders/:id/status", user.UpdateOrderStatus)
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

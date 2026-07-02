package main

import (
	"bytes"
	"embed"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/aarol/reload"
	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	"github.com/tfnick/go-svelte-starter/api/db"
	appmiddleware "github.com/tfnick/go-svelte-starter/api/framework/http/middleware"
	"github.com/tfnick/go-svelte-starter/api/framework/logging"
	"github.com/tfnick/go-svelte-starter/api/routes"
)

//go:embed public
var embeddedPublic embed.FS

func main() {
	isDevelopment := flag.Bool("dev", true, "Development mode")
	port := flag.String("port", "3000", "Port to serve the app")
	templatePath := flag.String("template-path", "", "External HTML template path. Files here override embedded templates.")
	flag.Parse()

	if err := logging.Init(*isDevelopment); err != nil {
		panic(err)
	}
	defer logging.Close()

	manager, err := initDatabases()
	if err != nil {
		panic(err)
	}
	defer manager.Close()

	router := echo.New()
	publicFS := echo.MustSubFS(embeddedPublic, "public")

	router.Use(echomiddleware.CORSWithConfig(echomiddleware.CORSConfig{
		AllowOrigins: []string{"http://localhost:3000", "http://localhost:4000"},
		AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization},
		AllowMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions},
	}))

	registerFrontendRoutes(router, publicFS, *templatePath)
	router.GET("/styles.css", streamEmbeddedFile(publicFS, "styles.css", "text/css; charset=utf-8"))
	router.GET("/extensions.js", streamEmbeddedFile(publicFS, "extensions.js", "application/javascript; charset=utf-8"))
	router.StaticFS("/assets", echo.MustSubFS(publicFS, "assets"))

	api := router.Group("/api")
	api.Use(appmiddleware.RequestLogger(string("api")))
	registerAPIRoutes(api)
	registerComponentRoutes(api, publicFS, *templatePath, *isDevelopment)

	if *isDevelopment {
		watchPaths := []string{"public/"}
		if strings.TrimSpace(*templatePath) != "" {
			watchPaths = append(watchPaths, *templatePath)
		}
		reloader := reload.New(watchPaths...)
		router.GET("/reload_ws", echo.WrapHandler(reloader.Handle(http.DefaultServeMux)))
		fmt.Println("Hot Reload Enabled...")
	}

	fmt.Printf("Listening on port %s\n", *port)
	router.Logger.Fatal(router.Start(":" + *port))
}

func registerFrontendRoutes(router *echo.Echo, publicFS fs.FS, templatePath string) {
	indexHandler := renderTemplate(publicFS, templatePath, "index.html", nil)
	router.GET("/", indexHandler)
	router.GET("/categories/:slug", indexHandler)
	router.GET("/post-*", indexHandler)
}

func initDatabases() (*db.DBManager, error) {
	if err := db.EnsureDataDir(); err != nil {
		return nil, err
	}

	runtimeDBs, err := db.LoadRuntimeDatabases(db.RuntimeConfigInput{})
	if err != nil {
		return nil, err
	}

	manager := db.NewDBManager()
	db.DefaultManager = manager
	if err := manager.OpenSpec(runtimeDBs.App); err != nil {
		return nil, err
	}
	if err := manager.AutoMigrate("app"); err != nil {
		return nil, err
	}
	if err := manager.OpenSpec(runtimeDBs.Shared); err != nil {
		return nil, err
	}
	if err := manager.AutoMigrate("shared"); err != nil {
		return nil, err
	}
	return manager, nil
}

func registerAPIRoutes(api *echo.Group) {
	api.POST("/auth/register", routes.Register)
	api.POST("/auth/login", routes.Login)
	api.POST("/auth/refresh", routes.RefreshToken)
	api.POST("/auth/logout", routes.Logout)
	api.POST("/auth/forgot-password", routes.ForgotPassword)
	api.POST("/auth/reset-password", routes.ResetPassword)
	api.GET("/auth/status", routes.GetAuthStatus, appmiddleware.OptionalAuth())
	api.GET("/auth/me", routes.GetCurrentUser, appmiddleware.RequireAuth())
	api.GET("/auth/oauth/:provider/start", routes.StartOAuthLogin)
	api.GET("/auth/oauth/:provider/callback", routes.CompleteOAuthLogin)
	api.POST("/auth/oauth/exchange", routes.ExchangeOAuthLoginResult)

	routes.RegisterForumRoutes(api)

	api.GET("/notifications", routes.ListNotifications, appmiddleware.RequireAuth())
	api.DELETE("/notifications", routes.ClearMyNotifications, appmiddleware.RequireAuth())
	api.GET("/user/points", routes.GetMyPoints, appmiddleware.RequireAuth())
	api.GET("/user/realtime/ws", routes.UserRealtimeWebSocket, appmiddleware.RequireAuthWithConfig(appmiddleware.AuthConfig{AllowQueryToken: true}))
}

func registerComponentRoutes(api *echo.Group, publicFS fs.FS, externalRoot string, isDevelopment bool) {
	api.GET("/components/*", func(c echo.Context) error {
		if isDevelopment {
			fmt.Println("Component Requested: " + c.Request().URL.Path)
		}
		component := strings.TrimPrefix(c.Request().URL.Path, "/api/components/")
		c.Response().Header().Set("Cache-Control", "no-store")
		return renderTemplate(publicFS, externalRoot, "components/"+component+".html", nil)(c)
	})
}

func renderTemplate(files fs.FS, externalRoot string, name string, data any) echo.HandlerFunc {
	return func(c echo.Context) error {
		body, err := loadTemplate(files, externalRoot, name)
		if err != nil {
			return err
		}
		tpl, err := template.New(path.Base(name)).Parse(string(body))
		if err != nil {
			return err
		}

		var out bytes.Buffer
		if err := tpl.Execute(&out, data); err != nil {
			return err
		}
		return c.HTML(http.StatusOK, out.String())
	}
}

func loadTemplate(files fs.FS, externalRoot string, name string) ([]byte, error) {
	clean, err := cleanTemplateName(name)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "invalid template path")
	}
	if strings.TrimSpace(externalRoot) != "" {
		externalPath := filepath.Join(externalRoot, filepath.FromSlash(clean))
		if body, err := os.ReadFile(externalPath); err == nil {
			return body, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	}
	body, err := fs.ReadFile(files, clean)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, echo.NewHTTPError(http.StatusNotFound, "template not found")
		}
		return nil, err
	}
	return body, nil
}

func cleanTemplateName(name string) (string, error) {
	name = strings.TrimSpace(strings.ReplaceAll(name, "\\", "/"))
	if name == "" || strings.HasPrefix(name, "/") || !strings.HasSuffix(name, ".html") {
		return "", fmt.Errorf("invalid template path")
	}
	clean := path.Clean(name)
	if clean == "." || strings.HasPrefix(clean, "../") || clean == ".." || clean != name {
		return "", fmt.Errorf("invalid template path")
	}
	return clean, nil
}

func streamEmbeddedFile(files fs.FS, name, contentType string) echo.HandlerFunc {
	return func(c echo.Context) error {
		file, err := files.Open(name)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return echo.NewHTTPError(http.StatusNotFound, "asset not found")
			}
			return err
		}
		defer file.Close()

		return c.Stream(http.StatusOK, contentType, file)
	}
}

package main

import (
	"bytes"
	"embed"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"net/http"
	"strings"

	"github.com/aarol/reload"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/zachatrocity/htmx-hyperscript-starter/api/forum"
	user "github.com/zachatrocity/htmx-hyperscript-starter/api/routes"
	"github.com/zachatrocity/htmx-hyperscript-starter/api/templates"
)

//go:embed public
var embeddedPublic embed.FS

func main() {
	isDevelopment := flag.Bool("dev", true, "Development mode")
	port := flag.String("port", "3000", "Port to serve the app")
	templatePath := flag.String("template-path", "", "External HTML template path. Files here override embedded templates.")
	flag.Parse()

	router := echo.New()
	publicFS := echo.MustSubFS(embeddedPublic, "public")
	templateResolver := templates.NewResolver(publicFS, *templatePath)

	// Add Middlewares Here
	// e.Use(middleware.Logger())
	router.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"http://localhost:3000", "http://localhost:4000"},
		AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept},
	}))

	router.HTTPErrorHandler = func(err error, c echo.Context) {
		if errors.Is(err, templates.ErrInvalidTemplatePath) {
			if handleErr := c.String(http.StatusBadRequest, "Invalid template path"); handleErr != nil {
				c.Logger().Error(handleErr)
			}
			return
		}
		if errors.Is(err, templates.ErrTemplateNotFound) {
			if handleErr := c.String(http.StatusNotFound, "Template not found"); handleErr != nil {
				c.Logger().Error(handleErr)
			}
			return
		}
		router.DefaultHTTPErrorHandler(err, c)
	}

	router.GET("/", renderTemplate(templateResolver, "index.html", nil))

	// Serve bundled static assets from the executable. HTML is rendered through
	// the resolver so external template overrides have a single path.
	router.GET("/styles.css", streamEmbeddedFile(publicFS, "styles.css", "text/css; charset=utf-8"))
	router.GET("/extensions.js", streamEmbeddedFile(publicFS, "extensions.js", "application/javascript; charset=utf-8"))
	router.StaticFS("/assets", echo.MustSubFS(publicFS, "assets"))

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
			return renderTemplate(templateResolver, "components/"+component+".html", nil)(c)
		})
		forum.Register(api.Group("/forum"), forum.NewStore(), templateResolver)
		// all other API requests
		api.GET("/users/:id", user.GetUser)
	}

	// hot reload from aarol/reload
	if *isDevelopment {
		// Watch for HTML changes in the public folder to trigger browser reload
		watchPaths := []string{"public/"}
		if *templatePath != "" {
			watchPaths = append(watchPaths, *templatePath)
		}
		reload := reload.New(watchPaths...)

		// reload.OnReload = func() {
		// build templates if that's your thing
		// }
		router.GET("/reload_ws", echo.WrapHandler(reload.Handle(http.DefaultServeMux)))

		fmt.Println("Hot Reload Enabled...")
	}

	fmt.Printf("Listening on port %s\n", *port)
	router.Logger.Fatal(router.Start(":" + *port))
}

func renderTemplate(resolver *templates.Resolver, name string, data any) echo.HandlerFunc {
	return func(c echo.Context) error {
		var out bytes.Buffer
		if err := resolver.Execute(&out, name, data); err != nil {
			return err
		}

		return c.HTML(http.StatusOK, out.String())
	}
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

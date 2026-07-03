package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/labstack/echo/v4"
)

func TestLoadTemplateFallsBackToEmbeddedTemplate(t *testing.T) {
	files := fstest.MapFS{
		"index.html": {Data: []byte("embedded")},
	}

	body, err := loadTemplate(files, "", "index.html")
	if err != nil {
		t.Fatalf("loadTemplate returned error: %v", err)
	}
	if string(body) != "embedded" {
		t.Fatalf("expected embedded template, got %q", string(body))
	}
}

func TestLoadTemplatePrefersExternalTemplate(t *testing.T) {
	files := fstest.MapFS{
		"index.html": {Data: []byte("embedded")},
	}
	externalRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(externalRoot, "index.html"), []byte("external"), 0o600); err != nil {
		t.Fatalf("write external template: %v", err)
	}

	body, err := loadTemplate(files, externalRoot, "index.html")
	if err != nil {
		t.Fatalf("loadTemplate returned error: %v", err)
	}
	if string(body) != "external" {
		t.Fatalf("expected external template override, got %q", string(body))
	}
}

func TestLoadTemplateRejectsUnsafeTemplateNames(t *testing.T) {
	files := fstest.MapFS{
		"index.html": {Data: []byte("embedded")},
	}
	for _, name := range []string{"", "/index.html", "../index.html", "components/../index.html", "styles.css"} {
		t.Run(name, func(t *testing.T) {
			if _, err := loadTemplate(files, "", name); err == nil {
				t.Fatalf("expected %q to be rejected", name)
			}
		})
	}
}

func TestPostRouteRendersPostTemplate(t *testing.T) {
	files := fstest.MapFS{
		"index.html":    {Data: []byte("embedded")},
		"login.html":    {Data: []byte("login")},
		"new-post.html": {Data: []byte("new-post")},
		"post.html":     {Data: []byte("post")},
		"register.html": {Data: []byte("register")},
	}
	router := echo.New()
	registerFrontendRoutes(router, files, "")

	req := httptest.NewRequest(http.MethodGet, "/post-thread-id-1", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected post route to render post template, got status %d", rec.Code)
	}
	if rec.Body.String() != "post" {
		t.Fatalf("expected post template body, got %q", rec.Body.String())
	}
}

func TestLoginRouteRendersLoginTemplate(t *testing.T) {
	files := fstest.MapFS{
		"index.html":    {Data: []byte("embedded")},
		"login.html":    {Data: []byte("login")},
		"new-post.html": {Data: []byte("new-post")},
		"post.html":     {Data: []byte("post")},
		"register.html": {Data: []byte("register")},
	}
	router := echo.New()
	registerFrontendRoutes(router, files, "")

	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected login route to render login template, got status %d", rec.Code)
	}
	if rec.Body.String() != "login" {
		t.Fatalf("expected login template body, got %q", rec.Body.String())
	}
}

func TestRegisterRouteRendersRegisterTemplate(t *testing.T) {
	files := fstest.MapFS{
		"index.html":    {Data: []byte("embedded")},
		"login.html":    {Data: []byte("login")},
		"new-post.html": {Data: []byte("new-post")},
		"post.html":     {Data: []byte("post")},
		"register.html": {Data: []byte("register")},
	}
	router := echo.New()
	registerFrontendRoutes(router, files, "")

	req := httptest.NewRequest(http.MethodGet, "/register", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected register route to render register template, got status %d", rec.Code)
	}
	if rec.Body.String() != "register" {
		t.Fatalf("expected register template body, got %q", rec.Body.String())
	}
}

func TestNewPostRouteRendersNewPostTemplate(t *testing.T) {
	files := fstest.MapFS{
		"index.html":    {Data: []byte("embedded")},
		"login.html":    {Data: []byte("login")},
		"new-post.html": {Data: []byte("new-post")},
		"post.html":     {Data: []byte("post")},
		"register.html": {Data: []byte("register")},
	}
	router := echo.New()
	registerFrontendRoutes(router, files, "")

	req := httptest.NewRequest(http.MethodGet, "/new-post", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected new post route to render new post template, got status %d", rec.Code)
	}
	if rec.Body.String() != "new-post" {
		t.Fatalf("expected new post template body, got %q", rec.Body.String())
	}
}

func TestForumComponentRoutesRenderCurrentFragments(t *testing.T) {
	publicFS := echo.MustSubFS(embeddedPublic, "public")
	router := echo.New()
	api := router.Group("/api")
	registerComponentRoutes(api, publicFS, "", false)

	for _, path := range []string{
		"/api/components/forum/thread-list",
		"/api/components/forum/thread-detail",
	} {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("expected component route to render, got status %d body=%s", rec.Code, rec.Body.String())
			}
			if strings.Contains(rec.Body.String(), "/replies") {
				t.Fatalf("component route should not expose old reply endpoint: %s", rec.Body.String())
			}
		})
	}
}

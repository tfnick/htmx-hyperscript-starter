package routes_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	fwcontext "github.com/tfnick/go-svelte-starter/api/framework/http/context"
	"github.com/tfnick/go-svelte-starter/api/models"
	"github.com/tfnick/go-svelte-starter/api/routes"
)

func TestCreateForumThreadReturnsInternalEnvelope(t *testing.T) {
	setupRouteTestDBs(t)

	router := echo.New()
	body := bytes.NewBufferString(`{"category_slug":"daily","title":"Route thread","body":"Route-created forum content."}`)
	req := httptest.NewRequest(http.MethodPost, "/api/forum/threads", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := router.NewContext(req, rec)
	fwcontext.SetCurrentUser(c, &models.User{
		ID:       forumRouteSeedUserID,
		Name:     "Route User",
		IsActive: 1,
	})

	if err := routes.CreateForumThread(c); err != nil {
		t.Fatalf("create forum thread: %v", err)
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusCreated, rec.Code, rec.Body.String())
	}

	var envelope struct {
		Success bool                             `json:"success"`
		Data    routes.ForumThreadDetailResponse `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !envelope.Success || envelope.Data.ID == "" || envelope.Data.Category.Slug != "daily" {
		t.Fatalf("unexpected forum response: %s", rec.Body.String())
	}
	if envelope.Data.Visibility != "public" {
		t.Fatalf("expected default public visibility, got %#v", envelope.Data)
	}
}

func TestCreateForumThreadAcceptsPrivateVisibility(t *testing.T) {
	setupRouteTestDBs(t)

	router := echo.New()
	body := bytes.NewBufferString(`{"category_slug":"daily","title":"Private route thread","body":"Private route-created forum content.","visibility":"private"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/forum/threads", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := router.NewContext(req, rec)
	fwcontext.SetCurrentUser(c, &models.User{
		ID:       forumRouteSeedUserID,
		Name:     "Route User",
		IsActive: 1,
	})

	if err := routes.CreateForumThread(c); err != nil {
		t.Fatalf("create private forum thread: %v", err)
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusCreated, rec.Code, rec.Body.String())
	}

	var envelope struct {
		Success bool                             `json:"success"`
		Data    routes.ForumThreadDetailResponse `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !envelope.Success || envelope.Data.Visibility != "private" {
		t.Fatalf("unexpected private forum response: %s", rec.Body.String())
	}
}

func TestCreateForumThreadRequiresCurrentUser(t *testing.T) {
	setupRouteTestDBs(t)

	router := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/forum/threads", bytes.NewBufferString(`{"category_slug":"daily","title":"No user","body":"No user."}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := router.NewContext(req, rec)

	if err := routes.CreateForumThread(c); err != nil {
		t.Fatalf("create forum thread: %v", err)
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"message":"not logged in"`) {
		t.Fatalf("expected unauthorized envelope, got %s", rec.Body.String())
	}
}

func TestListForumThreadsSupportsCategorySearchAndSort(t *testing.T) {
	setupRouteTestDBs(t)
	seedRouteForumThread(t, "daily", "Route launch thread", "A launch thread searchable from routes.")

	router := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/forum/categories/daily/threads?q=launch&sort=latest_post", nil)
	rec := httptest.NewRecorder()
	c := router.NewContext(req, rec)
	c.SetParamNames("slug")
	c.SetParamValues("daily")

	if err := routes.ListForumThreads(c); err != nil {
		t.Fatalf("list forum threads: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var envelope struct {
		Success bool                        `json:"success"`
		Data    routes.ForumThreadsResponse `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !envelope.Success || len(envelope.Data.Items) != 1 || envelope.Data.Items[0].Title != "Route launch thread" {
		t.Fatalf("unexpected thread list response: %s", rec.Body.String())
	}
}

func TestListForumThreadsWithoutCategoryReturnsAggregateFeed(t *testing.T) {
	setupRouteTestDBs(t)
	seedRouteForumThread(t, "daily", "Daily route thread", "A daily thread for the aggregate feed.")
	seedRouteForumThread(t, "tech", "Tech route thread", "A tech thread for the aggregate feed.")

	router := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/forum/threads?sort=latest_post", nil)
	rec := httptest.NewRecorder()
	c := router.NewContext(req, rec)

	if err := routes.ListForumThreads(c); err != nil {
		t.Fatalf("list aggregate forum threads: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var envelope struct {
		Success bool                        `json:"success"`
		Data    routes.ForumThreadsResponse `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !envelope.Success || len(envelope.Data.Items) != 2 {
		t.Fatalf("expected aggregate thread list, got %s", rec.Body.String())
	}

	slugs := map[string]bool{}
	for _, item := range envelope.Data.Items {
		slugs[item.Category.Slug] = true
	}
	if !slugs["daily"] || !slugs["tech"] {
		t.Fatalf("expected daily and tech threads in aggregate feed, got %#v", slugs)
	}
}

func TestPrivateForumThreadRouteVisibility(t *testing.T) {
	setupRouteTestDBs(t)
	privateThreadID := seedRouteForumThreadWithVisibility(t, "daily", "Private route note", "Route private body.", "private")

	router := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/forum/threads/"+privateThreadID, nil)
	rec := httptest.NewRecorder()
	c := router.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(privateThreadID)

	if err := routes.GetForumThread(c); err != nil {
		t.Fatalf("get private forum thread anonymously: %v", err)
	}
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected anonymous private thread lookup status %d, got %d body=%s", http.StatusNotFound, rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/forum/threads/"+privateThreadID, nil)
	rec = httptest.NewRecorder()
	c = router.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(privateThreadID)
	fwcontext.SetCurrentUser(c, &models.User{ID: forumRouteSeedUserID, Name: "Route User", IsActive: 1})

	if err := routes.GetForumThread(c); err != nil {
		t.Fatalf("get private forum thread as author: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected author private thread lookup status %d, got %d body=%s", http.StatusOK, rec.Code, rec.Body.String())
	}
}

func seedRouteForumThread(t *testing.T, categorySlug string, title string, bodyText string) {
	t.Helper()
	_ = seedRouteForumThreadWithVisibility(t, categorySlug, title, bodyText, "")
}

func seedRouteForumThreadWithVisibility(t *testing.T, categorySlug string, title string, bodyText string, visibility string) string {
	t.Helper()

	router := echo.New()
	payload := `{"category_slug":"` + categorySlug + `","title":"` + title + `","body":"` + bodyText + `"`
	if visibility != "" {
		payload += `,"visibility":"` + visibility + `"`
	}
	payload += `}`
	body := bytes.NewBufferString(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/forum/threads", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := router.NewContext(req, rec)
	fwcontext.SetCurrentUser(c, &models.User{ID: forumRouteSeedUserID, Name: "Route User", IsActive: 1})
	if err := routes.CreateForumThread(c); err != nil {
		t.Fatalf("seed forum thread: %v", err)
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("seed forum thread status %d body=%s", rec.Code, rec.Body.String())
	}
	var envelope struct {
		Data routes.ForumThreadDetailResponse `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode seeded forum thread: %v", err)
	}
	return envelope.Data.ID
}

const forumRouteSeedUserID = "019ea0c1-0001-7000-8000-000000000002"

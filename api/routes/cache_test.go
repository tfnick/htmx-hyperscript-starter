package routes_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/tfnick/go-svelte-starter/api/framework/cache"
	"github.com/tfnick/go-svelte-starter/api/routes"
)

func TestListCacheEntriesReturnsPaginatedDTO(t *testing.T) {
	setupRouteTestDBs(t)
	store := cache.DefaultRistrettoStore
	if err := store.Set(t.Context(), "settings.clarity", "script", []byte(""), time.Hour); err != nil {
		t.Fatalf("seed cache: %v", err)
	}
	t.Cleanup(func() { store.Delete(t.Context(), "settings.clarity", "script") })

	router := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/admin/cache?page=1&page_size=10", nil)
	rec := httptest.NewRecorder()
	c := router.NewContext(req, rec)

	if err := routes.ListCacheEntries(c); err != nil {
		t.Fatalf("list cache entries: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var envelope struct {
		Success bool                        `json:"success"`
		Data    routes.CacheEntriesResponse `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !envelope.Success || len(envelope.Data.Items) != 1 {
		t.Fatalf("unexpected cache list response: %s", rec.Body.String())
	}
	item := envelope.Data.Items[0]
	if item.Namespace != "settings.clarity" || item.Key != "script" || item.ValueSize != 0 {
		t.Fatalf("unexpected cache item: %#v", item)
	}
	if envelope.Data.Pagination.TotalItems != 1 {
		t.Fatalf("expected pagination total 1, got %#v", envelope.Data.Pagination)
	}
}

func TestGetCacheEntryReturnsTextValue(t *testing.T) {
	setupRouteTestDBs(t)
	store := cache.DefaultRistrettoStore
	if err := store.Set(t.Context(), "kb.embedding", "doc-1", []byte(`{"ready":true}`), 0); err != nil {
		t.Fatalf("seed cache: %v", err)
	}
	t.Cleanup(func() { store.Delete(t.Context(), "kb.embedding", "doc-1") })

	router := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/admin/cache/entry?namespace=kb.embedding&key=doc-1", nil)
	rec := httptest.NewRecorder()
	c := router.NewContext(req, rec)

	if err := routes.GetCacheEntry(c); err != nil {
		t.Fatalf("get cache entry: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var envelope struct {
		Success bool                      `json:"success"`
		Data    routes.CacheEntryResponse `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if envelope.Data.ValueEncoding != "text" || envelope.Data.Value != `{"ready":true}` {
		t.Fatalf("unexpected cache detail: %#v", envelope.Data)
	}
}

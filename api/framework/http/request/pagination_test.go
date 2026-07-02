package fwrequest_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	fwrequest "github.com/tfnick/go-svelte-starter/api/framework/http/request"
)

func TestPageQueryReadsPaginationQueryParameters(t *testing.T) {
	router := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/example?page=2&page_size=25", nil)
	c := router.NewContext(req, httptest.NewRecorder())

	query := fwrequest.PageQuery(c)

	if query.Page != 2 || query.PageSize != 25 {
		t.Fatalf("expected page query from request, got %#v", query)
	}
}

func TestPageQueryLeavesMissingValuesForUsecaseDefaults(t *testing.T) {
	router := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/example", nil)
	c := router.NewContext(req, httptest.NewRecorder())

	query := fwrequest.PageQuery(c)

	if query.Page != 0 || query.PageSize != 0 {
		t.Fatalf("expected missing query values to remain zero, got %#v", query)
	}
}

func TestPageQueryMarksInvalidValuesForUsecaseValidation(t *testing.T) {
	router := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/example?page=0&page_size=abc", nil)
	c := router.NewContext(req, httptest.NewRecorder())

	query := fwrequest.PageQuery(c)

	if query.Page != -1 || query.PageSize != -1 {
		t.Fatalf("expected invalid query values to become validation sentinels, got %#v", query)
	}
}

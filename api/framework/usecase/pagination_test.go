package usecase_test

import (
	"testing"

	fwusecase "github.com/tfnick/go-svelte-starter/api/framework/usecase"
)

func TestNormalizePageQueryAppliesDefaults(t *testing.T) {
	query, err := fwusecase.NormalizePageQuery(fwusecase.PageQuery{})
	if err != nil {
		t.Fatalf("normalize page query: %v", err)
	}

	if query.Page != fwusecase.DefaultPage || query.PageSize != fwusecase.DefaultPageSize {
		t.Fatalf("expected defaults, got %#v", query)
	}
	if query.Limit() != fwusecase.DefaultPageSize || query.Offset() != 0 {
		t.Fatalf("expected default limit/offset, got limit=%d offset=%d", query.Limit(), query.Offset())
	}
}

func TestNormalizePageQueryRejectsInvalidValues(t *testing.T) {
	cases := []struct {
		name  string
		query fwusecase.PageQuery
	}{
		{name: "negative page", query: fwusecase.PageQuery{Page: -1, PageSize: 10}},
		{name: "negative page size", query: fwusecase.PageQuery{Page: 1, PageSize: -1}},
		{name: "too large page size", query: fwusecase.PageQuery{Page: 1, PageSize: fwusecase.MaxPageSize + 1}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := fwusecase.NormalizePageQuery(tc.query)
			if err == nil {
				t.Fatalf("expected validation error")
			}
			if fwusecase.CodeOf(err) != fwusecase.CodeValidation {
				t.Fatalf("expected validation code, got %q", fwusecase.CodeOf(err))
			}
		})
	}
}

func TestNewPageResultCalculatesMetadata(t *testing.T) {
	page := fwusecase.NewPageResult(fwusecase.PageQuery{Page: 2, PageSize: 10}, 25)

	if page.TotalPages != 3 {
		t.Fatalf("expected 3 total pages, got %d", page.TotalPages)
	}
	if !page.HasPrevious || !page.HasNext {
		t.Fatalf("expected middle page to have previous and next, got %#v", page)
	}
}

func TestNewPageResultHandlesEmptyCollection(t *testing.T) {
	page := fwusecase.NewPageResult(fwusecase.PageQuery{Page: 1, PageSize: 10}, 0)

	if page.TotalPages != 0 || page.HasPrevious || page.HasNext {
		t.Fatalf("expected empty pagination metadata, got %#v", page)
	}
}

package namelookup

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestLoadDeduplicatesIDsBeforeBatchLoad(t *testing.T) {
	var loadedIDs []string
	names, err := Load(context.Background(), []string{"user-1", "", "user-2", "user-1"}, func(ctx context.Context, ids []string) (map[string]string, error) {
		loadedIDs = append(loadedIDs, ids...)
		return map[string]string{
			"user-1": "Ada",
			"user-2": "Grace",
		}, nil
	})
	if err != nil {
		t.Fatalf("load names: %v", err)
	}

	if !reflect.DeepEqual(loadedIDs, []string{"user-1", "user-2"}) {
		t.Fatalf("expected one batch load with deduplicated IDs, got %#v", loadedIDs)
	}
	if names["user-1"] != "Ada" || names["user-2"] != "Grace" {
		t.Fatalf("expected loaded names, got %#v", names)
	}
}

func TestLoadSkipsEmptyInput(t *testing.T) {
	called := false
	names, err := Load(context.Background(), []string{"", ""}, func(ctx context.Context, ids []string) (map[string]string, error) {
		called = true
		return nil, nil
	})
	if err != nil {
		t.Fatalf("load names: %v", err)
	}
	if called {
		t.Fatalf("did not expect loader to run for empty IDs")
	}
	if len(names) != 0 {
		t.Fatalf("expected empty name map, got %#v", names)
	}
}

func TestRowsToMap(t *testing.T) {
	names := RowsToMap([]Row{
		{ID: "p001", Name: "Phone"},
		{ID: "p002", Name: "Computer"},
	})

	if names["p001"] != "Phone" || names["p002"] != "Computer" {
		t.Fatalf("expected row map, got %#v", names)
	}
}

func TestRegistryResolveLoadsEachResourceOnceWithDeduplicatedIDs(t *testing.T) {
	type loadCall struct {
		key Key
		ids []string
	}
	var calls []loadCall

	registry := NewRegistry(
		Resource("user.display_name", func(ctx context.Context, ids []string) (map[string]string, error) {
			calls = append(calls, loadCall{key: "user.display_name", ids: append([]string(nil), ids...)})
			return map[string]string{
				"u1": "Ada",
				"u2": "Grace",
			}, nil
		}),
		Resource("product.display_name", func(ctx context.Context, ids []string) (map[string]string, error) {
			calls = append(calls, loadCall{key: "product.display_name", ids: append([]string(nil), ids...)})
			return map[string]string{
				"p1": "Keyboard",
			}, nil
		}),
	)

	result, err := registry.Resolve(context.Background(), func(batch *Batch) {
		batch.Add("user.display_name", "u1")
		batch.Add("user.display_name", "")
		batch.Add("user.display_name", "u2")
		batch.Add("user.display_name", "u1")
		batch.Add("product.display_name", "p1")
		batch.Add("product.display_name", "p1")
	})
	if err != nil {
		t.Fatalf("resolve names: %v", err)
	}

	expectedCalls := []loadCall{
		{key: "product.display_name", ids: []string{"p1"}},
		{key: "user.display_name", ids: []string{"u1", "u2"}},
	}
	if !reflect.DeepEqual(calls, expectedCalls) {
		t.Fatalf("expected sorted deduplicated loader calls %#v, got %#v", expectedCalls, calls)
	}
	if result.Name("user.display_name", "u1") != "Ada" {
		t.Fatalf("expected user name Ada")
	}
	if result.Name("product.display_name", "p1") != "Keyboard" {
		t.Fatalf("expected product name Keyboard")
	}
}

func TestCollectAddsIDsFromTypedItems(t *testing.T) {
	type productRow struct {
		CreatedByUserID string
		UpdatedByUserID string
	}

	batch := NewBatch()
	rows := []productRow{
		{CreatedByUserID: "u1", UpdatedByUserID: "u2"},
		{CreatedByUserID: "u1", UpdatedByUserID: ""},
	}

	Collect(batch, "user.nickname", rows, func(row productRow) string {
		return row.CreatedByUserID
	})
	Collect(batch, "user.nickname", rows, func(row productRow) string {
		return row.UpdatedByUserID
	})

	if !reflect.DeepEqual(batch.IDs("user.nickname"), []string{"u1", "u2"}) {
		t.Fatalf("expected collected unique user IDs, got %#v", batch.IDs("user.nickname"))
	}
}

func TestRegistryResolveFailsForMissingLoader(t *testing.T) {
	registry := NewRegistry()

	_, err := registry.Resolve(context.Background(), func(batch *Batch) {
		batch.Add("user.display_name", "u1")
	})
	if !errors.Is(err, ErrLoaderNotRegistered) {
		t.Fatalf("expected missing loader error, got %v", err)
	}
}

func TestRegistryResolveWrapsLoaderErrorWithKey(t *testing.T) {
	loadErr := errors.New("database unavailable")
	registry := NewRegistry(
		Resource("user.display_name", func(ctx context.Context, ids []string) (map[string]string, error) {
			return nil, loadErr
		}),
	)

	_, err := registry.Resolve(context.Background(), func(batch *Batch) {
		batch.Add("user.display_name", "u1")
	})
	if !errors.Is(err, loadErr) {
		t.Fatalf("expected wrapped loader error, got %v", err)
	}
	if err == nil || !strings.Contains(err.Error(), "user.display_name") {
		t.Fatalf("expected error to include lookup key, got %v", err)
	}
}

func TestRegistryResolvePassesContextToLoader(t *testing.T) {
	type contextKey struct{}
	ctx := context.WithValue(context.Background(), contextKey{}, "request-1")
	registry := NewRegistry(
		Resource("user.display_name", func(ctx context.Context, ids []string) (map[string]string, error) {
			if ctx.Value(contextKey{}) != "request-1" {
				t.Fatalf("expected request context to reach loader")
			}
			return map[string]string{"u1": "Ada"}, nil
		}),
	)

	_, err := registry.Resolve(ctx, func(batch *Batch) {
		batch.Add("user.display_name", "u1")
	})
	if err != nil {
		t.Fatalf("resolve names: %v", err)
	}
}

func TestResultReturnsEmptyForMissingNamesAndCopiesMaps(t *testing.T) {
	result := Result{names: map[Key]map[string]string{
		"user.display_name": {
			"u1": "Ada",
		},
	}}

	if result.Name("user.display_name", "missing") != "" {
		t.Fatalf("expected missing name to be empty")
	}

	names := result.Map("user.display_name")
	names["u1"] = "Changed"
	if result.Name("user.display_name", "u1") != "Ada" {
		t.Fatalf("expected result map to be immutable to callers")
	}
}

package cache

import (
	"context"
	"errors"
	"testing"
	"time"
)

type rebuildFakeStore struct {
	values map[string][]byte
	ttl    time.Duration
}

func resetRebuildRegistryForTest() {
	rebuildRegistry.Lock()
	defer rebuildRegistry.Unlock()
	rebuildRegistry.definitions = map[string]RebuildDefinition{}
}

func (s *rebuildFakeStore) Get(_ context.Context, namespace string, key string) ([]byte, bool, error) {
	if s.values == nil {
		return nil, false, nil
	}
	value, ok := s.values[definitionID(namespace, key)]
	if !ok {
		return nil, false, nil
	}
	return append([]byte(nil), value...), true, nil
}

func (s *rebuildFakeStore) Set(_ context.Context, namespace string, key string, value []byte, ttl time.Duration) error {
	if s.values == nil {
		s.values = map[string][]byte{}
	}
	s.values[definitionID(namespace, key)] = append([]byte(nil), value...)
	s.ttl = ttl
	return nil
}

func (s *rebuildFakeStore) Delete(_ context.Context, namespace string, key string) error {
	delete(s.values, definitionID(namespace, key))
	return nil
}

func TestRegisterRebuildableNormalizesAndListsDefinitions(t *testing.T) {
	resetRebuildRegistryForTest()
	t.Cleanup(resetRebuildRegistryForTest)

	if err := RegisterRebuildable(RebuildDefinition{
		Namespace: " settings.clarity ",
		CacheKey:  " script ",
		Owner:     "settings",
		Rebuild: func(context.Context, RebuildDefinition) (RebuildResult, error) {
			return RebuildResult{Value: []byte("script")}, nil
		},
	}); err != nil {
		t.Fatalf("register rebuildable: %v", err)
	}

	def, ok, err := LookupRebuildable("settings.clarity", "script")
	if err != nil || !ok {
		t.Fatalf("expected registered definition, ok=%v err=%v", ok, err)
	}
	if def.Namespace != "settings.clarity" || def.CacheKey != "script" || def.Owner != "settings" {
		t.Fatalf("unexpected definition: %#v", def)
	}

	definitions := RegisteredRebuildables()
	if len(definitions) != 1 || definitions[0].Namespace != "settings.clarity" {
		t.Fatalf("unexpected definitions: %#v", definitions)
	}
}

func TestGetOrRebuildReturnsExistingCacheHit(t *testing.T) {
	resetRebuildRegistryForTest()
	t.Cleanup(resetRebuildRegistryForTest)

	store := &rebuildFakeStore{values: map[string][]byte{
		definitionID("settings.clarity", "script"): []byte("cached"),
	}}
	called := false
	if err := RegisterRebuildable(RebuildDefinition{
		Namespace: "settings.clarity",
		CacheKey:  "script",
		Rebuild: func(context.Context, RebuildDefinition) (RebuildResult, error) {
			called = true
			return RebuildResult{Value: []byte("rebuilt")}, nil
		},
	}); err != nil {
		t.Fatalf("register rebuildable: %v", err)
	}

	value, ok, err := GetOrRebuild(context.Background(), store, "settings.clarity", "script")
	if err != nil || !ok || string(value) != "cached" {
		t.Fatalf("expected cache hit, value=%q ok=%v err=%v", value, ok, err)
	}
	if called {
		t.Fatalf("rebuild should not run on cache hit")
	}
}

func TestGetOrRebuildRebuildsAndStoresOnMiss(t *testing.T) {
	resetRebuildRegistryForTest()
	t.Cleanup(resetRebuildRegistryForTest)

	store := &rebuildFakeStore{}
	if err := RegisterRebuildable(RebuildDefinition{
		Namespace: "settings.clarity",
		CacheKey:  "script",
		Rebuild: func(_ context.Context, def RebuildDefinition) (RebuildResult, error) {
			if def.Namespace != "settings.clarity" || def.CacheKey != "script" {
				t.Fatalf("unexpected definition passed to rebuild: %#v", def)
			}
			return RebuildResult{Value: []byte("rebuilt"), TTL: time.Hour}, nil
		},
	}); err != nil {
		t.Fatalf("register rebuildable: %v", err)
	}

	value, ok, err := GetOrRebuild(context.Background(), store, "settings.clarity", "script")
	if err != nil || !ok || string(value) != "rebuilt" {
		t.Fatalf("expected rebuilt value, value=%q ok=%v err=%v", value, ok, err)
	}
	if stored := store.values[definitionID("settings.clarity", "script")]; string(stored) != "rebuilt" {
		t.Fatalf("expected rebuilt value to be stored, got %q", stored)
	}
	if store.ttl != time.Hour {
		t.Fatalf("expected rebuild ttl to be stored, got %s", store.ttl)
	}
}

func TestGetOrRebuildMissWithoutDefinition(t *testing.T) {
	resetRebuildRegistryForTest()
	t.Cleanup(resetRebuildRegistryForTest)

	value, ok, err := GetOrRebuild(context.Background(), &rebuildFakeStore{}, "kb", "missing")
	if err != nil || ok || value != nil {
		t.Fatalf("expected unresolved miss, value=%q ok=%v err=%v", value, ok, err)
	}
}

func TestRebuildReturnsDefinitionError(t *testing.T) {
	resetRebuildRegistryForTest()
	t.Cleanup(resetRebuildRegistryForTest)

	_, err := Rebuild(context.Background(), &rebuildFakeStore{}, "kb", "missing")
	if !errors.Is(err, ErrRebuildDefinitionNotFound) {
		t.Fatalf("expected missing definition error, got %v", err)
	}
}

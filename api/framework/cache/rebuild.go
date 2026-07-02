package cache

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

var (
	ErrInvalidRebuildDefinition  = errors.New("invalid cache rebuild definition")
	ErrRebuildDefinitionNotFound = errors.New("cache rebuild definition not found")
)

type RebuildFunc func(ctx context.Context, def RebuildDefinition) (RebuildResult, error)

type RebuildDefinition struct {
	Namespace   string
	CacheKey    string
	Owner       string
	Description string
	Source      string
	Rebuild     RebuildFunc
}

type RebuildResult struct {
	Value []byte
	TTL   time.Duration
}

var rebuildRegistry = struct {
	sync.RWMutex
	definitions map[string]RebuildDefinition
}{
	definitions: map[string]RebuildDefinition{},
}

func RegisterRebuildable(def RebuildDefinition) error {
	normalized, err := normalizeRebuildDefinition(def)
	if err != nil {
		return err
	}

	rebuildRegistry.Lock()
	defer rebuildRegistry.Unlock()
	rebuildRegistry.definitions[definitionID(normalized.Namespace, normalized.CacheKey)] = normalized
	return nil
}

func RegisteredRebuildables() []RebuildDefinition {
	rebuildRegistry.RLock()
	defer rebuildRegistry.RUnlock()

	definitions := make([]RebuildDefinition, 0, len(rebuildRegistry.definitions))
	for _, def := range rebuildRegistry.definitions {
		definitions = append(definitions, def)
	}
	sort.Slice(definitions, func(i int, j int) bool {
		if definitions[i].Namespace != definitions[j].Namespace {
			return definitions[i].Namespace < definitions[j].Namespace
		}
		return definitions[i].CacheKey < definitions[j].CacheKey
	})
	return definitions
}

func LookupRebuildable(namespace string, key string) (RebuildDefinition, bool, error) {
	namespace, key, err := normalizeKey(namespace, key)
	if err != nil {
		return RebuildDefinition{}, false, err
	}

	rebuildRegistry.RLock()
	defer rebuildRegistry.RUnlock()
	def, ok := rebuildRegistry.definitions[definitionID(namespace, key)]
	return def, ok, nil
}

func Rebuild(ctx context.Context, store Store, namespace string, key string) ([]byte, error) {
	if store == nil {
		return nil, fmt.Errorf("%w: store is required", ErrInvalidRebuildDefinition)
	}

	def, ok, err := LookupRebuildable(namespace, key)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("%w: %s/%s", ErrRebuildDefinitionNotFound, namespace, key)
	}

	result, err := def.Rebuild(ctx, def)
	if err != nil {
		return nil, fmt.Errorf("rebuild cache entry failed: %w", err)
	}
	if result.TTL < 0 {
		return nil, fmt.Errorf("%w: rebuild ttl must be zero or positive", ErrInvalidTTL)
	}
	value := append([]byte(nil), result.Value...)
	if err := store.Set(ctx, def.Namespace, def.CacheKey, value, result.TTL); err != nil {
		return nil, err
	}
	return value, nil
}

func GetOrRebuild(ctx context.Context, store Store, namespace string, key string) ([]byte, bool, error) {
	namespace, key, err := normalizeKey(namespace, key)
	if err != nil {
		return nil, false, err
	}
	if store == nil {
		return nil, false, fmt.Errorf("%w: store is required", ErrInvalidRebuildDefinition)
	}

	value, ok, err := store.Get(ctx, namespace, key)
	if err != nil {
		return nil, false, err
	}
	if ok {
		return value, true, nil
	}

	rebuilt, err := Rebuild(ctx, store, namespace, key)
	if err != nil {
		if errors.Is(err, ErrRebuildDefinitionNotFound) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return rebuilt, true, nil
}

func normalizeRebuildDefinition(def RebuildDefinition) (RebuildDefinition, error) {
	namespace, key, err := normalizeKey(def.Namespace, def.CacheKey)
	if err != nil {
		return RebuildDefinition{}, fmt.Errorf("%w: %v", ErrInvalidRebuildDefinition, err)
	}
	if def.Rebuild == nil {
		return RebuildDefinition{}, fmt.Errorf("%w: rebuild function is required", ErrInvalidRebuildDefinition)
	}
	def.Namespace = namespace
	def.CacheKey = key
	return def, nil
}

func definitionID(namespace string, key string) string {
	return namespace + "\x00" + key
}

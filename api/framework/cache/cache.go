package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"
)

var (
	ErrInvalidKey = errors.New("invalid cache key")
	ErrInvalidTTL = errors.New("invalid cache ttl")
)

type Store interface {
	Get(ctx context.Context, namespace string, key string) ([]byte, bool, error)
	Set(ctx context.Context, namespace string, key string, value []byte, ttl time.Duration) error
	Delete(ctx context.Context, namespace string, key string) error
}

type InspectableStore interface {
	Store
	CountEntries(ctx context.Context) (int, error)
	ListEntries(ctx context.Context, limit int, offset int) ([]EntrySummary, error)
	InspectEntry(ctx context.Context, namespace string, key string) (EntryDetail, bool, error)
	Metrics() CacheMetrics
}

func SharedStore() Store {
	return DefaultRistrettoStore
}

func SharedInspectableStore() InspectableStore {
	return DefaultRistrettoStore
}

var DefaultRistrettoStore *RistrettoStore

func init() {
	var err error
	DefaultRistrettoStore, err = NewRistrettoStore()
	if err != nil {
		log.Fatalf("failed to initialize default ristretto store: %v", err)
	}
}

type EntrySummary struct {
	Namespace string
	CacheKey  string
	ValueSize int
	ExpiresAt string
	CreatedAt string
	UpdatedAt string
	Expired   bool
}

type EntryDetail struct {
	EntrySummary
	Value []byte
}

func Cached[T any](ctx context.Context, namespace string, key string, ttl time.Duration, fetch func() (T, error)) (T, error) {
	return cachedWithStore(ctx, SharedStore(), namespace, key, ttl, fetch)
}

func cachedWithStore[T any](ctx context.Context, store Store, namespace string, key string, ttl time.Duration, fetch func() (T, error)) (T, error) {
	var zero T
	namespace, key, err := normalizeKey(namespace, key)
	if err != nil {
		return zero, err
	}

	raw, ok, err := store.Get(ctx, namespace, key)
	if err != nil {
		return zero, err
	}
	if ok {
		var result T
		if err := json.Unmarshal(raw, &result); err != nil {
			_ = store.Delete(ctx, namespace, key)
			return zero, fmt.Errorf("decode cached value: %w", err)
		}
		return result, nil
	}

	result, err := fetch()
	if err != nil {
		return zero, err
	}

	encoded, err := json.Marshal(result)
	if err != nil {
		return zero, fmt.Errorf("encode cache value: %w", err)
	}

	if err := store.Set(ctx, namespace, key, encoded, ttl); err != nil {
		return zero, fmt.Errorf("store cache entry: %w", err)
	}

	return result, nil
}

func normalizeKey(namespace string, key string) (string, string, error) {
	namespace = strings.TrimSpace(namespace)
	key = strings.TrimSpace(key)

	if namespace == "" {
		return "", "", fmt.Errorf("%w: namespace is required", ErrInvalidKey)
	}
	if key == "" {
		return "", "", fmt.Errorf("%w: key is required", ErrInvalidKey)
	}
	return namespace, key, nil
}

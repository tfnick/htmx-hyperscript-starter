package cache

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/dgraph-io/ristretto/v2"
	"github.com/tfnick/go-svelte-starter/api/framework/timefmt"
)

const (
	ristrettoMaxCost     = 10_000_000    // ~10 MB effective memory
	ristrettoNumCounters = 1_000_000     // 1M counters for frequency tracking
	ristrettoBufferItems = 64            // default buffer size per operation
	ristrettoMaxRetries  = 3             // retry Set if buffer is full
	ristrettoCost        = int64(1)      // uniform cost — each entry costs 1
)

type entryMeta struct {
	Namespace string
	CacheKey  string
	ValueSize int
	ExpiresAt string
	CreatedAt string
	UpdatedAt string
}

type CacheMetrics struct {
	Hits        uint64  `json:"hits"`
	Misses      uint64  `json:"misses"`
	HitRatio    float64 `json:"hit_ratio"`
	KeysAdded   uint64  `json:"keys_added"`
	KeysEvicted uint64  `json:"keys_evicted"`
	CostAdded   uint64  `json:"cost_added"`
	CostEvicted uint64  `json:"cost_evicted"`
	Entries     int     `json:"entries"`
}

type RistrettoStore struct {
	cache   *ristretto.Cache[string, []byte]
	mu      sync.RWMutex
	entries map[string]*entryMeta // key = namespace\x00cacheKey
}

func NewRistrettoStore() (*RistrettoStore, error) {
	c, err := ristretto.NewCache(&ristretto.Config[string, []byte]{
		MaxCost:     ristrettoMaxCost,
		NumCounters: ristrettoNumCounters,
		BufferItems: ristrettoBufferItems,
	})
	if err != nil {
		return nil, fmt.Errorf("create ristretto cache: %w", err)
	}

	return &RistrettoStore{
		cache:   c,
		entries: map[string]*entryMeta{},
	}, nil
}

func (s *RistrettoStore) Get(ctx context.Context, namespace string, key string) ([]byte, bool, error) {
	namespace, key, err := normalizeKey(namespace, key)
	if err != nil {
		return nil, false, err
	}

	value, ok := s.cache.Get(s.entryKey(namespace, key))
	if !ok {
		return nil, false, nil
	}

	s.mu.RLock()
	meta := s.entries[s.entryKey(namespace, key)]
	s.mu.RUnlock()

	if meta != nil && isRistrettoExpired(meta.ExpiresAt, timefmt.NowUTC()) {
		s.cache.Del(s.entryKey(namespace, key))
		s.mu.Lock()
		delete(s.entries, s.entryKey(namespace, key))
		s.mu.Unlock()
		return nil, false, nil
	}

	return value, true, nil
}

func (s *RistrettoStore) Set(ctx context.Context, namespace string, key string, value []byte, ttl time.Duration) error {
	namespace, key, err := normalizeKey(namespace, key)
	if err != nil {
		return err
	}
	if ttl < 0 {
		return fmt.Errorf("%w: ttl must be zero or positive", ErrInvalidTTL)
	}

	now := timefmt.NowUTC()
	var expiresAt string
	if ttl > 0 {
		expiresAt = timefmt.SQLiteDateTime(now.Add(ttl))
	}

	ek := s.entryKey(namespace, key)
	val := append([]byte(nil), value...)

	// Ristretto Set/SetWithTTL return false when the buffer is full.
	// Retry a few times with short backoff to avoid silent data loss.
	accepted := false
	for attempt := 0; attempt < ristrettoMaxRetries; attempt++ {
		if ttl > 0 {
			accepted = s.cache.SetWithTTL(ek, val, ristrettoCost, ttl)
		} else {
			accepted = s.cache.Set(ek, val, ristrettoCost)
		}
		if accepted {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if !accepted {
		return fmt.Errorf("ristretto set rejected: buffer full after %d retries", ristrettoMaxRetries)
	}

	nowSQL := timefmt.SQLiteDateTime(now)
	s.mu.Lock()
	s.entries[ek] = &entryMeta{
		Namespace: namespace,
		CacheKey:  key,
		ValueSize: len(val),
		ExpiresAt: expiresAt,
		CreatedAt: nowSQL,
		UpdatedAt: nowSQL,
	}
	s.mu.Unlock()

	return nil
}

func (s *RistrettoStore) Delete(ctx context.Context, namespace string, key string) error {
	namespace, key, err := normalizeKey(namespace, key)
	if err != nil {
		return err
	}

	ek := s.entryKey(namespace, key)
	s.cache.Del(ek)
	s.mu.Lock()
	delete(s.entries, ek)
	s.mu.Unlock()
	return nil
}

func (s *RistrettoStore) Metrics() CacheMetrics {
	m := s.cache.Metrics
	s.mu.RLock()
	entries := len(s.entries)
	s.mu.RUnlock()

	total := m.Hits() + m.Misses()
	var ratio float64
	if total > 0 {
		ratio = float64(m.Hits()) / float64(total)
	}

	return CacheMetrics{
		Hits:        m.Hits(),
		Misses:      m.Misses(),
		HitRatio:    ratio,
		KeysAdded:   m.KeysAdded(),
		KeysEvicted: m.KeysEvicted(),
		CostAdded:   m.CostAdded(),
		CostEvicted: m.CostEvicted(),
		Entries:     entries,
	}
}

func (s *RistrettoStore) CountEntries(ctx context.Context) (int, error) {
	s.mu.RLock()
	metaList := make([]*entryMeta, 0, len(s.entries))
	for _, meta := range s.entries {
		metaList = append(metaList, meta)
	}
	s.mu.RUnlock()

	count := 0
	for _, meta := range metaList {
		if _, ok := s.cache.Get(s.entryKey(meta.Namespace, meta.CacheKey)); ok {
			count++
		}
	}
	return count, nil
}

func (s *RistrettoStore) ListEntries(ctx context.Context, limit int, offset int) ([]EntrySummary, error) {
	if limit <= 0 {
		limit = 10
	}
	if offset < 0 {
		offset = 0
	}

	now := timefmt.NowUTC()

	s.mu.RLock()
	all := make([]*entryMeta, 0, len(s.entries))
	for _, meta := range s.entries {
		all = append(all, meta)
	}
	s.mu.RUnlock()

	sort.Slice(all, func(i, j int) bool {
		if all[i].UpdatedAt != all[j].UpdatedAt {
			return all[i].UpdatedAt > all[j].UpdatedAt
		}
		if all[i].Namespace != all[j].Namespace {
			return all[i].Namespace < all[j].Namespace
		}
		return all[i].CacheKey < all[j].CacheKey
	})

	entries := make([]EntrySummary, 0, limit)
	for i := offset; i < len(all) && len(entries) < limit; i++ {
		if _, ok := s.cache.Get(s.entryKey(all[i].Namespace, all[i].CacheKey)); !ok {
			continue
		}
		entries = append(entries, EntrySummary{
			Namespace: all[i].Namespace,
			CacheKey:  all[i].CacheKey,
			ValueSize: all[i].ValueSize,
			ExpiresAt: all[i].ExpiresAt,
			CreatedAt: all[i].CreatedAt,
			UpdatedAt: all[i].UpdatedAt,
			Expired:   isRistrettoExpired(all[i].ExpiresAt, now),
		})
	}
	return entries, nil
}

func (s *RistrettoStore) InspectEntry(ctx context.Context, namespace string, key string) (EntryDetail, bool, error) {
	namespace, key, err := normalizeKey(namespace, key)
	if err != nil {
		return EntryDetail{}, false, err
	}

	value, ok := s.cache.Get(s.entryKey(namespace, key))
	if !ok {
		return EntryDetail{}, false, nil
	}

	s.mu.RLock()
	meta := s.entries[s.entryKey(namespace, key)]
	s.mu.RUnlock()

	if meta == nil {
		return EntryDetail{}, false, nil
	}

	return EntryDetail{
		EntrySummary: EntrySummary{
			Namespace: meta.Namespace,
			CacheKey:  meta.CacheKey,
			ValueSize: meta.ValueSize,
			ExpiresAt: meta.ExpiresAt,
			CreatedAt: meta.CreatedAt,
			UpdatedAt: meta.UpdatedAt,
			Expired:   isRistrettoExpired(meta.ExpiresAt, timefmt.NowUTC()),
		},
		Value: append([]byte(nil), value...),
	}, true, nil
}

func (s *RistrettoStore) entryKey(namespace string, key string) string {
	return namespace + "\x00" + key
}

func isRistrettoExpired(expiresAt string, now time.Time) bool {
	if expiresAt == "" {
		return false
	}
	parsed, err := time.ParseInLocation(timefmt.SQLiteDateTimeLayout, expiresAt, time.UTC)
	if err != nil {
		return false
	}
	return !parsed.After(now.UTC())
}

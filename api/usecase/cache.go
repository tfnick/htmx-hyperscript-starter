package usecase

import (
	"encoding/base64"
	"strings"
	"unicode/utf8"

	"github.com/tfnick/go-svelte-starter/api/framework/cache"
	fwusecase "github.com/tfnick/go-svelte-starter/api/framework/usecase"
)

const (
	CacheEntryStatusActive  = "active"
	CacheEntryStatusExpired = "expired"
	CacheEntryStatusMissing = "missing"
)

type CacheEntriesQry struct {
	Page     int
	PageSize int
}

type CacheEntryQry struct {
	Namespace string
	Key       string
}

type CacheEntryCo struct {
	Namespace     string
	Key           string
	ValueSize     int
	ExpiresAt     string
	CreatedAt     string
	UpdatedAt     string
	Expired       bool
	Status        string
	Rebuildable   bool
	Missing       bool
	Owner         string
	Description   string
	Source        string
	ValueEncoding string
	Value         string
}

type CacheEntriesCo struct {
	Items      []CacheEntryCo
	Pagination fwusecase.PageResult
}

func ListCacheEntries(ctx fwusecase.Context, qry CacheEntriesQry) (CacheEntriesCo, error) {
	pageQuery, err := fwusecase.NormalizePageQuery(fwusecase.PageQuery{
		Page:     qry.Page,
		PageSize: qry.PageSize,
	})
	if err != nil {
		return CacheEntriesCo{}, err
	}

	entries, totalItems, err := listInspectableCacheEntries(ctx, cache.SharedInspectableStore(), pageQuery)
	if err != nil {
		return CacheEntriesCo{}, err
	}

	return CacheEntriesCo{
		Items:      entries,
		Pagination: fwusecase.NewPageResult(pageQuery, totalItems),
	}, nil
}

func GetCacheEntry(ctx fwusecase.Context, qry CacheEntryQry) (CacheEntryCo, error) {
	namespace, key, err := normalizeCacheIdentity(qry.Namespace, qry.Key)
	if err != nil {
		return CacheEntryCo{}, err
	}

	store := cache.SharedInspectableStore()
	entry, ok, err := store.InspectEntry(ctx.Std(), namespace, key)
	if err != nil {
		return CacheEntryCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to load cache entry", err)
	}
	if !ok {
		def, rebuildable, err := cache.LookupRebuildable(namespace, key)
		if err != nil {
			return CacheEntryCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to inspect cache rebuild definition", err)
		}
		if rebuildable {
			return cacheEntryCoFromMissingDefinition(def), nil
		}
		return CacheEntryCo{}, fwusecase.E(fwusecase.CodeNotFound, "cache entry not found", nil)
	}
	return cacheEntryCoFromDetail(entry), nil
}

func normalizeCacheIdentity(namespace string, key string) (string, string, error) {
	namespace = strings.TrimSpace(namespace)
	key = strings.TrimSpace(key)
	if namespace == "" {
		return "", "", fwusecase.E(fwusecase.CodeValidation, "cache namespace is required", nil)
	}
	if key == "" {
		return "", "", fwusecase.E(fwusecase.CodeValidation, "cache key is required", nil)
	}
	return namespace, key, nil
}

func listInspectableCacheEntries(ctx fwusecase.Context, inspector cache.InspectableStore, pageQuery fwusecase.PageQuery) ([]CacheEntryCo, int, error) {
	physicalTotal, err := inspector.CountEntries(ctx.Std())
	if err != nil {
		return nil, 0, fwusecase.E(fwusecase.CodeInternal, "failed to count cache entries", err)
	}

	missingDefinitions, err := missingRebuildableDefinitions(ctx, inspector)
	if err != nil {
		return nil, 0, err
	}

	items := make([]CacheEntryCo, 0, pageQuery.Limit())
	offset := pageQuery.Offset()
	if offset < physicalTotal {
		physicalLimit := pageQuery.Limit()
		remainingPhysical := physicalTotal - offset
		if physicalLimit > remainingPhysical {
			physicalLimit = remainingPhysical
		}
		entries, err := inspector.ListEntries(ctx.Std(), physicalLimit, offset)
		if err != nil {
			return nil, 0, fwusecase.E(fwusecase.CodeInternal, "failed to load cache entries", err)
		}
		items = append(items, cacheEntryCosFromSummaries(entries)...)
	}

	missingOffset := offset - physicalTotal
	if missingOffset < 0 {
		missingOffset = 0
	}
	if len(items) < pageQuery.Limit() && missingOffset < len(missingDefinitions) {
		missingLimit := pageQuery.Limit() - len(items)
		missingEnd := missingOffset + missingLimit
		if missingEnd > len(missingDefinitions) {
			missingEnd = len(missingDefinitions)
		}
		for i := missingOffset; i < missingEnd; i++ {
			items = append(items, cacheEntryCoFromMissingDefinition(missingDefinitions[i]))
		}
	}

	return items, physicalTotal + len(missingDefinitions), nil
}

func missingRebuildableDefinitions(ctx fwusecase.Context, inspector cache.InspectableStore) ([]cache.RebuildDefinition, error) {
	definitions := cache.RegisteredRebuildables()
	missing := make([]cache.RebuildDefinition, 0, len(definitions))
	for i := range definitions {
		_, ok, err := inspector.InspectEntry(ctx.Std(), definitions[i].Namespace, definitions[i].CacheKey)
		if err != nil {
			return nil, fwusecase.E(fwusecase.CodeInternal, "failed to inspect cache rebuild definition", err)
		}
		if ok {
			continue
		}
		missing = append(missing, definitions[i])
	}
	return missing, nil
}

func cacheEntryCoFromSummary(entry cache.EntrySummary) CacheEntryCo {
	status := CacheEntryStatusActive
	if entry.Expired {
		status = CacheEntryStatusExpired
	}
	result := CacheEntryCo{
		Namespace: entry.Namespace,
		Key:       entry.CacheKey,
		ValueSize: entry.ValueSize,
		ExpiresAt: entry.ExpiresAt,
		CreatedAt: entry.CreatedAt,
		UpdatedAt: entry.UpdatedAt,
		Expired:   entry.Expired,
		Status:    status,
	}
	if def, ok, err := cache.LookupRebuildable(result.Namespace, result.Key); err == nil && ok {
		applyRebuildDefinition(&result, def)
	}
	return result
}

func cacheEntryCosFromSummaries(entries []cache.EntrySummary) []CacheEntryCo {
	result := make([]CacheEntryCo, 0, len(entries))
	for i := range entries {
		result = append(result, cacheEntryCoFromSummary(entries[i]))
	}
	return result
}

func cacheEntryCoFromDetail(entry cache.EntryDetail) CacheEntryCo {
	result := cacheEntryCoFromSummary(entry.EntrySummary)
	result.ValueEncoding, result.Value = encodeCacheValue(entry.Value)
	return result
}

func cacheEntryCoFromMissingDefinition(def cache.RebuildDefinition) CacheEntryCo {
	result := CacheEntryCo{
		Namespace:   def.Namespace,
		Key:         def.CacheKey,
		Status:      CacheEntryStatusMissing,
		Rebuildable: true,
		Missing:     true,
	}
	applyRebuildDefinition(&result, def)
	return result
}

func applyRebuildDefinition(entry *CacheEntryCo, def cache.RebuildDefinition) {
	entry.Rebuildable = true
	entry.Owner = def.Owner
	entry.Description = def.Description
	entry.Source = def.Source
}

func GetCacheMetrics(ctx fwusecase.Context) (cache.CacheMetrics, error) {
	store := cache.SharedInspectableStore()
	return store.Metrics(), nil
}

func encodeCacheValue(value []byte) (string, string) {
	if utf8.Valid(value) {
		return "text", string(value)
	}
	return "base64", base64.StdEncoding.EncodeToString(value)
}

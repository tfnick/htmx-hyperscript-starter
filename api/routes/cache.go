package routes

import (
	"github.com/labstack/echo/v4"
	fwcontext "github.com/tfnick/go-svelte-starter/api/framework/http/context"
	fwrequest "github.com/tfnick/go-svelte-starter/api/framework/http/request"
	httpresponse "github.com/tfnick/go-svelte-starter/api/framework/http/response"
	"github.com/tfnick/go-svelte-starter/api/usecase"
)

type CacheEntryResponse struct {
	Namespace     string `json:"namespace"`
	Key           string `json:"key"`
	ValueSize     int    `json:"value_size"`
	ExpiresAt     string `json:"expires_at"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
	Expired       bool   `json:"expired"`
	Status        string `json:"status"`
	Rebuildable   bool   `json:"rebuildable"`
	Missing       bool   `json:"missing"`
	Owner         string `json:"owner,omitempty"`
	Description   string `json:"description,omitempty"`
	Source        string `json:"source,omitempty"`
	ValueEncoding string `json:"value_encoding,omitempty"`
	Value         string `json:"value,omitempty"`
}

type CacheMetricsResponse struct {
	Hits        uint64  `json:"hits"`
	Misses      uint64  `json:"misses"`
	HitRatio    float64 `json:"hit_ratio"`
	KeysAdded   uint64  `json:"keys_added"`
	KeysEvicted uint64  `json:"keys_evicted"`
	CostAdded   uint64  `json:"cost_added"`
	CostEvicted uint64  `json:"cost_evicted"`
	Entries     int     `json:"entries"`
}

func GetCacheMetrics(c echo.Context) error {
	ctx := fwcontext.InternalUsecaseContext(c)
	metrics, err := usecase.GetCacheMetrics(ctx)
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}
	return httpresponse.OK(c, CacheMetricsResponse{
		Hits:        metrics.Hits,
		Misses:      metrics.Misses,
		HitRatio:    metrics.HitRatio,
		KeysAdded:   metrics.KeysAdded,
		KeysEvicted: metrics.KeysEvicted,
		CostAdded:   metrics.CostAdded,
		CostEvicted: metrics.CostEvicted,
		Entries:     metrics.Entries,
	})
}

type CacheEntriesResponse struct {
	Items      []CacheEntryResponse `json:"items"`
	Pagination PaginationResponse   `json:"pagination"`
}

func ListCacheEntries(c echo.Context) error {
	page := fwrequest.PageQuery(c)
	ctx := fwcontext.InternalUsecaseContext(c)
	entries, err := usecase.ListCacheEntries(ctx, usecase.CacheEntriesQry{
		Page:     page.Page,
		PageSize: page.PageSize,
	})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}
	return httpresponse.OK(c, ToCacheEntriesResponse(entries))
}

func GetCacheEntry(c echo.Context) error {
	ctx := fwcontext.InternalUsecaseContext(c)
	entry, err := usecase.GetCacheEntry(ctx, usecase.CacheEntryQry{
		Namespace: c.QueryParam("namespace"),
		Key:       c.QueryParam("key"),
	})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}
	return httpresponse.OK(c, ToCacheEntryResponse(entry))
}

func ToCacheEntryResponse(entry usecase.CacheEntryCo) CacheEntryResponse {
	return CacheEntryResponse{
		Namespace:     entry.Namespace,
		Key:           entry.Key,
		ValueSize:     entry.ValueSize,
		ExpiresAt:     entry.ExpiresAt,
		CreatedAt:     entry.CreatedAt,
		UpdatedAt:     entry.UpdatedAt,
		Expired:       entry.Expired,
		Status:        entry.Status,
		Rebuildable:   entry.Rebuildable,
		Missing:       entry.Missing,
		Owner:         entry.Owner,
		Description:   entry.Description,
		Source:        entry.Source,
		ValueEncoding: entry.ValueEncoding,
		Value:         entry.Value,
	}
}

func ToCacheEntryResponses(entries []usecase.CacheEntryCo) []CacheEntryResponse {
	responses := make([]CacheEntryResponse, 0, len(entries))
	for i := range entries {
		responses = append(responses, ToCacheEntryResponse(entries[i]))
	}
	return responses
}

func ToCacheEntriesResponse(entries usecase.CacheEntriesCo) CacheEntriesResponse {
	return CacheEntriesResponse{
		Items:      ToCacheEntryResponses(entries.Items),
		Pagination: ToPaginationResponse(entries.Pagination),
	}
}

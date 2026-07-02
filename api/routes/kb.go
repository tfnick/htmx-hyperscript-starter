package routes

import (
	"github.com/labstack/echo/v4"
	fwcontext "github.com/tfnick/go-svelte-starter/api/framework/http/context"
	httpresponse "github.com/tfnick/go-svelte-starter/api/framework/http/response"
	"github.com/tfnick/go-svelte-starter/api/usecase"
)

// --- Request DTOs ---

type SaveKBSourceRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	SourceType  string `json:"source_type"`
}

type SetKBSourceEnabledRequest struct {
	Enabled bool `json:"enabled"`
}

type SaveKBDocumentRequest struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

type SetKBDocumentEnabledRequest struct {
	Enabled bool `json:"enabled"`
}

// --- Response DTOs ---

type KBSourceResponse struct {
	ID             string `json:"id"`
	Title          string `json:"title"`
	Description    string `json:"description"`
	SourceType     string `json:"source_type"`
	Category       string `json:"category"`
	Tags           string `json:"tags"`
	SourceURL      string `json:"source_url"`
	Enabled        bool   `json:"enabled"`
	IndexStatus    string `json:"index_status"`
	Version        int    `json:"version"`
	ContentHash    string `json:"content_hash"`
	LastIndexedAt  string `json:"last_indexed_at,omitempty"`
	LastIndexError string `json:"last_index_error,omitempty"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}

type KBDocumentResponse struct {
	ID             string `json:"id"`
	SourceID       string `json:"source_id"`
	Title          string `json:"title"`
	Content        string `json:"content"`
	ExtractedText  string `json:"extracted_text,omitempty"`
	ContentHash    string `json:"content_hash"`
	Version        int    `json:"version"`
	Enabled        bool   `json:"enabled"`
	IndexStatus    string `json:"index_status"`
	LastIndexedAt  string `json:"last_indexed_at,omitempty"`
	LastIndexError string `json:"last_index_error,omitempty"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}

type KBSourceMutationResponse struct {
	Message string           `json:"message"`
	Source  KBSourceResponse `json:"source"`
}

type KBDocumentMutationResponse struct {
	Message  string             `json:"message"`
	Document KBDocumentResponse `json:"document"`
}

// --- Converter helpers ---

func toKBSourceResponse(source usecase.KBSourceCo) KBSourceResponse {
	return KBSourceResponse{
		ID:             source.ID,
		Title:          source.Title,
		Description:    source.Description,
		SourceType:     source.SourceType,
		Category:       source.Category,
		Tags:           source.Tags,
		SourceURL:      source.SourceURL,
		Enabled:        source.Enabled,
		IndexStatus:    source.IndexStatus,
		Version:        source.Version,
		ContentHash:    source.ContentHash,
		LastIndexedAt:  source.LastIndexedAt,
		LastIndexError: source.LastIndexError,
		CreatedAt:      source.CreatedAt,
		UpdatedAt:      source.UpdatedAt,
	}
}

func toKBSourceResponses(sources []usecase.KBSourceCo) []KBSourceResponse {
	result := make([]KBSourceResponse, 0, len(sources))
	for i := range sources {
		result = append(result, toKBSourceResponse(sources[i]))
	}
	return result
}

func toKBDocumentResponse(doc usecase.KBDocumentCo) KBDocumentResponse {
	return KBDocumentResponse{
		ID:             doc.ID,
		SourceID:       doc.SourceID,
		Title:          doc.Title,
		Content:        doc.Content,
		ExtractedText:  doc.ExtractedText,
		ContentHash:    doc.ContentHash,
		Version:        doc.Version,
		Enabled:        doc.Enabled,
		IndexStatus:    doc.IndexStatus,
		LastIndexedAt:  doc.LastIndexedAt,
		LastIndexError: doc.LastIndexError,
		CreatedAt:      doc.CreatedAt,
		UpdatedAt:      doc.UpdatedAt,
	}
}

func toKBDocumentResponses(docs []usecase.KBDocumentCo) []KBDocumentResponse {
	result := make([]KBDocumentResponse, 0, len(docs))
	for i := range docs {
		result = append(result, toKBDocumentResponse(docs[i]))
	}
	return result
}

// --- Source handlers ---

func ListKBSources(c echo.Context) error {
	ctx := fwcontext.InternalUsecaseContext(c)
	sources, err := usecase.ListKBSources(ctx)
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}
	return httpresponse.OK(c, toKBSourceResponses(sources))
}

func CreateKBSource(c echo.Context) error {
	var req SaveKBSourceRequest
	if err := c.Bind(&req); err != nil {
		return httpresponse.BadRequest(c, "invalid request data")
	}
	ctx := fwcontext.InternalUsecaseContext(c)
	source, err := usecase.CreateKBSource(ctx, usecase.CreateKBSourceCmd{
		Title:       req.Title,
		Description: req.Description,
		SourceType:  req.SourceType,
	})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}
	return httpresponse.Created(c, KBSourceMutationResponse{
		Message: "knowledge source created",
		Source:  toKBSourceResponse(source),
	})
}

func UpdateKBSource(c echo.Context) error {
	var req SaveKBSourceRequest
	if err := c.Bind(&req); err != nil {
		return httpresponse.BadRequest(c, "invalid request data")
	}
	ctx := fwcontext.InternalUsecaseContext(c)
	source, err := usecase.UpdateKBSource(ctx, usecase.UpdateKBSourceCmd{
		ID:          c.Param("id"),
		Title:       req.Title,
		Description: req.Description,
	})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}
	return httpresponse.OK(c, KBSourceMutationResponse{
		Message: "knowledge source updated",
		Source:  toKBSourceResponse(source),
	})
}

func SetKBSourceEnabled(c echo.Context) error {
	var req SetKBSourceEnabledRequest
	if err := c.Bind(&req); err != nil {
		return httpresponse.BadRequest(c, "invalid request data")
	}
	ctx := fwcontext.InternalUsecaseContext(c)
	source, err := usecase.SetKBSourceEnabled(ctx, c.Param("id"), req.Enabled)
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}
	return httpresponse.OK(c, KBSourceMutationResponse{
		Message: "knowledge source enabled status updated",
		Source:  toKBSourceResponse(source),
	})
}

// --- Document handlers ---

func ListKBDocuments(c echo.Context) error {
	ctx := fwcontext.InternalUsecaseContext(c)
	docs, err := usecase.ListKBDocuments(ctx, c.Param("source_id"))
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}
	return httpresponse.OK(c, toKBDocumentResponses(docs))
}

func CreateKBDocument(c echo.Context) error {
	var req SaveKBDocumentRequest
	if err := c.Bind(&req); err != nil {
		return httpresponse.BadRequest(c, "invalid request data")
	}
	ctx := fwcontext.InternalUsecaseContext(c)
	doc, err := usecase.CreateKBDocument(ctx, usecase.CreateKBDocumentCmd{
		SourceID: c.Param("source_id"),
		Title:    req.Title,
		Content:  req.Content,
	})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}
	return httpresponse.Created(c, KBDocumentMutationResponse{
		Message:  "document created",
		Document: toKBDocumentResponse(doc),
	})
}

func UpdateKBDocument(c echo.Context) error {
	var req SaveKBDocumentRequest
	if err := c.Bind(&req); err != nil {
		return httpresponse.BadRequest(c, "invalid request data")
	}
	ctx := fwcontext.InternalUsecaseContext(c)
	doc, err := usecase.UpdateKBDocument(ctx, usecase.UpdateKBDocumentCmd{
		ID:      c.Param("id"),
		Title:   req.Title,
		Content: req.Content,
	})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}
	return httpresponse.OK(c, KBDocumentMutationResponse{
		Message:  "document updated",
		Document: toKBDocumentResponse(doc),
	})
}

func SetKBDocumentEnabled(c echo.Context) error {
	var req SetKBDocumentEnabledRequest
	if err := c.Bind(&req); err != nil {
		return httpresponse.BadRequest(c, "invalid request data")
	}
	ctx := fwcontext.InternalUsecaseContext(c)
	doc, err := usecase.SetKBDocumentEnabled(ctx, c.Param("id"), req.Enabled)
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}
	return httpresponse.OK(c, KBDocumentMutationResponse{
		Message:  "document enabled status updated",
		Document: toKBDocumentResponse(doc),
	})
}

func DeleteKBDocument(c echo.Context) error {
	ctx := fwcontext.InternalUsecaseContext(c)
	if err := usecase.DeleteKBDocument(ctx, usecase.DeleteKBDocumentCmd{
		SourceID:   c.Param("source_id"),
		DocumentID: c.Param("id"),
	}); err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}
	return httpresponse.OKMessage(c, "document deleted")
}

func ReindexKBDocument(c echo.Context) error {
	ctx := fwcontext.InternalUsecaseContext(c)
	if err := usecase.IndexDocument(ctx, usecase.IndexDocumentCmd{DocumentID: c.Param("id")}); err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}
	return httpresponse.OK(c, map[string]string{"message": "document reindexed"})
}

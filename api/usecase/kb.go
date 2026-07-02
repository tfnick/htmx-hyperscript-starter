package usecase

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"

	"github.com/tfnick/go-svelte-starter/api/framework/data/modelerror"
	fwusecase "github.com/tfnick/go-svelte-starter/api/framework/usecase"
	"github.com/tfnick/go-svelte-starter/api/models"
)

// --- CO types ---

type KBSourceCo struct {
	ID             string
	Title          string
	Description    string
	SourceType     string
	Category       string
	Tags           string
	SourceURL      string
	Enabled        bool
	IndexStatus    string
	Version        int
	ContentHash    string
	LastIndexedAt  string
	LastIndexError string
	CreatedAt      string
	UpdatedAt      string
}

type KBDocumentCo struct {
	ID             string
	SourceID       string
	Title          string
	Content        string
	ExtractedText  string
	ContentHash    string
	Version        int
	Enabled        bool
	IndexStatus    string
	LastIndexedAt  string
	LastIndexError string
	CreatedAt      string
	UpdatedAt      string
}

// --- Commands ---

type CreateKBSourceCmd struct {
	Title       string
	Description string
	SourceType  string
}

type UpdateKBSourceCmd struct {
	ID          string
	Title       string
	Description string
}

type CreateKBDocumentCmd struct {
	SourceID string
	Title    string
	Content  string
}

type UpdateKBDocumentCmd struct {
	ID      string
	Title   string
	Content string
}

type DeleteKBDocumentCmd struct {
	SourceID   string
	DocumentID string
}

// --- Usecase functions ---

func ListKBSources(ctx fwusecase.Context) ([]KBSourceCo, error) {
	records, err := models.ListKnowledgeSources(ctx.Std())
	if err != nil {
		return nil, fwusecase.E(fwusecase.CodeInternal, "failed to load knowledge sources", err)
	}
	result := make([]KBSourceCo, 0, len(records))
	for i := range records {
		result = append(result, kbSourceCoFromModel(records[i]))
	}
	return result, nil
}

func CreateKBSource(ctx fwusecase.Context, cmd CreateKBSourceCmd) (KBSourceCo, error) {
	title := strings.TrimSpace(cmd.Title)
	sourceType := strings.TrimSpace(cmd.SourceType)

	if title == "" {
		return KBSourceCo{}, fwusecase.E(fwusecase.CodeValidation, "source title is required", nil)
	}
	if sourceType == "" {
		sourceType = models.KBSourceTypeManual
	}
	if !isValidKBSourceType(sourceType) {
		return KBSourceCo{}, fwusecase.E(fwusecase.CodeValidation, "invalid source type", nil)
	}

	content := strings.TrimSpace(cmd.Description)
	contentHash := computeContentHash(content)

	record, err := models.CreateKnowledgeSource(ctx.Std(), models.SaveKnowledgeSourceCmd{
		Title:       title,
		SourceType:  sourceType,
		Enabled:     true,
		Content:     content,
		ContentHash: contentHash,
	})
	if err != nil {
		return KBSourceCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to create knowledge source", err)
	}
	return kbSourceCoFromModel(record), nil
}

func UpdateKBSource(ctx fwusecase.Context, cmd UpdateKBSourceCmd) (KBSourceCo, error) {
	id := strings.TrimSpace(cmd.ID)
	if id == "" {
		return KBSourceCo{}, fwusecase.E(fwusecase.CodeValidation, "source ID is required", nil)
	}

	title := strings.TrimSpace(cmd.Title)
	if title == "" {
		return KBSourceCo{}, fwusecase.E(fwusecase.CodeValidation, "source title is required", nil)
	}

	existing, err := models.GetKnowledgeSourceByID(ctx.Std(), id)
	if err != nil {
		if errors.Is(err, modelerror.ErrNotFound) {
			return KBSourceCo{}, fwusecase.E(fwusecase.CodeNotFound, "knowledge source not found", err)
		}
		return KBSourceCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to load knowledge source", err)
	}

	content := strings.TrimSpace(cmd.Description)
	contentHash := computeContentHash(content)

	record, err := models.UpdateKnowledgeSource(ctx.Std(), models.SaveKnowledgeSourceCmd{
		ID:          id,
		Title:       title,
		SourceType:  existing.SourceType,
		Enabled:     existing.Enabled,
		Content:     content,
		ContentHash: contentHash,
	})
	if err != nil {
		return KBSourceCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to update knowledge source", err)
	}
	return kbSourceCoFromModel(record), nil
}

func SetKBSourceEnabled(ctx fwusecase.Context, id string, enabled bool) (KBSourceCo, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return KBSourceCo{}, fwusecase.E(fwusecase.CodeValidation, "source ID is required", nil)
	}
	record, err := models.SetKnowledgeSourceEnabled(ctx.Std(), id, enabled)
	if err != nil {
		if errors.Is(err, modelerror.ErrNotFound) {
			return KBSourceCo{}, fwusecase.E(fwusecase.CodeNotFound, "knowledge source not found", err)
		}
		return KBSourceCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to update knowledge source", err)
	}
	return kbSourceCoFromModel(record), nil
}

func ListKBDocuments(ctx fwusecase.Context, sourceID string) ([]KBDocumentCo, error) {
	sourceID = strings.TrimSpace(sourceID)
	if sourceID == "" {
		return nil, fwusecase.E(fwusecase.CodeValidation, "source ID is required", nil)
	}
	docs, err := models.ListKBDocumentsBySourceID(ctx.Std(), sourceID)
	if err != nil {
		return nil, fwusecase.E(fwusecase.CodeInternal, "failed to load documents", err)
	}
	result := make([]KBDocumentCo, 0, len(docs))
	for i := range docs {
		result = append(result, kbDocumentCoFromModel(docs[i]))
	}
	return result, nil
}

func CreateKBDocument(ctx fwusecase.Context, cmd CreateKBDocumentCmd) (KBDocumentCo, error) {
	sourceID := strings.TrimSpace(cmd.SourceID)
	title := strings.TrimSpace(cmd.Title)

	if sourceID == "" {
		return KBDocumentCo{}, fwusecase.E(fwusecase.CodeValidation, "source ID is required", nil)
	}
	if title == "" {
		return KBDocumentCo{}, fwusecase.E(fwusecase.CodeValidation, "document title is required", nil)
	}

	content := strings.TrimSpace(cmd.Content)
	if content == "" {
		return KBDocumentCo{}, fwusecase.E(fwusecase.CodeValidation, "document content is required", nil)
	}
	contentHash := computeContentHash(content)

	doc, err := models.CreateKBDocument(ctx.Std(), models.SaveKBDocumentCmd{
		SourceID:    sourceID,
		Title:       title,
		Content:     content,
		ContentHash: contentHash,
		Enabled:     true,
	})
	if err != nil {
		if errors.Is(err, modelerror.ErrNotFound) {
			return KBDocumentCo{}, fwusecase.E(fwusecase.CodeNotFound, "knowledge source not found", err)
		}
		return KBDocumentCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to create document", err)
	}

	// Enqueue indexing asynchronously
	enqueueIndexDocument(ctx, doc.ID)

	return kbDocumentCoFromModel(doc), nil
}

func UpdateKBDocument(ctx fwusecase.Context, cmd UpdateKBDocumentCmd) (KBDocumentCo, error) {
	id := strings.TrimSpace(cmd.ID)
	if id == "" {
		return KBDocumentCo{}, fwusecase.E(fwusecase.CodeValidation, "document ID is required", nil)
	}

	title := strings.TrimSpace(cmd.Title)
	if title == "" {
		return KBDocumentCo{}, fwusecase.E(fwusecase.CodeValidation, "document title is required", nil)
	}

	content := strings.TrimSpace(cmd.Content)
	if content == "" {
		return KBDocumentCo{}, fwusecase.E(fwusecase.CodeValidation, "document content is required", nil)
	}
	contentHash := computeContentHash(content)

	doc, err := models.UpdateKBDocumentContent(ctx.Std(), id, title, content, contentHash)
	if err != nil {
		if errors.Is(err, modelerror.ErrNotFound) {
			return KBDocumentCo{}, fwusecase.E(fwusecase.CodeNotFound, "document not found", err)
		}
		return KBDocumentCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to update document", err)
	}

	// Enqueue indexing asynchronously
	enqueueIndexDocument(ctx, id)

	return kbDocumentCoFromModel(doc), nil
}

func SetKBDocumentEnabled(ctx fwusecase.Context, id string, enabled bool) (KBDocumentCo, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return KBDocumentCo{}, fwusecase.E(fwusecase.CodeValidation, "document ID is required", nil)
	}
	doc, err := models.SetKBDocumentEnabled(ctx.Std(), id, enabled)
	if err != nil {
		if errors.Is(err, modelerror.ErrNotFound) {
			return KBDocumentCo{}, fwusecase.E(fwusecase.CodeNotFound, "document not found", err)
		}
		return KBDocumentCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to update document", err)
	}
	return kbDocumentCoFromModel(doc), nil
}

func DeleteKBDocument(ctx fwusecase.Context, cmd DeleteKBDocumentCmd) error {
	sourceID := strings.TrimSpace(cmd.SourceID)
	documentID := strings.TrimSpace(cmd.DocumentID)
	if sourceID == "" {
		return fwusecase.E(fwusecase.CodeValidation, "source ID is required", nil)
	}
	if documentID == "" {
		return fwusecase.E(fwusecase.CodeValidation, "document ID is required", nil)
	}

	err := fwusecase.WithAppTx(ctx, func(txCtx fwusecase.Context) error {
		doc, err := models.GetKBDocumentByID(txCtx.Std(), documentID)
		if err != nil {
			return err
		}
		if doc.SourceID != sourceID {
			return modelerror.ErrNotFound
		}
		if err := models.DeleteKBDocument(txCtx.Std(), documentID); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, modelerror.ErrNotFound) {
			return fwusecase.E(fwusecase.CodeNotFound, "document not found", err)
		}
		return fwusecase.E(fwusecase.CodeInternal, "failed to delete document", err)
	}
	return nil
}

// --- Converters ---

func kbSourceCoFromModel(record models.KnowledgeSourceRecord) KBSourceCo {
	return KBSourceCo{
		ID:             record.ID,
		Title:          record.Title,
		Description:    record.Content,
		SourceType:     record.SourceType,
		Category:       record.Category,
		Tags:           record.Tags,
		SourceURL:      record.SourceURL,
		Enabled:        record.Enabled,
		IndexStatus:    record.IndexStatus,
		Version:        record.Version,
		ContentHash:    record.ContentHash,
		LastIndexedAt:  record.LastIndexedAt,
		LastIndexError: record.LastIndexError,
		CreatedAt:      record.CreatedAt,
		UpdatedAt:      record.UpdatedAt,
	}
}

func kbDocumentCoFromModel(doc models.KBDocument) KBDocumentCo {
	return KBDocumentCo{
		ID:             doc.ID,
		SourceID:       doc.SourceID,
		Title:          doc.Title,
		Content:        doc.Content,
		ExtractedText:  doc.ExtractedText,
		ContentHash:    doc.ContentHash,
		Version:        doc.Version,
		Enabled:        doc.Enabled == 1,
		IndexStatus:    doc.IndexStatus,
		LastIndexedAt:  doc.LastIndexedAt,
		LastIndexError: doc.LastIndexError,
		CreatedAt:      doc.CreatedAt,
		UpdatedAt:      doc.UpdatedAt,
	}
}

// --- Indexing ---

// IndexDocumentCmd initiates an indexing job for a document.
type IndexDocumentCmd struct {
	DocumentID string
}

// IndexDocument performs the full indexing workflow: chunk content, generate embeddings, store vectors.
// On success the document and source status are set to "indexed". On failure they are set to "failed"
// with the error message recorded.
func IndexDocument(ctx fwusecase.Context, cmd IndexDocumentCmd) error {
	documentID := strings.TrimSpace(cmd.DocumentID)
	if documentID == "" {
		return fwusecase.E(fwusecase.CodeValidation, "document ID is required", nil)
	}

	// Verify document exists
	_, err := models.GetKBDocumentByID(ctx.Std(), documentID)
	if err != nil {
		if errors.Is(err, modelerror.ErrNotFound) {
			return fwusecase.E(fwusecase.CodeNotFound, "document not found", err)
		}
		return fwusecase.E(fwusecase.CodeInternal, "failed to load document for indexing", err)
	}

	// Perform indexing
	if err := indexDocumentInternal(ctx.Std(), documentID); err != nil {
		if errors.Is(err, errKBDocumentHasNoContent) {
			return fwusecase.E(fwusecase.CodeValidation, "document content is required before indexing", err)
		}
		if errors.Is(err, errKBDocumentHasNoChunks) {
			return fwusecase.E(fwusecase.CodeValidation, "document content produced no indexable chunks", err)
		}
		return fwusecase.E(fwusecase.CodeInternal, "failed to index document", err)
	}

	// Reload the document to return the latest status
	_, err = models.GetKBDocumentByID(ctx.Std(), documentID)
	if err != nil {
		return fwusecase.E(fwusecase.CodeInternal, "failed to load document after indexing", err)
	}

	return nil
}

// enqueueIndexDocument runs the indexing synchronously for MVP.
// The document already has status "pending" set by Create/Update, and indexDocumentInternal
// will update it to "processing" -> "indexed" or "failed".
// Synchronous execution ensures the admin sees the result immediately and the context
// is not cancelled before the indexing completes.
func enqueueIndexDocument(ctx fwusecase.Context, documentID string) {
	_ = indexDocumentInternal(ctx.Std(), documentID)
}

// --- Helpers ---

func isValidKBSourceType(value string) bool {
	switch value {
	case models.KBSourceTypeManual, models.KBSourceTypeMarkdown, models.KBSourceTypeURL:
		return true
	default:
		return false
	}
}

func computeContentHash(content string) string {
	if content == "" {
		return ""
	}
	h := sha256.Sum256([]byte(content))
	return hex.EncodeToString(h[:])
}

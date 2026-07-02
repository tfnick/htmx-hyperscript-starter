package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"time"

	"github.com/tfnick/go-svelte-starter/api/framework/cache"
	"github.com/tfnick/go-svelte-starter/api/models"
	"github.com/tfnick/go-svelte-starter/api/usecase/integrations/embedding"
	"github.com/tfnick/go-svelte-starter/api/usecase/integrations/kb"
)

var (
	errKBDocumentHasNoContent = errors.New("document has no content")
	errKBDocumentHasNoChunks  = errors.New("document produced no chunks")
)

const sqliteVecEmbeddingDimensions = 64

// SQLiteVecRetriever implements kb.Retriever and kb.Indexer using the app SQLite database
// and the sqlite-vec vec0 virtual table for KNN vector search.
type SQLiteVecRetriever struct {
	store KnowledgeVectorStore
}

type KnowledgeVectorStore interface {
	ReplaceDocumentChunks(ctx context.Context, documentID string, chunks []models.KnowledgeChunkInsert) error
	Search(ctx context.Context, embeddingJSON string, limit int) ([]models.KnowledgeSearchResult, error)
	SearchText(ctx context.Context, terms []string, limit int) ([]models.KnowledgeSearchResult, error)
}

type SQLiteKnowledgeVectorStore struct{}

// NewSQLiteVecRetriever creates a new retriever backed by the default SQLite database manager.
func NewSQLiteVecRetriever() *SQLiteVecRetriever {
	return NewKnowledgeRetriever(NewDefaultKnowledgeVectorStore())
}

func NewKnowledgeRetriever(store KnowledgeVectorStore) *SQLiteVecRetriever {
	if store == nil {
		store = NewDefaultKnowledgeVectorStore()
	}
	return &SQLiteVecRetriever{store: store}
}

func NewDefaultKnowledgeVectorStore() KnowledgeVectorStore {
	return SQLiteKnowledgeVectorStore{}
}

func (SQLiteKnowledgeVectorStore) ReplaceDocumentChunks(ctx context.Context, documentID string, chunks []models.KnowledgeChunkInsert) error {
	return models.ReplaceKnowledgeDocumentChunks(ctx, documentID, chunks)
}

func (SQLiteKnowledgeVectorStore) Search(ctx context.Context, embeddingJSON string, limit int) ([]models.KnowledgeSearchResult, error) {
	return models.SearchKnowledgeChunks(ctx, embeddingJSON, limit)
}

func (SQLiteKnowledgeVectorStore) SearchText(ctx context.Context, terms []string, limit int) ([]models.KnowledgeSearchResult, error) {
	return models.SearchKnowledgeChunksByText(ctx, terms, limit)
}

// Search performs a KNN vector search against the kb_chunk_embedding_vec table,
// joins with kb_chunks to retrieve content and metadata, and filters by enabled sources/documents/chunks.
func (r *SQLiteVecRetriever) Search(ctx context.Context, queryEmbedding []float32, topK int, minScore float64) ([]kb.Chunk, error) {
	if topK <= 0 {
		topK = 5
	}

	embeddingJSON, err := json.Marshal(normalizeEmbeddingVector(queryEmbedding, sqliteVecEmbeddingDimensions))
	if err != nil {
		return nil, fmt.Errorf("marshal query embedding: %w", err)
	}

	results, err := r.store.Search(ctx, string(embeddingJSON), topK)
	if err != nil {
		return nil, fmt.Errorf("search knowledge chunks: %w", err)
	}

	return knowledgeSearchResultsToChunks(results, minScore), nil
}

func (r *SQLiteVecRetriever) SearchHybrid(ctx context.Context, query string, queryEmbedding []float32, topK int, minScore float64) ([]kb.Chunk, error) {
	chunks, err := r.Search(ctx, queryEmbedding, topK, minScore)
	if err != nil || len(chunks) > 0 {
		return chunks, err
	}

	terms := kbLexicalSearchTerms(query)
	if len(terms) == 0 {
		return chunks, nil
	}
	results, err := r.store.SearchText(ctx, terms, topK)
	if err != nil {
		return nil, fmt.Errorf("search knowledge chunks by text: %w", err)
	}
	return knowledgeSearchResultsToChunks(results, 0), nil
}

func knowledgeSearchResultsToChunks(results []models.KnowledgeSearchResult, minScore float64) []kb.Chunk {
	chunks := make([]kb.Chunk, 0, len(results))
	for _, result := range results {
		if minScore > 0 && result.Distance > minScore {
			continue
		}
		chunks = append(chunks, kb.Chunk{
			ChunkID:    result.ChunkID,
			SourceID:   result.SourceID,
			DocumentID: result.DocumentID,
			SourceName: result.Title,
			Content:    result.Content,
			Score:      result.Distance,
		})
	}
	return chunks
}

var kbQuestionPhrases = []string{
	"是干什么的",
	"是做什么的",
	"是干啥的",
	"是啥",
	"是什么",
	"成立于哪一年",
	"成立于哪年",
	"哪一年成立",
	"哪年成立",
	"什么时候成立",
	"有哪些业务",
	"有什么业务",
	"业务有哪些",
	"业务是什么",
	"干什么",
	"做什么",
	"介绍一下",
	"介绍下",
	"了解一下",
	"告诉我",
}

var kbQuestionPrefixes = []string{
	"请问",
	"我想知道",
	"想知道",
	"帮我看看",
	"帮我查一下",
	"帮我查查",
}

var kbEntitySuffixes = []string{
	"这家公司",
	"这个公司",
	"这家机构",
	"这个机构",
	"这家集团",
	"这个集团",
	"公司",
	"机构",
	"集团",
}

func kbLexicalSearchTerms(query string) []string {
	cleaned := normalizeKBSearchText(query)
	if cleaned == "" {
		return nil
	}

	var terms []string
	add := func(term string) {
		term = normalizeKBSearchText(term)
		term = trimKBQuestionPrefix(term)
		term = trimKBEntitySuffix(term)
		term = strings.TrimSpace(term)
		if !isUsefulKBSearchTerm(term) {
			return
		}
		for _, existing := range terms {
			if existing == term {
				return
			}
		}
		terms = append(terms, term)
	}

	for _, phrase := range kbQuestionPhrases {
		if idx := strings.Index(cleaned, phrase); idx > 0 {
			add(cleaned[:idx])
		}
	}
	add(cleaned)

	return terms
}

func normalizeKBSearchText(value string) string {
	return strings.TrimFunc(strings.TrimSpace(value), func(r rune) bool {
		return unicode.IsPunct(r) || unicode.IsSpace(r) || unicode.IsSymbol(r)
	})
}

func trimKBQuestionPrefix(value string) string {
	for _, prefix := range kbQuestionPrefixes {
		value = strings.TrimPrefix(value, prefix)
	}
	return value
}

func trimKBEntitySuffix(value string) string {
	for _, suffix := range kbEntitySuffixes {
		value = strings.TrimSuffix(value, suffix)
	}
	return value
}

func isUsefulKBSearchTerm(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	if len([]rune(value)) < 2 {
		return false
	}
	return true
}

// IndexDocument chunks a document's content, generates embeddings, and stores them.
// It uses the DefaultEmbeddingProvider config pattern, following the LLM config loading approach.
func (r *SQLiteVecRetriever) IndexDocument(ctx context.Context, documentID string) error {
	// Delegate to the full IndexDocument usecase flow.
	// We create a fwusecase context from the provided context and call IndexDocument.
	// This ensures the full lifecycle (status updates, chunking, embedding, storing) is executed.
	return indexDocumentWithStore(ctx, documentID, r.store)
}

// indexDocumentInternal is the internal implementation that performs the actual indexing.
// It is called by both the async task handler and the synchronous fallback.
func indexDocumentInternal(ctx context.Context, documentID string) error {
	return indexDocumentWithStore(ctx, documentID, NewDefaultKnowledgeVectorStore())
}

func indexDocumentWithStore(ctx context.Context, documentID string, store KnowledgeVectorStore) error {
	if store == nil {
		store = NewDefaultKnowledgeVectorStore()
	}

	doc, err := models.GetKBDocumentByID(ctx, documentID)
	if err != nil {
		return fmt.Errorf("load document for indexing: %w", err)
	}

	// Set status to processing
	if err := models.UpdateKnowledgeIndexStatus(ctx, doc.SourceID, documentID, models.KBIndexStatusProcessing, ""); err != nil {
		return fmt.Errorf("set document status to processing: %w", err)
	}

	// Determine content to index
	content := doc.Content
	if content == "" {
		content = doc.ExtractedText
	}
	if content == "" {
		if err := models.UpdateKnowledgeIndexStatus(ctx, doc.SourceID, documentID, models.KBIndexStatusFailed, "document has no content"); err != nil {
			return fmt.Errorf("set document status to failed: %w", err)
		}
		return fmt.Errorf("document %s has no content: %w", documentID, errKBDocumentHasNoContent)
	}

	// Chunk the content
	chunker := &SimpleChunker{MaxTokens: 500}
	chunks := chunker.Chunk(content)
	if len(chunks) == 0 {
		if err := models.UpdateKnowledgeIndexStatus(ctx, doc.SourceID, documentID, models.KBIndexStatusFailed, "no chunks produced from document"); err != nil {
			return fmt.Errorf("set document status to failed: %w", err)
		}
		return fmt.Errorf("document %s produced no chunks: %w", documentID, errKBDocumentHasNoChunks)
	}

	// Load embedding config
	embedCfg, err := cache.Cached(ctx, "config.embedding", "enabled:embed", 5*time.Minute, func() (models.IntegrationEmbeddingConfig, error) {
		return models.GetEnabledEmbeddingConfig(ctx, models.EmbeddingConfigQuery{
			Scenario:  models.IntegrationScenarioEmbedding,
			Operation: embeddingOperationCreate,
		})
	})
	if err != nil {
		setIndexError := models.UpdateKnowledgeIndexStatus(ctx, doc.SourceID, documentID, models.KBIndexStatusFailed,
			fmt.Sprintf("embedding config missing: %v - configure an embedding provider in Parameter > Embedding (scenario=embedding, operation=embedding_create)", err))
		if setIndexError != nil {
			return fmt.Errorf("set document status to failed: %w (original: %v)", setIndexError, err)
		}
		return fmt.Errorf("load embedding config: %w", err)
	}

	// Resolve the runtime through the same fallback path used by support chat so
	// an unconfigured external provider can still index with the local adapter.
	embedRuntime, err := supportChatEmbeddingRuntime(ctx, embedCfg)
	if err != nil {
		setIndexError := models.UpdateKnowledgeIndexStatus(ctx, doc.SourceID, documentID, models.KBIndexStatusFailed,
			fmt.Sprintf("embedding provider config invalid: %v", err))
		if setIndexError != nil {
			return fmt.Errorf("set document status to failed: %w (original: %v)", setIndexError, err)
		}
		return fmt.Errorf("resolve embedding runtime: %w", err)
	}
	embedCfg = embedRuntime.Config
	adapter := embedRuntime.Adapter
	providerCfg := embedRuntime.ProviderConfig

	// Collect chunk texts for batch embedding
	chunkTexts := make([]string, len(chunks))
	for i, c := range chunks {
		chunkTexts[i] = c.Content
	}

	// Generate embeddings
	embedResult, err := adapter.Embed(ctx, providerCfg, embedding.EmbedRequest{
		Operation: embeddingOperationCreate,
		Texts:     chunkTexts,
		Params:    providerCfg.ModelSettings,
	})
	if err != nil {
		setIndexError := models.UpdateKnowledgeIndexStatus(ctx, doc.SourceID, documentID, models.KBIndexStatusFailed,
			fmt.Sprintf("embedding generation failed: %v", err))
		if setIndexError != nil {
			return fmt.Errorf("set document status to failed: %w (original: %v)", setIndexError, err)
		}
		return fmt.Errorf("generate embeddings: %w", err)
	}

	if len(embedResult.Vectors) != len(chunks) {
		setIndexError := models.UpdateKnowledgeIndexStatus(ctx, doc.SourceID, documentID, models.KBIndexStatusFailed,
			fmt.Sprintf("embedding result count mismatch: got %d vectors for %d chunks", len(embedResult.Vectors), len(chunks)))
		if setIndexError != nil {
			return fmt.Errorf("set document status to failed: %w (original mismatch)", setIndexError)
		}
		return fmt.Errorf("embedding result count mismatch: got %d, want %d", len(embedResult.Vectors), len(chunks))
	}

	// Prepare chunks for storage
	modelInserts := make([]models.KnowledgeChunkInsert, len(chunks))
	embeddingModelCode := embedResult.ModelCode
	if embeddingModelCode == "" {
		embeddingModelCode = embedCfg.Model.ModelCode
	}
	embeddingProviderModelID := embedResult.ProviderModelID
	if embeddingProviderModelID == "" {
		embeddingProviderModelID = embedCfg.Model.ProviderModelID
	}

	for i, c := range chunks {
		embeddingValues := normalizeEmbeddingVector(embedResult.Vectors[i].Values, sqliteVecEmbeddingDimensions)
		embeddingJSON, err := json.Marshal(embeddingValues)
		if err != nil {
			return fmt.Errorf("marshal embedding for chunk %d: %w", i, err)
		}

		modelInserts[i] = models.KnowledgeChunkInsert{
			ID:                       c.ID,
			SourceID:                 doc.SourceID,
			DocumentID:               documentID,
			ChunkIndex:               i,
			Content:                  c.Content,
			ContentHash:              c.ContentHash,
			TokenCount:               c.TokenCount,
			CharCount:                c.CharCount,
			EmbeddingModelCode:       embeddingModelCode,
			EmbeddingProviderModelID: embeddingProviderModelID,
			EmbeddingDimensions:      len(embeddingValues),
			EmbeddingJSON:            string(embeddingJSON),
		}
	}

	// Store chunks and embeddings in the database
	if err := store.ReplaceDocumentChunks(ctx, documentID, modelInserts); err != nil {
		setIndexError := models.UpdateKnowledgeIndexStatus(ctx, doc.SourceID, documentID, models.KBIndexStatusFailed,
			fmt.Sprintf("store chunks failed: %v", err))
		if setIndexError != nil {
			return fmt.Errorf("set document status to failed: %w (original: %v)", setIndexError, err)
		}
		return fmt.Errorf("store chunks: %w", err)
	}

	// Also update the source's index status
	if err := models.UpdateKnowledgeIndexStatus(ctx, doc.SourceID, documentID, models.KBIndexStatusIndexed, ""); err != nil {
		return fmt.Errorf("update source index status: %w", err)
	}

	return nil
}

func normalizeEmbeddingVector(values []float32, dimensions int) []float32 {
	if dimensions <= 0 {
		dimensions = sqliteVecEmbeddingDimensions
	}
	normalized := make([]float32, dimensions)
	copy(normalized, values)
	return normalized
}

// Ensure SQLiteVecRetriever implements both interfaces.
var (
	_ kb.Retriever = (*SQLiteVecRetriever)(nil)
	_ kb.Indexer   = (*SQLiteVecRetriever)(nil)
)

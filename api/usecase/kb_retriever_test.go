package usecase_test

import (
	"context"
	"encoding/json"
	"testing"

	fwusecase "github.com/tfnick/go-svelte-starter/api/framework/usecase"
	"github.com/tfnick/go-svelte-starter/api/models"
	"github.com/tfnick/go-svelte-starter/api/usecase"
)

type fakeKnowledgeVectorStore struct {
	replaced    bool
	documentID  string
	chunkCount  int
	firstVector string
	searched    bool
	limit       int
}

func (s *fakeKnowledgeVectorStore) ReplaceDocumentChunks(_ context.Context, documentID string, chunks []models.KnowledgeChunkInsert) error {
	s.replaced = true
	s.documentID = documentID
	s.chunkCount = len(chunks)
	if len(chunks) > 0 {
		s.firstVector = chunks[0].EmbeddingJSON
	}
	return nil
}

func (s *fakeKnowledgeVectorStore) Search(_ context.Context, _ string, limit int) ([]models.KnowledgeSearchResult, error) {
	s.searched = true
	s.limit = limit
	return []models.KnowledgeSearchResult{{
		ChunkID:    "chunk-1",
		SourceID:   "source-1",
		DocumentID: "document-1",
		Title:      "Doc",
		Content:    "Content",
		Distance:   0.1,
	}}, nil
}

func (s *fakeKnowledgeVectorStore) SearchText(context.Context, []string, int) ([]models.KnowledgeSearchResult, error) {
	return nil, nil
}

func TestKnowledgeRetrieverUsesInjectedVectorStore(t *testing.T) {
	store := &fakeKnowledgeVectorStore{}
	retriever := usecase.NewKnowledgeRetriever(store)

	chunks, err := retriever.Search(t.Context(), []float32{1, 2, 3}, 7, 0)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if !store.searched || store.limit != 7 {
		t.Fatalf("vector store was not used: %#v", store)
	}
	if len(chunks) != 1 || chunks[0].ChunkID != "chunk-1" {
		t.Fatalf("unexpected chunks: %#v", chunks)
	}
}

func TestKnowledgeRetrieverUsesInjectedVectorStoreForIndexing(t *testing.T) {
	manager := setupUsecaseOrderTxDB(t)
	appDB, err := manager.GetDB("app")
	if err != nil {
		t.Fatalf("get app db: %v", err)
	}
	seedEmbeddingChannelOnlyConfig(t, appDB, "kb-injected-store-embedding", "kb-injected-store-api-key")
	seedEmbeddingModelOption(t, appDB, "kb-injected-store-embedding", "qwen3-embedding-4b", "Qwen/Qwen3-Embedding-4B")

	adapterKey := "embedding.siliconflow.test.injected-store"
	if err := usecase.RegisterEmbeddingAdapter(adapterKey, shortEmbeddingAdapter{t: t}); err != nil {
		t.Fatalf("register short embedding adapter: %v", err)
	}
	if _, err := appDB.Exec(`
		UPDATE integration_channels
		SET adapter_key = ?
		WHERE scenario = 'embedding' AND channel_code = 'kb-injected-store-embedding'
	`, adapterKey); err != nil {
		t.Fatalf("isolate injected store embedding adapter key: %v", err)
	}

	source, err := models.CreateKnowledgeSource(t.Context(), models.SaveKnowledgeSourceCmd{
		ID:         "kb-injected-store-source",
		Title:      "Injected Store Source",
		SourceType: models.KBSourceTypeManual,
		Enabled:    true,
		Content:    "Indexing should call the injected vector store so storage can be replaced by dialect.",
	})
	if err != nil {
		t.Fatalf("create knowledge source: %v", err)
	}

	store := &fakeKnowledgeVectorStore{}
	retriever := usecase.NewKnowledgeRetriever(store)
	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	if err := retriever.IndexDocument(ctx.Std(), source.DocumentID); err != nil {
		t.Fatalf("index document through injected store: %v", err)
	}

	if !store.replaced || store.documentID != source.DocumentID || store.chunkCount == 0 {
		t.Fatalf("expected injected store to replace document chunks, got %#v", store)
	}
	var values []float32
	if err := json.Unmarshal([]byte(store.firstVector), &values); err != nil {
		t.Fatalf("unmarshal injected store vector: %v", err)
	}
	if len(values) != 64 {
		t.Fatalf("expected normalized vector passed to injected store, got %d dimensions", len(values))
	}
}

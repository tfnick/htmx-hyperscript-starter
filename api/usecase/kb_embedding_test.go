package usecase_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	fwusecase "github.com/tfnick/go-svelte-starter/api/framework/usecase"
	"github.com/tfnick/go-svelte-starter/api/models"
	"github.com/tfnick/go-svelte-starter/api/providers/embedding/localhash"
	"github.com/tfnick/go-svelte-starter/api/usecase"
	"github.com/tfnick/go-svelte-starter/api/usecase/integrations/embedding"
	"github.com/tfnick/go-svelte-starter/api/usecase/integrations/llm"
	"github.com/tfnick/sqlx"
)

type fakeEmbeddingAdapter struct {
	t *testing.T
}

func (a fakeEmbeddingAdapter) Embed(_ context.Context, cfg embedding.ProviderConfig, req embedding.EmbedRequest) (embedding.EmbedResult, error) {
	a.t.Helper()
	if cfg.ChannelCode != "kb-embedding-channel-only" {
		a.t.Fatalf("unexpected embedding channel code: %s", cfg.ChannelCode)
	}
	if cfg.ModelCode != "qwen3-embedding-4b" || cfg.ProviderModelID != "Qwen/Qwen3-Embedding-4B" {
		a.t.Fatalf("unexpected embedding model mapping: %#v", cfg)
	}
	if cfg.ProviderSettings["endpoint_path"] != "/v1/embeddings" {
		a.t.Fatalf("expected channel embedding endpoint settings, got %#v", cfg.ProviderSettings)
	}
	dimensions, ok := req.Params["dimensions"].(int)
	if !ok || dimensions != 64 || req.Params["encoding_format"] != "float" {
		a.t.Fatalf("expected dimensions=64 from model defaults, got %#v", req.Params)
	}
	if len(req.Texts) == 0 {
		a.t.Fatalf("expected chunk texts in embedding request")
	}

	vectors := make([]embedding.Vector, len(req.Texts))
	for i := range req.Texts {
		values := make([]float32, 64)
		values[i%len(values)] = 1
		vectors[i] = embedding.Vector{Values: values}
	}

	return embedding.EmbedResult{
		Vectors:         vectors,
		ModelCode:       cfg.ModelCode,
		ProviderModelID: cfg.ProviderModelID,
		Dimensions:      64,
		Usage:           embedding.Usage{PromptTokens: len(req.Texts), TotalTokens: len(req.Texts)},
	}, nil
}

type shortEmbeddingAdapter struct {
	t *testing.T
}

func (a shortEmbeddingAdapter) Embed(_ context.Context, cfg embedding.ProviderConfig, req embedding.EmbedRequest) (embedding.EmbedResult, error) {
	a.t.Helper()
	if len(req.Texts) == 0 {
		a.t.Fatalf("expected chunk texts in embedding request")
	}

	vectors := make([]embedding.Vector, len(req.Texts))
	for i := range req.Texts {
		vectors[i] = embedding.Vector{Values: []float32{1, 0}}
	}

	return embedding.EmbedResult{
		Vectors:         vectors,
		ModelCode:       cfg.ModelCode,
		ProviderModelID: cfg.ProviderModelID,
		Dimensions:      2,
		Usage:           embedding.Usage{PromptTokens: len(req.Texts), TotalTokens: len(req.Texts)},
	}, nil
}

type supportChatLLMAdapter struct {
	t *testing.T
}

func (a supportChatLLMAdapter) Generate(_ context.Context, cfg llm.ProviderConfig, req llm.GenerateRequest) (llm.GenerateResult, error) {
	a.t.Helper()
	if cfg.ChannelCode != "kb-chat-llm" || cfg.ModelCode != "deepseek-chat" {
		a.t.Fatalf("unexpected LLM config: %#v", cfg)
	}
	if len(req.Messages) < 2 || !strings.Contains(req.Messages[1].Content, "Local fallback answer") {
		a.t.Fatalf("expected retrieved KB context in LLM prompt: %#v", req.Messages)
	}
	return llm.GenerateResult{
		Content:           "Local fallback answer",
		ProviderRequestID: "kb-chat-llm-request",
		Usage:             llm.Usage{PromptTokens: 10, CompletionTokens: 3, TotalTokens: 13},
	}, nil
}

type supportChatConfiguredEmbeddingAdapter struct {
	t *testing.T
}

func (a supportChatConfiguredEmbeddingAdapter) Embed(_ context.Context, cfg embedding.ProviderConfig, req embedding.EmbedRequest) (embedding.EmbedResult, error) {
	a.t.Helper()
	if cfg.ChannelCode != "kb-chat-configured-embedding" {
		a.t.Fatalf("unexpected embedding channel code: %s", cfg.ChannelCode)
	}
	if cfg.ModelCode != "qwen3-embedding-4b" || cfg.ProviderModelID != "Qwen/Qwen3-Embedding-4B" {
		a.t.Fatalf("unexpected embedding model mapping: %#v", cfg)
	}
	dimensions, ok := req.Params["dimensions"].(int)
	if !ok || dimensions != 64 || req.Params["encoding_format"] != "float" {
		a.t.Fatalf("expected configured chat embedding dimensions=64, got %#v", req.Params)
	}
	if len(req.Texts) == 0 {
		a.t.Fatalf("expected embedding texts")
	}

	vectors := make([]embedding.Vector, len(req.Texts))
	for i := range req.Texts {
		values := make([]float32, 64)
		values[0] = 1
		vectors[i] = embedding.Vector{Values: values}
	}

	return embedding.EmbedResult{
		Vectors:         vectors,
		ModelCode:       cfg.ModelCode,
		ProviderModelID: cfg.ProviderModelID,
		Dimensions:      64,
		Usage:           embedding.Usage{PromptTokens: len(req.Texts), TotalTokens: len(req.Texts)},
	}, nil
}

type supportChatConfiguredLLMAdapter struct {
	t *testing.T
}

func (a supportChatConfiguredLLMAdapter) Generate(_ context.Context, cfg llm.ProviderConfig, req llm.GenerateRequest) (llm.GenerateResult, error) {
	a.t.Helper()
	if cfg.ChannelCode != "kb-chat-configured-llm" || cfg.ModelCode != "deepseek-chat" {
		a.t.Fatalf("unexpected LLM config: %#v", cfg)
	}
	if len(req.Messages) < 2 || !strings.Contains(req.Messages[1].Content, "Pigsty is a PostgreSQL distribution") {
		a.t.Fatalf("expected Pigsty KB context in LLM prompt: %#v", req.Messages)
	}
	return llm.GenerateResult{
		Content:           "Pigsty is a PostgreSQL distribution and observability stack.",
		ProviderRequestID: "kb-chat-configured-llm-request",
		Usage:             llm.Usage{PromptTokens: 10, CompletionTokens: 8, TotalTokens: 18},
	}, nil
}

type supportChatLexicalFallbackEmbeddingAdapter struct {
	t *testing.T
}

func (a supportChatLexicalFallbackEmbeddingAdapter) Embed(_ context.Context, cfg embedding.ProviderConfig, req embedding.EmbedRequest) (embedding.EmbedResult, error) {
	a.t.Helper()
	if cfg.ChannelCode != "kb-chat-lexical-embedding" {
		a.t.Fatalf("unexpected lexical fallback embedding channel code: %s", cfg.ChannelCode)
	}
	if cfg.ModelCode != "qwen3-embedding-4b" || cfg.ProviderModelID != "Qwen/Qwen3-Embedding-4B" {
		a.t.Fatalf("unexpected lexical fallback embedding model mapping: %#v", cfg)
	}
	if len(req.Texts) == 0 {
		a.t.Fatalf("expected embedding texts")
	}

	vectors := make([]embedding.Vector, len(req.Texts))
	for i, text := range req.Texts {
		values := make([]float32, 64)
		if strings.Contains(text, "教育集团") {
			values[0] = 1
		} else {
			values[1] = 1
		}
		vectors[i] = embedding.Vector{Values: values}
	}

	return embedding.EmbedResult{
		Vectors:         vectors,
		ModelCode:       cfg.ModelCode,
		ProviderModelID: cfg.ProviderModelID,
		Dimensions:      64,
		Usage:           embedding.Usage{PromptTokens: len(req.Texts), TotalTokens: len(req.Texts)},
	}, nil
}

type supportChatChineseEntityLLMAdapter struct {
	t *testing.T
}

func (a supportChatChineseEntityLLMAdapter) Generate(_ context.Context, cfg llm.ProviderConfig, req llm.GenerateRequest) (llm.GenerateResult, error) {
	a.t.Helper()
	if cfg.ChannelCode != "kb-chat-lexical-llm" || cfg.ModelCode != "deepseek-chat" {
		a.t.Fatalf("unexpected LLM config: %#v", cfg)
	}
	if len(req.Messages) < 2 ||
		!strings.Contains(req.Messages[1].Content, "桃李未来") ||
		!strings.Contains(req.Messages[1].Content, "教育集团") {
		a.t.Fatalf("expected Chinese entity KB context in LLM prompt: %#v", req.Messages)
	}
	return llm.GenerateResult{
		Content:           "桃李未来是一家位于深圳的教育集团。",
		ProviderRequestID: "kb-chat-lexical-llm-request",
		Usage:             llm.Usage{PromptTokens: 12, CompletionTokens: 6, TotalTokens: 18},
	}, nil
}

type supportChatChineseEntityFactsLLMAdapter struct {
	t *testing.T
}

func (a supportChatChineseEntityFactsLLMAdapter) Generate(_ context.Context, cfg llm.ProviderConfig, req llm.GenerateRequest) (llm.GenerateResult, error) {
	a.t.Helper()
	if cfg.ChannelCode != "kb-chat-lexical-llm" || cfg.ModelCode != "deepseek-chat" {
		a.t.Fatalf("unexpected LLM config: %#v", cfg)
	}
	if len(req.Messages) < 2 ||
		!strings.Contains(req.Messages[1].Content, "桃李未来") ||
		!strings.Contains(req.Messages[1].Content, "教育集团") ||
		!strings.Contains(req.Messages[1].Content, "2018年") ||
		!strings.Contains(req.Messages[1].Content, "22家校区") ||
		!strings.Contains(req.Messages[1].Content, "业务线") {
		a.t.Fatalf("expected Chinese entity facts in LLM prompt: %#v", req.Messages)
	}

	userPrompt := req.Messages[1].Content
	question := userPrompt
	if idx := strings.LastIndex(userPrompt, "Question:"); idx >= 0 {
		question = userPrompt[idx+len("Question:"):]
	}
	content := "桃李未来是一家位于深圳的教育集团。"
	if strings.Contains(question, "成立") {
		content = "桃李未来成立于2018年。"
	}
	if strings.Contains(question, "业务") {
		content = "桃李未来拥有桃李培优、桃李1对1、桃李国际等业务线。"
	}
	if strings.Contains(question, "校区") {
		content = "桃李未来有22家校区。"
	}

	return llm.GenerateResult{
		Content:           content,
		ProviderRequestID: "kb-chat-lexical-llm-request",
		Usage:             llm.Usage{PromptTokens: 20, CompletionTokens: 8, TotalTokens: 28},
	}, nil
}

func TestIndexDocumentUsesEmbeddingChannelOnlyConfig(t *testing.T) {
	manager := setupUsecaseOrderTxDB(t)
	appDB, err := manager.GetDB("app")
	if err != nil {
		t.Fatalf("get app db: %v", err)
	}
	seedEmbeddingChannelOnlyConfig(t, appDB, "kb-embedding-channel-only", "kb-embedding-api-key")

	if err := usecase.RegisterEmbeddingAdapter("embedding.siliconflow.openai_compatible", fakeEmbeddingAdapter{t: t}); err != nil {
		t.Fatalf("register fake embedding adapter: %v", err)
	}

	source, err := models.CreateKnowledgeSource(t.Context(), models.SaveKnowledgeSourceCmd{
		ID:         "kb-embedding-source",
		Title:      "Embedding Source",
		SourceType: models.KBSourceTypeManual,
		Enabled:    true,
		Content:    "Embedding configuration should work after only creating a channel in Parameter.",
	})
	if err != nil {
		t.Fatalf("create knowledge source: %v", err)
	}

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	if err := usecase.IndexDocument(ctx, usecase.IndexDocumentCmd{DocumentID: source.DocumentID}); err != nil {
		t.Fatalf("index document: %v", err)
	}

	doc, err := models.GetKBDocumentByID(t.Context(), source.DocumentID)
	if err != nil {
		t.Fatalf("load indexed document: %v", err)
	}
	if doc.IndexStatus != models.KBIndexStatusIndexed || doc.LastIndexError != "" {
		t.Fatalf("expected indexed document without error, got %#v", doc)
	}

	var chunk struct {
		EmbeddingModelCode       string `db:"embedding_model_code"`
		EmbeddingProviderModelID string `db:"embedding_provider_model_id"`
		EmbeddingDimensions      int    `db:"embedding_dimensions"`
	}
	if err := appDB.Get(&chunk, `
		SELECT embedding_model_code, embedding_provider_model_id, embedding_dimensions
		FROM kb_chunks
		WHERE document_id = ?
		LIMIT 1`, source.DocumentID); err != nil {
		t.Fatalf("load chunk: %v", err)
	}
	if chunk.EmbeddingModelCode != "qwen3-embedding-4b" || chunk.EmbeddingProviderModelID != "Qwen/Qwen3-Embedding-4B" || chunk.EmbeddingDimensions != 64 {
		t.Fatalf("unexpected chunk embedding metadata: %#v", chunk)
	}
}

func TestIndexDocumentNormalizesProviderEmbeddingDimensions(t *testing.T) {
	manager := setupUsecaseOrderTxDB(t)
	appDB, err := manager.GetDB("app")
	if err != nil {
		t.Fatalf("get app db: %v", err)
	}
	seedEmbeddingChannelOnlyConfig(t, appDB, "kb-short-embedding", "kb-short-embedding-api-key")
	seedEmbeddingModelOption(t, appDB, "kb-short-embedding", "qwen3-embedding-4b", "Qwen/Qwen3-Embedding-4B")

	adapterKey := "embedding.siliconflow.test.short-dimensions"
	if err := usecase.RegisterEmbeddingAdapter(adapterKey, shortEmbeddingAdapter{t: t}); err != nil {
		t.Fatalf("register short embedding adapter: %v", err)
	}
	if _, err := appDB.Exec(`
		UPDATE integration_channels
		SET adapter_key = ?
		WHERE scenario = 'embedding' AND channel_code = 'kb-short-embedding'
	`, adapterKey); err != nil {
		t.Fatalf("isolate short embedding adapter key: %v", err)
	}

	source, err := models.CreateKnowledgeSource(t.Context(), models.SaveKnowledgeSourceCmd{
		ID:         "kb-short-embedding-source",
		Title:      "Short Embedding Source",
		SourceType: models.KBSourceTypeManual,
		Enabled:    true,
		Content:    "Provider embeddings may not honor the requested dimensions, but sqlite-vec storage is fixed width.",
	})
	if err != nil {
		t.Fatalf("create knowledge source: %v", err)
	}

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	if err := usecase.IndexDocument(ctx, usecase.IndexDocumentCmd{DocumentID: source.DocumentID}); err != nil {
		doc, loadErr := models.GetKBDocumentByID(t.Context(), source.DocumentID)
		if loadErr != nil {
			t.Fatalf("index document: %v; load failed document: %v", err, loadErr)
		}
		t.Fatalf("index document: %v; last index error: %s", err, doc.LastIndexError)
	}

	var stored struct {
		ChunkDimensions     int    `db:"chunk_dimensions"`
		EmbeddingDimensions int    `db:"embedding_dimensions"`
		EmbeddingJSON       string `db:"embedding_json"`
	}
	if err := appDB.Get(&stored, `
		SELECT c.embedding_dimensions AS chunk_dimensions,
		       e.dimensions AS embedding_dimensions,
		       e.embedding_json
		FROM kb_chunks c
		JOIN kb_chunk_embeddings e ON e.chunk_id = c.id
		WHERE c.document_id = ?
		LIMIT 1`, source.DocumentID); err != nil {
		t.Fatalf("load stored embedding: %v", err)
	}
	if stored.ChunkDimensions != 64 || stored.EmbeddingDimensions != 64 {
		t.Fatalf("expected normalized 64-dimension metadata, got %#v", stored)
	}
	var values []float32
	if err := json.Unmarshal([]byte(stored.EmbeddingJSON), &values); err != nil {
		t.Fatalf("unmarshal stored embedding json: %v", err)
	}
	if len(values) != 64 || values[0] != 1 || values[1] != 0 || values[63] != 0 {
		t.Fatalf("expected padded 64-dimension embedding, got len=%d values=%#v", len(values), values)
	}

	query := []float32{1, 0}
	if _, err := usecase.NewSQLiteVecRetriever().Search(t.Context(), query, 5, 0); err != nil {
		t.Fatalf("search with short query embedding: %v", err)
	}
}

func TestIndexDocumentUsesDefaultLocalHashEmbeddingConfig(t *testing.T) {
	manager := setupUsecaseOrderTxDB(t)
	appDB, err := manager.GetDB("app")
	if err != nil {
		t.Fatalf("get app db: %v", err)
	}
	adapterKey := "embedding.local_hash_64.test.default"
	if err := usecase.RegisterEmbeddingAdapter(adapterKey, localhash.NewAdapter(64)); err != nil {
		t.Fatalf("register local embedding adapter: %v", err)
	}
	if _, err := appDB.Exec(`
		UPDATE integration_channels
		SET adapter_key = ?
		WHERE scenario = 'embedding' AND channel_code = 'local-hash-64'
	`, adapterKey); err != nil {
		t.Fatalf("isolate local embedding adapter key: %v", err)
	}
	if _, err := appDB.Exec(`
		UPDATE integration_operation_configs
		SET channel_code = 'local-hash-64', model_code = 'local-hash-64'
		WHERE scenario = 'embedding' AND operation = 'embedding_create'
	`); err != nil {
		t.Fatalf("select local embedding operation config: %v", err)
	}

	source, err := models.CreateKnowledgeSource(t.Context(), models.SaveKnowledgeSourceCmd{
		ID:         "kb-default-local-embedding-source",
		Title:      "Default Local Embedding Source",
		SourceType: models.KBSourceTypeManual,
		Enabled:    true,
		Content:    "Default local hash embedding should index without calling an external provider.",
	})
	if err != nil {
		t.Fatalf("create knowledge source: %v", err)
	}

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	if err := usecase.IndexDocument(ctx, usecase.IndexDocumentCmd{DocumentID: source.DocumentID}); err != nil {
		doc, loadErr := models.GetKBDocumentByID(t.Context(), source.DocumentID)
		if loadErr != nil {
			t.Fatalf("index document: %v; load failed document: %v", err, loadErr)
		}
		t.Fatalf("index document: %v; last index error: %s", err, doc.LastIndexError)
	}

	doc, err := models.GetKBDocumentByID(t.Context(), source.DocumentID)
	if err != nil {
		t.Fatalf("load indexed document: %v", err)
	}
	if doc.IndexStatus != models.KBIndexStatusIndexed || doc.LastIndexError != "" {
		t.Fatalf("expected indexed document without error, got %#v", doc)
	}

	var chunk struct {
		EmbeddingModelCode       string `db:"embedding_model_code"`
		EmbeddingProviderModelID string `db:"embedding_provider_model_id"`
		EmbeddingDimensions      int    `db:"embedding_dimensions"`
	}
	if err := appDB.Get(&chunk, `
		SELECT embedding_model_code, embedding_provider_model_id, embedding_dimensions
		FROM kb_chunks
		WHERE document_id = ?
		LIMIT 1`, source.DocumentID); err != nil {
		t.Fatalf("load chunk: %v", err)
	}
	if chunk.EmbeddingModelCode != "local-hash-64" || chunk.EmbeddingProviderModelID != "local-hash-64" || chunk.EmbeddingDimensions != 64 {
		t.Fatalf("unexpected chunk embedding metadata: %#v", chunk)
	}
}

func TestIndexDocumentFallsBackToLocalHashWhenDefaultEmbeddingCredentialIsEmpty(t *testing.T) {
	manager := setupUsecaseOrderTxDB(t)
	appDB, err := manager.GetDB("app")
	if err != nil {
		t.Fatalf("get app db: %v", err)
	}
	adapterKey := "embedding.local_hash_64.test.index-fallback"
	if err := usecase.RegisterEmbeddingAdapter(adapterKey, localhash.NewAdapter(64)); err != nil {
		t.Fatalf("register local embedding adapter: %v", err)
	}
	externalAdapterKey := "embedding.external.test.index-fallback"
	if err := usecase.RegisterEmbeddingAdapter(externalAdapterKey, localhash.NewAdapter(64)); err != nil {
		t.Fatalf("register external embedding adapter: %v", err)
	}
	if _, err := appDB.Exec(`
		UPDATE integration_channels
		SET adapter_key = ?
		WHERE scenario = 'embedding' AND channel_code = 'local-hash-64'
	`, adapterKey); err != nil {
		t.Fatalf("isolate local embedding adapter key: %v", err)
	}
	if _, err := appDB.Exec(`
		UPDATE integration_channels
		SET enabled = 1, adapter_key = ?
		WHERE scenario = 'embedding' AND channel_code = 'siliconflow-qwen3-embedding'
	`, externalAdapterKey); err != nil {
		t.Fatalf("enable siliconflow embedding channel: %v", err)
	}
	if _, err := appDB.Exec(`
		UPDATE integration_credentials
		SET value_text = '', ciphertext = ''
		WHERE id = (
			SELECT credential_id
			FROM integration_channels
			WHERE scenario = 'embedding' AND channel_code = 'siliconflow-qwen3-embedding'
		)
	`); err != nil {
		t.Fatalf("clear siliconflow embedding credential: %v", err)
	}
	if _, err := appDB.Exec(`
		UPDATE integration_operation_configs
		SET channel_code = 'siliconflow-qwen3-embedding', model_code = 'qwen3-embedding-0.6b'
		WHERE scenario = 'embedding' AND operation = 'embedding_create'
	`); err != nil {
		t.Fatalf("select siliconflow embedding operation config: %v", err)
	}

	source, err := models.CreateKnowledgeSource(t.Context(), models.SaveKnowledgeSourceCmd{
		ID:         "kb-index-embedding-fallback-source",
		Title:      "Index Embedding Fallback Source",
		SourceType: models.KBSourceTypeManual,
		Enabled:    true,
		Content:    "Indexing should fall back to local hash when the configured embedding provider is missing credentials.",
	})
	if err != nil {
		t.Fatalf("create knowledge source: %v", err)
	}

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	if err := usecase.IndexDocument(ctx, usecase.IndexDocumentCmd{DocumentID: source.DocumentID}); err != nil {
		doc, loadErr := models.GetKBDocumentByID(t.Context(), source.DocumentID)
		if loadErr != nil {
			t.Fatalf("index document: %v; load failed document: %v", err, loadErr)
		}
		t.Fatalf("index document: %v; last index error: %s", err, doc.LastIndexError)
	}

	var chunk struct {
		EmbeddingModelCode       string `db:"embedding_model_code"`
		EmbeddingProviderModelID string `db:"embedding_provider_model_id"`
		EmbeddingDimensions      int    `db:"embedding_dimensions"`
	}
	if err := appDB.Get(&chunk, `
		SELECT embedding_model_code, embedding_provider_model_id, embedding_dimensions
		FROM kb_chunks
		WHERE document_id = ?
		LIMIT 1`, source.DocumentID); err != nil {
		t.Fatalf("load chunk: %v", err)
	}
	if chunk.EmbeddingModelCode != "local-hash-64" || chunk.EmbeddingProviderModelID != "local-hash-64" || chunk.EmbeddingDimensions != 64 {
		t.Fatalf("expected local fallback embedding metadata, got %#v", chunk)
	}
}

func TestGenerateSupportAnswerFallsBackToLocalHashWhenDefaultEmbeddingCredentialIsEmpty(t *testing.T) {
	manager := setupUsecaseOrderTxDB(t)
	appDB, err := manager.GetDB("app")
	if err != nil {
		t.Fatalf("get app db: %v", err)
	}

	embeddingAdapterKey := "embedding.local_hash_64.test.chat-fallback"
	if err := usecase.RegisterEmbeddingAdapter(embeddingAdapterKey, localhash.NewAdapter(64)); err != nil {
		t.Fatalf("register local embedding adapter: %v", err)
	}
	if _, err := appDB.Exec(`
		UPDATE integration_channels
		SET adapter_key = ?
		WHERE scenario = 'embedding' AND channel_code = 'local-hash-64'
	`, embeddingAdapterKey); err != nil {
		t.Fatalf("isolate local embedding adapter key: %v", err)
	}

	source, err := models.CreateKnowledgeSource(t.Context(), models.SaveKnowledgeSourceCmd{
		ID:         "kb-chat-fallback-source",
		Title:      "Chat Fallback Source",
		SourceType: models.KBSourceTypeManual,
		Enabled:    true,
		Content:    "Local fallback answer is available from the local hash knowledge base.",
	})
	if err != nil {
		t.Fatalf("create knowledge source: %v", err)
	}

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	if _, err := appDB.Exec(`
		UPDATE integration_operation_configs
		SET channel_code = 'local-hash-64', model_code = 'local-hash-64'
		WHERE scenario = 'embedding' AND operation = 'embedding_create'
	`); err != nil {
		t.Fatalf("select local embedding operation config for indexing: %v", err)
	}
	if err := usecase.IndexDocument(ctx, usecase.IndexDocumentCmd{DocumentID: source.DocumentID}); err != nil {
		t.Fatalf("index document with local embedding: %v", err)
	}
	if _, err := appDB.Exec(`
		UPDATE integration_operation_configs
		SET channel_code = 'siliconflow-qwen3-embedding', model_code = 'qwen3-embedding-0.6b'
		WHERE scenario = 'embedding' AND operation = 'embedding_create'
	`); err != nil {
		t.Fatalf("restore default SiliconFlow embedding operation: %v", err)
	}
	unconfiguredExternalAdapterKey := "embedding.siliconflow.test.chat-unconfigured"
	if err := usecase.RegisterEmbeddingAdapter(unconfiguredExternalAdapterKey, localhash.NewAdapter(64)); err != nil {
		t.Fatalf("register unconfigured external embedding adapter placeholder: %v", err)
	}
	if _, err := appDB.Exec(`
		UPDATE integration_channels
		SET adapter_key = ?
		WHERE scenario = 'embedding' AND channel_code = 'siliconflow-qwen3-embedding'
	`, unconfiguredExternalAdapterKey); err != nil {
		t.Fatalf("isolate unconfigured external embedding adapter key: %v", err)
	}

	appEngine, err := manager.GetEngine("app")
	if err != nil {
		t.Fatalf("get app engine: %v", err)
	}
	seedDeepSeekChannelOnlyConfig(t, appEngine, "kb-chat-llm", "kb-chat-llm-key")
	llmAdapterKey := "llm.deepseek.kb-chat-fallback"
	if err := usecase.RegisterLLMAdapter(llmAdapterKey, supportChatLLMAdapter{t: t}); err != nil {
		t.Fatalf("register LLM adapter: %v", err)
	}
	if _, err := appDB.Exec(`
		UPDATE integration_channels
		SET adapter_key = ?
		WHERE scenario = 'llm' AND channel_code = 'kb-chat-llm'
	`, llmAdapterKey); err != nil {
		t.Fatalf("isolate LLM adapter key: %v", err)
	}
	if _, err := appDB.Exec(`
		INSERT INTO integration_model_options (
			id, scenario, channel_id, model_code, provider_model_id, default_params_json, enabled
		) VALUES ('kb-chat-llm-model', 'llm', 'kb-chat-llm-channel', 'deepseek-chat', 'deepseek-chat', '{}', 1)
	`); err != nil {
		t.Fatalf("insert LLM model option for chat: %v", err)
	}
	if _, err := appDB.Exec(`
		UPDATE integration_operation_configs
		SET channel_code = 'kb-chat-llm', model_code = 'deepseek-chat'
		WHERE scenario = 'llm' AND operation = 'text_summary'
	`); err != nil {
		t.Fatalf("select LLM operation config for chat: %v", err)
	}

	conversation, err := usecase.StartSupportConversation(ctx, usecase.StartConversationCmd{
		VisitorID:  "kb-chat-fallback-visitor",
		VisitorIP:  "127.0.0.1",
		SourcePage: "/app",
	})
	if err != nil {
		t.Fatalf("start support conversation: %v", err)
	}

	response, err := usecase.GenerateSupportAnswer(ctx, usecase.SupportChatMessageCmd{
		ConversationID: conversation.ConversationID,
		Message:        "Local fallback answer is available from the local hash knowledge base.",
	})
	if err != nil {
		t.Fatalf("generate support answer: %v; cause: %v", err, fwusecase.LogErrorOf(err))
	}
	if response.Message != "Local fallback answer" || len(response.Citations) == 0 {
		t.Fatalf("expected answer with citations from local fallback, got %#v", response)
	}
}

func TestGenerateSupportAnswerUsesConfiguredExternalEmbeddingForSearch(t *testing.T) {
	manager := setupUsecaseOrderTxDB(t)
	appDB, err := manager.GetDB("app")
	if err != nil {
		t.Fatalf("get app db: %v", err)
	}

	seedEmbeddingChannelOnlyConfig(t, appDB, "kb-chat-configured-embedding", "kb-chat-configured-api-key")
	embeddingAdapterKey := "embedding.siliconflow.test.chat-configured"
	if err := usecase.RegisterEmbeddingAdapter(embeddingAdapterKey, supportChatConfiguredEmbeddingAdapter{t: t}); err != nil {
		t.Fatalf("register configured embedding adapter: %v", err)
	}
	if _, err := appDB.Exec(`
		UPDATE integration_channels
		SET adapter_key = ?
		WHERE scenario = 'embedding' AND channel_code = 'kb-chat-configured-embedding'
	`, embeddingAdapterKey); err != nil {
		t.Fatalf("isolate configured embedding adapter key: %v", err)
	}
	if _, err := appDB.Exec(`
		INSERT INTO integration_model_options (
			id, scenario, channel_id, model_code, provider_model_id, default_params_json, enabled
		) VALUES (
			'kb-chat-configured-embedding-model',
			'embedding',
			'kb-chat-configured-embedding-channel',
			'qwen3-embedding-4b',
			'Qwen/Qwen3-Embedding-4B',
			'{"dimensions":64,"encoding_format":"float"}',
			1
		)
	`); err != nil {
		t.Fatalf("insert configured embedding model option: %v", err)
	}
	if _, err := appDB.Exec(`
		UPDATE integration_operation_configs
		SET channel_code = 'kb-chat-configured-embedding', model_code = 'qwen3-embedding-4b'
		WHERE scenario = 'embedding' AND operation = 'embedding_create'
	`); err != nil {
		t.Fatalf("select configured embedding operation config: %v", err)
	}

	source, err := models.CreateKnowledgeSource(t.Context(), models.SaveKnowledgeSourceCmd{
		ID:         "kb-chat-configured-source",
		Title:      "Pigsty Source",
		SourceType: models.KBSourceTypeManual,
		Enabled:    true,
		Content:    "Pigsty is a PostgreSQL distribution and observability stack for production databases.",
	})
	if err != nil {
		t.Fatalf("create knowledge source: %v", err)
	}

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	if err := usecase.IndexDocument(ctx, usecase.IndexDocumentCmd{DocumentID: source.DocumentID}); err != nil {
		t.Fatalf("index configured external embedding document: %v", err)
	}

	appEngine, err := manager.GetEngine("app")
	if err != nil {
		t.Fatalf("get app engine: %v", err)
	}
	seedDeepSeekChannelOnlyConfig(t, appEngine, "kb-chat-configured-llm", "kb-chat-configured-llm-key")
	llmAdapterKey := "llm.deepseek.kb-chat-configured"
	if err := usecase.RegisterLLMAdapter(llmAdapterKey, supportChatConfiguredLLMAdapter{t: t}); err != nil {
		t.Fatalf("register configured LLM adapter: %v", err)
	}
	if _, err := appDB.Exec(`
		UPDATE integration_channels
		SET adapter_key = ?
		WHERE scenario = 'llm' AND channel_code = 'kb-chat-configured-llm'
	`, llmAdapterKey); err != nil {
		t.Fatalf("isolate configured LLM adapter key: %v", err)
	}
	if _, err := appDB.Exec(`
		INSERT INTO integration_model_options (
			id, scenario, channel_id, model_code, provider_model_id, default_params_json, enabled
		) VALUES ('kb-chat-configured-llm-model', 'llm', 'kb-chat-configured-llm-channel', 'deepseek-chat', 'deepseek-chat', '{}', 1)
	`); err != nil {
		t.Fatalf("insert configured LLM model option: %v", err)
	}
	if _, err := appDB.Exec(`
		UPDATE integration_operation_configs
		SET channel_code = 'kb-chat-configured-llm', model_code = 'deepseek-chat'
		WHERE scenario = 'llm' AND operation = 'text_summary'
	`); err != nil {
		t.Fatalf("select configured LLM operation config: %v", err)
	}

	conversation, err := usecase.StartSupportConversation(ctx, usecase.StartConversationCmd{
		VisitorID:  "kb-chat-configured-visitor",
		VisitorIP:  "127.0.0.1",
		SourcePage: "/app",
	})
	if err != nil {
		t.Fatalf("start configured support conversation: %v", err)
	}

	response, err := usecase.GenerateSupportAnswer(ctx, usecase.SupportChatMessageCmd{
		ConversationID: conversation.ConversationID,
		Message:        "pigsty是什么？",
	})
	if err != nil {
		t.Fatalf("generate configured support answer: %v; cause: %v", err, fwusecase.LogErrorOf(err))
	}
	if !strings.Contains(response.Message, "Pigsty") || len(response.Citations) == 0 {
		t.Fatalf("expected configured provider answer with citations, got %#v", response)
	}
}

func TestGenerateSupportAnswerUsesLexicalFallbackForChineseEntityQuestions(t *testing.T) {
	manager := setupUsecaseOrderTxDB(t)
	appDB, err := manager.GetDB("app")
	if err != nil {
		t.Fatalf("get app db: %v", err)
	}

	seedEmbeddingChannelOnlyConfig(t, appDB, "kb-chat-lexical-embedding", "kb-chat-lexical-api-key")
	embeddingAdapterKey := "embedding.siliconflow.test.chat-lexical-fallback"
	if err := usecase.RegisterEmbeddingAdapter(embeddingAdapterKey, supportChatLexicalFallbackEmbeddingAdapter{t: t}); err != nil {
		t.Fatalf("register lexical fallback embedding adapter: %v", err)
	}
	if _, err := appDB.Exec(`
		UPDATE integration_channels
		SET adapter_key = ?
		WHERE scenario = 'embedding' AND channel_code = 'kb-chat-lexical-embedding'
	`, embeddingAdapterKey); err != nil {
		t.Fatalf("isolate lexical fallback embedding adapter key: %v", err)
	}
	if _, err := appDB.Exec(`
		INSERT INTO integration_model_options (
			id, scenario, channel_id, model_code, provider_model_id, default_params_json, enabled
		) VALUES (
			'kb-chat-lexical-embedding-model',
			'embedding',
			'kb-chat-lexical-embedding-channel',
			'qwen3-embedding-4b',
			'Qwen/Qwen3-Embedding-4B',
			'{"dimensions":64,"encoding_format":"float"}',
			1
		)
	`); err != nil {
		t.Fatalf("insert lexical fallback embedding model option: %v", err)
	}
	if _, err := appDB.Exec(`
		UPDATE integration_operation_configs
		SET channel_code = 'kb-chat-lexical-embedding', model_code = 'qwen3-embedding-4b'
		WHERE scenario = 'embedding' AND operation = 'embedding_create'
	`); err != nil {
		t.Fatalf("select lexical fallback embedding operation config: %v", err)
	}

	source, err := models.CreateKnowledgeSource(t.Context(), models.SaveKnowledgeSourceCmd{
		ID:         "kb-chat-lexical-source",
		Title:      "桃李未来",
		SourceType: models.KBSourceTypeManual,
		Enabled:    true,
		Content:    "桃李未来是深圳知名的教育集团，成立于2018年，拥有8条业务线、22家校区，包括桃李培优、桃李1对1、桃李国际等。",
	})
	if err != nil {
		t.Fatalf("create Chinese entity knowledge source: %v", err)
	}

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	if err := usecase.IndexDocument(ctx, usecase.IndexDocumentCmd{DocumentID: source.DocumentID}); err != nil {
		t.Fatalf("index Chinese entity document: %v", err)
	}

	vectorOnlyQuery := make([]float32, 64)
	vectorOnlyQuery[1] = 1
	vectorOnlyChunks, err := usecase.NewSQLiteVecRetriever().Search(t.Context(), vectorOnlyQuery, 5, 0.5)
	if err != nil {
		t.Fatalf("run vector-only search: %v", err)
	}
	if len(vectorOnlyChunks) != 0 {
		t.Fatalf("expected vector-only retrieval to miss, got %#v", vectorOnlyChunks)
	}

	appEngine, err := manager.GetEngine("app")
	if err != nil {
		t.Fatalf("get app engine: %v", err)
	}
	seedDeepSeekChannelOnlyConfig(t, appEngine, "kb-chat-lexical-llm", "kb-chat-lexical-llm-key")
	llmAdapterKey := "llm.deepseek.kb-chat-lexical"
	if err := usecase.RegisterLLMAdapter(llmAdapterKey, supportChatChineseEntityFactsLLMAdapter{t: t}); err != nil {
		t.Fatalf("register lexical fallback LLM adapter: %v", err)
	}
	if _, err := appDB.Exec(`
		UPDATE integration_channels
		SET adapter_key = ?
		WHERE scenario = 'llm' AND channel_code = 'kb-chat-lexical-llm'
	`, llmAdapterKey); err != nil {
		t.Fatalf("isolate lexical fallback LLM adapter key: %v", err)
	}
	if _, err := appDB.Exec(`
		INSERT INTO integration_model_options (
			id, scenario, channel_id, model_code, provider_model_id, default_params_json, enabled
		) VALUES ('kb-chat-lexical-llm-model', 'llm', 'kb-chat-lexical-llm-channel', 'deepseek-chat', 'deepseek-chat', '{}', 1)
	`); err != nil {
		t.Fatalf("insert lexical fallback LLM model option: %v", err)
	}
	if _, err := appDB.Exec(`
		UPDATE integration_operation_configs
		SET channel_code = 'kb-chat-lexical-llm', model_code = 'deepseek-chat'
		WHERE scenario = 'llm' AND operation = 'text_summary'
	`); err != nil {
		t.Fatalf("select lexical fallback LLM operation config: %v", err)
	}

	cases := []struct {
		name       string
		question   string
		wantAnswer string
	}{
		{name: "definition", question: "桃李未来是干什么的？", wantAnswer: "教育集团"},
		{name: "founded year", question: "桃李未来成立于哪一年？", wantAnswer: "2018年"},
		{name: "business lines", question: "桃李未来有哪些业务？", wantAnswer: "业务线"},
		{name: "campus count", question: "桃李未来有多少个校区？", wantAnswer: "22家校区"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			conversation, err := usecase.StartSupportConversation(ctx, usecase.StartConversationCmd{
				VisitorID:  "kb-chat-lexical-visitor-" + tc.name,
				VisitorIP:  "127.0.0.1",
				SourcePage: "/app",
			})
			if err != nil {
				t.Fatalf("start lexical fallback support conversation: %v", err)
			}

			response, err := usecase.GenerateSupportAnswer(ctx, usecase.SupportChatMessageCmd{
				ConversationID: conversation.ConversationID,
				Message:        tc.question,
			})
			if err != nil {
				t.Fatalf("generate lexical fallback support answer: %v; cause: %v", err, fwusecase.LogErrorOf(err))
			}
			if !strings.Contains(response.Message, tc.wantAnswer) || len(response.Citations) == 0 {
				t.Fatalf("expected Chinese entity answer containing %q with citations, got %#v", tc.wantAnswer, response)
			}
			if response.Citations[0].SourceName != "桃李未来" {
				t.Fatalf("expected citation source name to come from title match, got %#v", response.Citations)
			}

			messages, err := models.ListSupportConversationMessages(t.Context(), conversation.ConversationID)
			if err != nil {
				t.Fatalf("list support conversation messages: %v", err)
			}
			foundRetrieved := false
			for _, message := range messages {
				if message.Role == models.SupportMessageRoleAssistant && message.RetrievalStatus == "retrieved" {
					foundRetrieved = true
				}
			}
			if !foundRetrieved {
				t.Fatalf("expected assistant message retrieval_status=retrieved, got %#v", messages)
			}
		})
	}
}

func TestSearchKnowledgeChunksByTextMatchesTitleContainedInQuestion(t *testing.T) {
	manager := setupUsecaseOrderTxDB(t)
	appDB, err := manager.GetDB("app")
	if err != nil {
		t.Fatalf("get app db: %v", err)
	}

	source, err := models.CreateKnowledgeSource(t.Context(), models.SaveKnowledgeSourceCmd{
		ID:         "kb-chat-title-contained-source",
		Title:      "桃李未来",
		SourceType: models.KBSourceTypeManual,
		Enabled:    true,
		Content:    "桃李未来是深圳知名的教育集团，成立于2018年，拥有8条业务线、22家校区。",
	})
	if err != nil {
		t.Fatalf("create Chinese entity knowledge source: %v", err)
	}
	if _, err := appDB.Exec(`
		INSERT INTO kb_chunks (
		  id, source_id, document_id, chunk_index, content, content_hash, token_count, char_count,
		  embedding_model_code, embedding_provider_model_id, embedding_dimensions, embedding_status,
		  enabled, created_at, updated_at
		) VALUES (
		  'kb-chat-title-contained-chunk', ?, ?, 0,
		  '桃李未来是深圳知名的教育集团，成立于2018年，拥有8条业务线、22家校区。',
		  'chunk-hash', 10, 40, 'test-model', 'test-provider-model', 64, 'embedded',
		  1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
		)`, source.ID, source.DocumentID); err != nil {
		t.Fatalf("insert embedded chunk: %v", err)
	}

	cases := []struct {
		query       string
		wantContent string
	}{
		{query: "桃李未来成立于哪一年", wantContent: "2018年"},
		{query: "桃李未来有多少个校区", wantContent: "22家校区"},
	}
	for _, tc := range cases {
		results, err := models.SearchKnowledgeChunksByText(t.Context(), []string{tc.query}, 5)
		if err != nil {
			t.Fatalf("search knowledge chunks by text: %v", err)
		}
		if len(results) == 0 {
			t.Fatalf("expected title-contained question %q to match source title", tc.query)
		}
		if results[0].Title != "桃李未来" || !strings.Contains(results[0].Content, tc.wantContent) {
			t.Fatalf("unexpected lexical search result for %q: %#v", tc.query, results[0])
		}
	}
}

func TestIndexDocumentMissingEmbeddingConfigMentionsParameterMenu(t *testing.T) {
	manager := setupUsecaseOrderTxDB(t)
	appDB, err := manager.GetDB("app")
	if err != nil {
		t.Fatalf("get app db: %v", err)
	}
	for _, query := range []string{
		`DELETE FROM integration_operation_configs WHERE scenario = 'embedding'`,
		`DELETE FROM integration_model_options WHERE scenario = 'embedding'`,
		`DELETE FROM integration_channels WHERE scenario = 'embedding'`,
		`DELETE FROM integration_credentials WHERE credential_type = 'none'`,
	} {
		if _, err := appDB.Exec(query); err != nil {
			t.Fatalf("remove default embedding config: %v", err)
		}
	}

	source, err := models.CreateKnowledgeSource(t.Context(), models.SaveKnowledgeSourceCmd{
		ID:         "kb-missing-embedding-source",
		Title:      "Missing Embedding Source",
		SourceType: models.KBSourceTypeManual,
		Enabled:    true,
		Content:    "This document should fail because no embedding channel exists.",
	})
	if err != nil {
		t.Fatalf("create knowledge source: %v", err)
	}

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	if err := usecase.IndexDocument(ctx, usecase.IndexDocumentCmd{DocumentID: source.DocumentID}); err == nil {
		t.Fatalf("expected missing embedding config error")
	}

	doc, err := models.GetKBDocumentByID(t.Context(), source.DocumentID)
	if err != nil {
		t.Fatalf("load failed document: %v", err)
	}
	if doc.IndexStatus != models.KBIndexStatusFailed {
		t.Fatalf("expected failed index status, got %#v", doc)
	}
	if !strings.Contains(doc.LastIndexError, "Parameter > Embedding") {
		t.Fatalf("expected Parameter > Embedding guidance, got %q", doc.LastIndexError)
	}
	if strings.Contains(doc.LastIndexError, "Settings > Integrations") {
		t.Fatalf("did not expect stale settings guidance, got %q", doc.LastIndexError)
	}
}

func TestIndexDocumentWithoutContentReturnsValidation(t *testing.T) {
	setupUsecaseOrderTxDB(t)
	source, err := models.CreateKnowledgeSource(t.Context(), models.SaveKnowledgeSourceCmd{
		ID:         "kb-empty-content-source",
		Title:      "Empty Content Source",
		SourceType: models.KBSourceTypeManual,
		Enabled:    true,
	})
	if err != nil {
		t.Fatalf("create knowledge source: %v", err)
	}

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	err = usecase.IndexDocument(ctx, usecase.IndexDocumentCmd{DocumentID: source.DocumentID})
	if err == nil {
		t.Fatalf("expected empty content validation error")
	}
	if fwusecase.CodeOf(err) != fwusecase.CodeValidation {
		t.Fatalf("expected validation code, got %q: %v", fwusecase.CodeOf(err), err)
	}
	if !strings.Contains(err.Error(), "document content is required before indexing") {
		t.Fatalf("unexpected safe error message: %v", err)
	}

	doc, err := models.GetKBDocumentByID(t.Context(), source.DocumentID)
	if err != nil {
		t.Fatalf("load failed document: %v", err)
	}
	if doc.IndexStatus != models.KBIndexStatusFailed || doc.LastIndexError != "document has no content" {
		t.Fatalf("expected failed status with content error, got %#v", doc)
	}

	sources, err := usecase.ListKBSources(ctx)
	if err != nil {
		t.Fatalf("list sources: %v", err)
	}
	if len(sources) != 1 || sources[0].IndexStatus != models.KBIndexStatusFailed || sources[0].LastIndexError != "document has no content" {
		t.Fatalf("expected source status to mirror document failure, got %#v", sources)
	}
}

func TestIndexDocumentWithoutContentDoesNotDeleteExistingChunks(t *testing.T) {
	manager := setupUsecaseOrderTxDB(t)
	appDB, err := manager.GetDB("app")
	if err != nil {
		t.Fatalf("get app db: %v", err)
	}
	source, err := models.CreateKnowledgeSource(t.Context(), models.SaveKnowledgeSourceCmd{
		ID:          "kb-empty-content-keeps-chunks-source",
		Title:       "Empty Content Keeps Chunks Source",
		SourceType:  models.KBSourceTypeManual,
		Enabled:     true,
		ContentHash: "old-hash",
	})
	if err != nil {
		t.Fatalf("create knowledge source: %v", err)
	}
	if _, err := appDB.Exec(`
		INSERT INTO kb_chunks (
		  id, source_id, document_id, chunk_index, content, content_hash, token_count, char_count,
		  embedding_model_code, embedding_provider_model_id, embedding_dimensions, embedding_status,
		  enabled, created_at, updated_at
		) VALUES (
		  'old-chunk', ?, ?, 0, 'old indexed content', 'old-chunk-hash', 3, 19,
		  'old-model', 'old-provider-model', 64, 'embedded', 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
		)`, source.ID, source.DocumentID); err != nil {
		t.Fatalf("insert old chunk: %v", err)
	}

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	err = usecase.IndexDocument(ctx, usecase.IndexDocumentCmd{DocumentID: source.DocumentID})
	if fwusecase.CodeOf(err) != fwusecase.CodeValidation {
		t.Fatalf("expected validation code, got %q: %v", fwusecase.CodeOf(err), err)
	}

	var chunkCount int
	if err := appDB.Get(&chunkCount, `SELECT COUNT(1) FROM kb_chunks WHERE document_id = ?`, source.DocumentID); err != nil {
		t.Fatalf("count chunks: %v", err)
	}
	if chunkCount != 1 {
		t.Fatalf("expected empty-content reindex to preserve existing chunks, got %d", chunkCount)
	}
}

func TestCreateKBDocumentUsesRequestedSource(t *testing.T) {
	setupUsecaseOrderTxDB(t)
	source, err := models.CreateKnowledgeSource(t.Context(), models.SaveKnowledgeSourceCmd{
		ID:         "kb-document-parent",
		Title:      "Parent Source",
		SourceType: models.KBSourceTypeManual,
		Enabled:    true,
		Content:    "Parent source description",
	})
	if err != nil {
		t.Fatalf("create knowledge source: %v", err)
	}

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	doc, err := usecase.CreateKBDocument(ctx, usecase.CreateKBDocumentCmd{
		SourceID: source.ID,
		Title:    "Child Document",
		Content:  "Child document content",
	})
	if err != nil {
		t.Fatalf("create document: %v", err)
	}
	if doc.SourceID != source.ID {
		t.Fatalf("expected document source %q, got %#v", source.ID, doc)
	}

	sources, err := usecase.ListKBSources(ctx)
	if err != nil {
		t.Fatalf("list sources: %v", err)
	}
	if len(sources) != 1 {
		t.Fatalf("expected one source after creating child document, got %d: %#v", len(sources), sources)
	}
	if sources[0].Description != "Parent source description" {
		t.Fatalf("expected source description preserved, got %#v", sources[0])
	}

	docs, err := usecase.ListKBDocuments(ctx, source.ID)
	if err != nil {
		t.Fatalf("list documents: %v", err)
	}
	if len(docs) != 2 {
		t.Fatalf("expected parent document plus child document, got %d: %#v", len(docs), docs)
	}
	foundChild := false
	for _, candidate := range docs {
		if candidate.ID == doc.ID && candidate.SourceID == source.ID && candidate.Content == "Child document content" {
			foundChild = true
		}
	}
	if !foundChild {
		t.Fatalf("created document not found under requested source: %#v", docs)
	}
}

func TestDeleteKBDocumentRemovesDocumentChunksAndVectors(t *testing.T) {
	manager := setupUsecaseOrderTxDB(t)
	appDB, err := manager.GetDB("app")
	if err != nil {
		t.Fatalf("get app db: %v", err)
	}
	adapterKey := "embedding.local_hash_64.test.delete-document"
	if err := usecase.RegisterEmbeddingAdapter(adapterKey, localhash.NewAdapter(64)); err != nil {
		t.Fatalf("register local embedding adapter: %v", err)
	}
	if _, err := appDB.Exec(`
		UPDATE integration_channels
		SET adapter_key = ?
		WHERE scenario = 'embedding' AND channel_code = 'local-hash-64'
	`, adapterKey); err != nil {
		t.Fatalf("isolate local embedding adapter key: %v", err)
	}
	if _, err := appDB.Exec(`
		UPDATE integration_operation_configs
		SET channel_code = 'local-hash-64', model_code = 'local-hash-64'
		WHERE scenario = 'embedding' AND operation = 'embedding_create'
	`); err != nil {
		t.Fatalf("select local embedding operation config: %v", err)
	}

	source, err := models.CreateKnowledgeSource(t.Context(), models.SaveKnowledgeSourceCmd{
		ID:         "kb-delete-document-source",
		Title:      "Delete Document Source",
		SourceType: models.KBSourceTypeManual,
		Enabled:    true,
		Content:    "The only document may be deleted without deleting the source.",
	})
	if err != nil {
		t.Fatalf("create knowledge source: %v", err)
	}
	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	if err := usecase.IndexDocument(ctx, usecase.IndexDocumentCmd{DocumentID: source.DocumentID}); err != nil {
		t.Fatalf("index document before delete: %v", err)
	}

	var vectorCount int
	if err := appDB.Get(&vectorCount, `SELECT COUNT(1) FROM kb_chunk_embedding_vec`); err != nil {
		t.Fatalf("count vectors before delete: %v", err)
	}
	if vectorCount == 0 {
		t.Fatalf("expected vectors before delete")
	}

	if err := usecase.DeleteKBDocument(ctx, usecase.DeleteKBDocumentCmd{
		SourceID:   source.ID,
		DocumentID: source.DocumentID,
	}); err != nil {
		t.Fatalf("delete document: %v", err)
	}

	counts := map[string]int{}
	for name, query := range map[string]string{
		"documents":         `SELECT COUNT(1) FROM kb_documents WHERE id = ?`,
		"chunks":            `SELECT COUNT(1) FROM kb_chunks WHERE document_id = ?`,
		"chunk_embeddings":  `SELECT COUNT(1) FROM kb_chunk_embeddings`,
		"embedding_rows":    `SELECT COUNT(1) FROM kb_embedding_rows`,
		"embedding_vectors": `SELECT COUNT(1) FROM kb_chunk_embedding_vec`,
	} {
		var count int
		if name == "documents" || name == "chunks" {
			err = appDB.Get(&count, query, source.DocumentID)
		} else {
			err = appDB.Get(&count, query)
		}
		if err != nil {
			t.Fatalf("count %s after delete: %v", name, err)
		}
		counts[name] = count
	}
	for name, count := range counts {
		if count != 0 {
			t.Fatalf("expected %s to be deleted, got %d (all counts %#v)", name, count, counts)
		}
	}

	docs, err := usecase.ListKBDocuments(ctx, source.ID)
	if err != nil {
		t.Fatalf("list documents after delete: %v", err)
	}
	if len(docs) != 0 {
		t.Fatalf("expected source to remain with no documents, got %#v", docs)
	}
	sources, err := usecase.ListKBSources(ctx)
	if err != nil {
		t.Fatalf("list sources after delete: %v", err)
	}
	if len(sources) != 1 || sources[0].ID != source.ID {
		t.Fatalf("expected source to remain after deleting last document, got %#v", sources)
	}
	if sources[0].IndexStatus != models.KBIndexStatusPending || sources[0].LastIndexError != "" {
		t.Fatalf("expected empty source status to reset to pending, got %#v", sources[0])
	}
}

func TestDeleteKBDocumentRequiresMatchingSource(t *testing.T) {
	setupUsecaseOrderTxDB(t)
	source, err := models.CreateKnowledgeSource(t.Context(), models.SaveKnowledgeSourceCmd{
		ID:         "kb-delete-document-owner",
		Title:      "Delete Document Owner",
		SourceType: models.KBSourceTypeManual,
		Enabled:    true,
		Content:    "Owned document content.",
	})
	if err != nil {
		t.Fatalf("create knowledge source: %v", err)
	}

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	err = usecase.DeleteKBDocument(ctx, usecase.DeleteKBDocumentCmd{
		SourceID:   "other-source",
		DocumentID: source.DocumentID,
	})
	if fwusecase.CodeOf(err) != fwusecase.CodeNotFound {
		t.Fatalf("expected not found for source mismatch, got code=%q err=%v", fwusecase.CodeOf(err), err)
	}
	if _, err := models.GetKBDocumentByID(t.Context(), source.DocumentID); err != nil {
		t.Fatalf("document should remain after source mismatch delete: %v", err)
	}
}

func TestUpdateKBSourcePreservesDescriptionForFrontendRoundTrip(t *testing.T) {
	setupUsecaseOrderTxDB(t)
	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)

	source, err := usecase.CreateKBSource(ctx, usecase.CreateKBSourceCmd{
		Title:       "Original Source",
		Description: "Original description",
		SourceType:  models.KBSourceTypeManual,
	})
	if err != nil {
		t.Fatalf("create source: %v", err)
	}
	if source.Description != "Original description" {
		t.Fatalf("expected created description, got %#v", source)
	}

	updated, err := usecase.UpdateKBSource(ctx, usecase.UpdateKBSourceCmd{
		ID:          source.ID,
		Title:       "Updated Source",
		Description: source.Description,
	})
	if err != nil {
		t.Fatalf("update source: %v", err)
	}
	if updated.Description != "Original description" {
		t.Fatalf("expected description to round-trip, got %#v", updated)
	}

	docs, err := usecase.ListKBDocuments(ctx, source.ID)
	if err != nil {
		t.Fatalf("list documents: %v", err)
	}
	if len(docs) != 1 || docs[0].Content != "Original description" {
		t.Fatalf("source update should preserve primary document content, got %#v", docs)
	}
}

func seedEmbeddingChannelOnlyConfig(t *testing.T, appDB *sqlx.DB, channelCode string, apiKey string) {
	t.Helper()

	credentialValue, err := credentialsForTest(apiKey)
	if err != nil {
		t.Fatalf("prepare credential: %v", err)
	}
	credentialID := channelCode + "-credential"
	channelID := channelCode + "-channel"

	if _, err := appDB.Exec(`
		INSERT INTO integration_credentials (id, credential_type, ciphertext, key_version, masked_value, value_text, enabled)
		VALUES (?, 'api_key', ?, '', '', ?, 1)
	`, credentialID, credentialValue, credentialValue); err != nil {
		t.Fatalf("insert embedding credential: %v", err)
	}
	if _, err := appDB.Exec(`
		INSERT INTO integration_channels (
			id, scenario, channel_code, provider_code, adapter_key, environment, enabled, priority, credential_id, config_json
		) VALUES (?, 'embedding', ?, 'siliconflow', 'embedding.siliconflow.openai_compatible', 'test', 1, 1, ?, '{"base_url":"https://api.siliconflow.cn","model":"Qwen/Qwen3-Embedding-4B","dimensions":64,"encoding_format":"float","endpoint_path":"/v1/embeddings"}')
	`, channelID, channelCode, credentialID); err != nil {
		t.Fatalf("insert embedding channel: %v", err)
	}
	if _, err := appDB.Exec(`
		INSERT INTO integration_operation_configs (
			id, scenario, operation, channel_code, model_code, enabled, config_json
		) VALUES (?, 'embedding', 'embedding_create', ?, '', 1, '{}')
		ON CONFLICT(scenario, operation) DO UPDATE SET
			channel_code = excluded.channel_code,
			model_code = excluded.model_code,
			enabled = excluded.enabled,
			config_json = excluded.config_json
	`, channelCode+"-operation", channelCode); err != nil {
		t.Fatalf("insert embedding operation config: %v", err)
	}
}

func seedEmbeddingModelOption(t *testing.T, appDB *sqlx.DB, channelCode, modelCode, providerModelID string) {
	t.Helper()

	if _, err := appDB.Exec(`
		INSERT INTO integration_model_options (
			id, scenario, channel_id, model_code, provider_model_id, default_params_json, enabled
		) VALUES (?, 'embedding', ?, ?, ?, '{"dimensions":64,"encoding_format":"float"}', 1)
	`, channelCode+"-"+modelCode+"-model", channelCode+"-channel", modelCode, providerModelID); err != nil {
		t.Fatalf("insert embedding model option: %v", err)
	}
}

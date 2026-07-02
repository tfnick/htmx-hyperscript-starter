package models

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/tfnick/go-svelte-starter/api/db"
	"github.com/tfnick/go-svelte-starter/api/framework/data/modelerror"
	"github.com/tfnick/go-svelte-starter/api/framework/timefmt"
)

const (
	KBSourceTypeManual   = "manual"
	KBSourceTypeMarkdown = "markdown"
	KBSourceTypeURL      = "url"

	KBIndexStatusPending    = "pending"
	KBIndexStatusProcessing = "processing"
	KBIndexStatusIndexed    = "indexed"
	KBIndexStatusFailed     = "failed"

	KBEmbeddingStatusEmbedded = "embedded"
	KBEmbeddingStatusFailed   = "failed"

	SupportConversationStatusOpen         = "open"
	SupportConversationStatusLeadCaptured = "lead_captured"
	SupportConversationStatusClosed       = "closed"

	SupportLeadCaptureIdle      = "idle"
	SupportLeadCaptureRequested = "requested"
	SupportLeadCaptureCaptured  = "captured"

	SupportMessageRoleVisitor   = "visitor"
	SupportMessageRoleAssistant = "assistant"
	SupportMessageRoleSystem    = "system"
)

type KnowledgeSourceRecord struct {
	ID                  string `db:"id"`
	Title               string `db:"title"`
	SourceType          string `db:"source_type"`
	Category            string `db:"category"`
	Tags                string `db:"tags"`
	SourceURL           string `db:"source_url"`
	Enabled             bool   `db:"enabled"`
	IndexStatus         string `db:"index_status"`
	Version             int    `db:"version"`
	ContentHash         string `db:"content_hash"`
	LastIndexedAt       string `db:"last_indexed_at"`
	LastIndexError      string `db:"last_index_error"`
	CreatedAt           string `db:"created_at"`
	UpdatedAt           string `db:"updated_at"`
	DocumentID          string `db:"document_id"`
	DocumentTitle       string `db:"document_title"`
	Content             string `db:"content"`
	ExtractedText       string `db:"extracted_text"`
	DocumentContentHash string `db:"document_content_hash"`
	DocumentVersion     int    `db:"document_version"`
	FileName            string `db:"file_name"`
	FileMimeType        string `db:"file_mime_type"`
	FileSize            int64  `db:"file_size"`
	ChunkCount          int    `db:"chunk_count"`
}

type SaveKnowledgeSourceCmd struct {
	ID            string
	Title         string
	SourceType    string
	Category      string
	Tags          string
	SourceURL     string
	Enabled       bool
	Content       string
	ExtractedText string
	ContentHash   string
	FileName      string
	FileMimeType  string
	FileSize      int64
	FileText      string
}

type SaveKBDocumentCmd struct {
	SourceID      string
	Title         string
	Content       string
	ExtractedText string
	ContentHash   string
	Enabled       bool
}

type KnowledgeChunkInsert struct {
	ID                       string
	SourceID                 string
	DocumentID               string
	ChunkIndex               int
	Content                  string
	ContentHash              string
	TokenCount               int
	CharCount                int
	EmbeddingModelCode       string
	EmbeddingProviderModelID string
	EmbeddingDimensions      int
	EmbeddingJSON            string
}

type KnowledgeSearchResult struct {
	ChunkID                  string  `db:"chunk_id"`
	SourceID                 string  `db:"source_id"`
	DocumentID               string  `db:"document_id"`
	Title                    string  `db:"title"`
	SourceType               string  `db:"source_type"`
	SourceURL                string  `db:"source_url"`
	Content                  string  `db:"content"`
	Distance                 float64 `db:"distance"`
	EmbeddingModelCode       string  `db:"embedding_model_code"`
	EmbeddingProviderModelID string  `db:"embedding_provider_model_id"`
}

type SupportConversation struct {
	ID               string `db:"id"`
	VisitorTokenHash string `db:"visitor_token_hash"`
	VisitorIPHash    string `db:"visitor_ip_hash"`
	SourcePage       string `db:"source_page"`
	Referrer         string `db:"referrer"`
	Status           string `db:"status"`
	LeadCaptureState string `db:"lead_capture_state"`
	DetectedIntent   string `db:"detected_intent"`
	Summary          string `db:"summary"`
	MessageCount     int    `db:"message_count"`
	LastMessageAt    string `db:"last_message_at"`
	CreatedAt        string `db:"created_at"`
	UpdatedAt        string `db:"updated_at"`
}

type SupportMessage struct {
	ID              string `db:"id"`
	ConversationID  string `db:"conversation_id"`
	Role            string `db:"role"`
	Content         string `db:"content"`
	RetrievalStatus string `db:"retrieval_status"`
	CreatedAt       string `db:"created_at"`
}

type SupportCitation struct {
	ID             string  `db:"id"`
	MessageID      string  `db:"message_id"`
	ConversationID string  `db:"conversation_id"`
	ChunkID        string  `db:"chunk_id"`
	SourceID       string  `db:"source_id"`
	DocumentID     string  `db:"document_id"`
	Snippet        string  `db:"snippet"`
	Distance       float64 `db:"distance"`
	CreatedAt      string  `db:"created_at"`
	Title          string  `db:"title"`
	SourceType     string  `db:"source_type"`
	SourceURL      string  `db:"source_url"`
}

type SupportLead struct {
	ID                  string `db:"id"`
	ConversationID      string `db:"conversation_id"`
	ContactEmail        string `db:"contact_email"`
	ContactPhone        string `db:"contact_phone"`
	Name                string `db:"name"`
	Company             string `db:"company"`
	NeedDescription     string `db:"need_description"`
	SourcePage          string `db:"source_page"`
	DetectedIntent      string `db:"detected_intent"`
	ConversationSummary string `db:"conversation_summary"`
	CreatedAt           string `db:"created_at"`
}

func ListKnowledgeSources(ctx context.Context) ([]KnowledgeSourceRecord, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return nil, fmt.Errorf("database unavailable: %w", err)
	}
	var records []KnowledgeSourceRecord
	err = d.SelectP(&records, `
	SELECT
	  s.id, s.title, s.source_type, s.category, s.tags, s.source_url, s.enabled, s.index_status,
	  s.version, s.content_hash, COALESCE(s.last_indexed_at, '') AS last_indexed_at,
	  s.last_index_error, s.created_at, s.updated_at,
	  COALESCE(d.id, '') AS document_id,
	  COALESCE(d.title, '') AS document_title,
	  COALESCE(d.content, '') AS content,
	  COALESCE(d.extracted_text, '') AS extracted_text,
	  COALESCE(d.content_hash, '') AS document_content_hash,
	  COALESCE(d.version, 0) AS document_version,
	  COALESCE(f.file_name, '') AS file_name,
	  COALESCE(f.file_mime_type, '') AS file_mime_type,
	  COALESCE(f.file_size, 0) AS file_size,
	  COALESCE((SELECT COUNT(1) FROM kb_chunks c WHERE c.source_id = s.id), 0) AS chunk_count
	FROM kb_sources s
	LEFT JOIN kb_documents d ON d.id = (
	  SELECT kd.id
	  FROM kb_documents kd
	  WHERE kd.source_id = s.id
	  ORDER BY kd.created_at ASC, kd.id ASC
	  LIMIT 1
	)
	LEFT JOIN kb_source_files f ON f.document_id = d.id
	ORDER BY s.updated_at DESC, s.created_at DESC`)
	return records, err
}

func GetKnowledgeSourceByID(ctx context.Context, id string) (KnowledgeSourceRecord, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return KnowledgeSourceRecord{}, fmt.Errorf("database unavailable: %w", err)
	}
	var record KnowledgeSourceRecord
	err = d.GetP(&record, `
	SELECT
	  s.id, s.title, s.source_type, s.category, s.tags, s.source_url, s.enabled, s.index_status,
	  s.version, s.content_hash, COALESCE(s.last_indexed_at, '') AS last_indexed_at,
	  s.last_index_error, s.created_at, s.updated_at,
	  COALESCE(d.id, '') AS document_id,
	  COALESCE(d.title, '') AS document_title,
	  COALESCE(d.content, '') AS content,
	  COALESCE(d.extracted_text, '') AS extracted_text,
	  COALESCE(d.content_hash, '') AS document_content_hash,
	  COALESCE(d.version, 0) AS document_version,
	  COALESCE(f.file_name, '') AS file_name,
	  COALESCE(f.file_mime_type, '') AS file_mime_type,
	  COALESCE(f.file_size, 0) AS file_size,
	  COALESCE((SELECT COUNT(1) FROM kb_chunks c WHERE c.source_id = s.id), 0) AS chunk_count
	FROM kb_sources s
	LEFT JOIN kb_documents d ON d.id = (
	  SELECT kd.id
	  FROM kb_documents kd
	  WHERE kd.source_id = s.id
	  ORDER BY kd.created_at ASC, kd.id ASC
	  LIMIT 1
	)
	LEFT JOIN kb_source_files f ON f.document_id = d.id
	WHERE s.id = ?`, strings.TrimSpace(id))
	if errors.Is(err, sql.ErrNoRows) {
		return KnowledgeSourceRecord{}, modelerror.ErrNotFound
	}
	return record, err
}

func CreateKnowledgeSource(ctx context.Context, cmd SaveKnowledgeSourceCmd) (KnowledgeSourceRecord, error) {
	now := timefmt.NowSQLiteDateTime()
	sourceID := strings.TrimSpace(cmd.ID)
	if sourceID == "" {
		sourceID = uuid.Must(uuid.NewV7()).String()
	}
	documentID := uuid.Must(uuid.NewV7()).String()
	contentHash := strings.TrimSpace(cmd.ContentHash)

	exec, err := db.EngineFor(ctx, "app")
	if err != nil {
		return KnowledgeSourceRecord{}, fmt.Errorf("database unavailable: %w", err)
	}
	if _, err := exec.ExecP(`
	INSERT INTO kb_sources (
	  id, title, source_type, category, tags, source_url, enabled, index_status, version,
	  content_hash, last_index_error, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, 1, ?, '', ?, ?)`,
		sourceID, cmd.Title, cmd.SourceType, cmd.Category, cmd.Tags, cmd.SourceURL, boolToInt(cmd.Enabled),
		KBIndexStatusPending, contentHash, now, now); err != nil {
		return KnowledgeSourceRecord{}, fmt.Errorf("insert kb source: %w", err)
	}

	if _, err := exec.ExecP(`
	INSERT INTO kb_documents (
	  id, source_id, title, content, extracted_text, content_hash, version, enabled, index_status,
	  last_index_error, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, 1, ?, ?, '', ?, ?)`,
		documentID, sourceID, cmd.Title, cmd.Content, cmd.ExtractedText, contentHash, boolToInt(cmd.Enabled),
		KBIndexStatusPending, now, now); err != nil {
		return KnowledgeSourceRecord{}, fmt.Errorf("insert kb document: %w", err)
	}

	if strings.TrimSpace(cmd.FileName) != "" {
		if _, err := exec.ExecP(`
	INSERT INTO kb_source_files (
	  id, source_id, document_id, file_name, file_mime_type, file_size, file_text, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			uuid.Must(uuid.NewV7()).String(), sourceID, documentID, cmd.FileName, cmd.FileMimeType,
			cmd.FileSize, cmd.FileText, now); err != nil {
			return KnowledgeSourceRecord{}, fmt.Errorf("insert kb source file: %w", err)
		}
	}

	return GetKnowledgeSourceByID(ctx, sourceID)
}

func UpdateKnowledgeSource(ctx context.Context, cmd SaveKnowledgeSourceCmd) (KnowledgeSourceRecord, error) {
	now := timefmt.NowSQLiteDateTime()
	sourceID := strings.TrimSpace(cmd.ID)
	if sourceID == "" {
		return KnowledgeSourceRecord{}, modelerror.ErrNotFound
	}

	record, err := GetKnowledgeSourceByID(ctx, sourceID)
	if err != nil {
		return KnowledgeSourceRecord{}, err
	}
	contentHash := strings.TrimSpace(cmd.ContentHash)
	exec, err := db.EngineFor(ctx, "app")
	if err != nil {
		return KnowledgeSourceRecord{}, fmt.Errorf("database unavailable: %w", err)
	}
	res, err := exec.ExecP(`
	UPDATE kb_sources
	SET title = ?, category = ?, tags = ?, source_url = ?, enabled = ?, index_status = ?,
	    version = version + 1, content_hash = ?, last_index_error = '', updated_at = ?
	WHERE id = ?`,
		cmd.Title, cmd.Category, cmd.Tags, cmd.SourceURL, boolToInt(cmd.Enabled),
		KBIndexStatusPending, contentHash, now, sourceID)
	if err != nil {
		return KnowledgeSourceRecord{}, fmt.Errorf("update kb source: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return KnowledgeSourceRecord{}, fmt.Errorf("read affected rows: %w", err)
	}
	if affected == 0 {
		return KnowledgeSourceRecord{}, modelerror.ErrNotFound
	}

	if _, err := exec.ExecP(`
	UPDATE kb_documents
	SET title = ?, content = ?, extracted_text = ?, content_hash = ?, version = version + 1,
	    enabled = ?, index_status = ?, last_index_error = '', updated_at = ?
	WHERE id = ?`,
		cmd.Title, cmd.Content, cmd.ExtractedText, contentHash, boolToInt(cmd.Enabled),
		KBIndexStatusPending, now, record.DocumentID); err != nil {
		return KnowledgeSourceRecord{}, fmt.Errorf("update kb document: %w", err)
	}

	if strings.TrimSpace(cmd.FileName) != "" {
		if _, err := exec.ExecP(`DELETE FROM kb_source_files WHERE document_id = ?`, record.DocumentID); err != nil {
			return KnowledgeSourceRecord{}, fmt.Errorf("delete kb source files: %w", err)
		}
		if _, err := exec.ExecP(`
	INSERT INTO kb_source_files (
	  id, source_id, document_id, file_name, file_mime_type, file_size, file_text, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			uuid.Must(uuid.NewV7()).String(), sourceID, record.DocumentID, cmd.FileName, cmd.FileMimeType,
			cmd.FileSize, cmd.FileText, now); err != nil {
			return KnowledgeSourceRecord{}, fmt.Errorf("insert kb source file: %w", err)
		}
	}

	return GetKnowledgeSourceByID(ctx, sourceID)
}

func SetKnowledgeSourceEnabled(ctx context.Context, id string, enabled bool) (KnowledgeSourceRecord, error) {
	now := timefmt.NowSQLiteDateTime()
	exec, err := db.EngineFor(ctx, "app")
	if err != nil {
		return KnowledgeSourceRecord{}, fmt.Errorf("database unavailable: %w", err)
	}
	res, err := exec.ExecP(`
	UPDATE kb_sources SET enabled = ?, updated_at = ? WHERE id = ?`, boolToInt(enabled), now, strings.TrimSpace(id))
	if err != nil {
		return KnowledgeSourceRecord{}, fmt.Errorf("update kb source enabled: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return KnowledgeSourceRecord{}, fmt.Errorf("read affected rows: %w", err)
	}
	if affected == 0 {
		return KnowledgeSourceRecord{}, modelerror.ErrNotFound
	}
	if _, err := exec.ExecP(`
	UPDATE kb_documents SET enabled = ?, updated_at = ? WHERE source_id = ?`, boolToInt(enabled), now, strings.TrimSpace(id)); err != nil {
		return KnowledgeSourceRecord{}, fmt.Errorf("update kb documents enabled: %w", err)
	}
	if _, err := exec.ExecP(`
	UPDATE kb_chunks SET enabled = ?, updated_at = ? WHERE source_id = ?`, boolToInt(enabled), now, strings.TrimSpace(id)); err != nil {
		return KnowledgeSourceRecord{}, fmt.Errorf("update kb chunks enabled: %w", err)
	}
	return GetKnowledgeSourceByID(ctx, id)
}

func UpdateKnowledgeIndexStatus(ctx context.Context, sourceID, documentID, status, errMsg string) error {
	now := timefmt.NowSQLiteDateTime()
	lastIndexedAt := ""
	if status == KBIndexStatusIndexed {
		lastIndexedAt = now
	}
	exec, err := db.EngineFor(ctx, "app")
	if err != nil {
		return fmt.Errorf("database unavailable: %w", err)
	}
	_, err = exec.ExecP(`
	UPDATE kb_sources
	SET index_status = ?, last_index_error = ?, last_indexed_at = NULLIF(?, ''), updated_at = ?
	WHERE id = ?`, status, errMsg, lastIndexedAt, now, sourceID)
	if err != nil {
		return fmt.Errorf("update kb source index status: %w", err)
	}
	_, err = exec.ExecP(`
	UPDATE kb_documents
	SET index_status = ?, last_index_error = ?, last_indexed_at = NULLIF(?, ''), updated_at = ?
	WHERE id = ?`, status, errMsg, lastIndexedAt, now, documentID)
	if err != nil {
		return fmt.Errorf("update kb document index status: %w", err)
	}
	return nil
}

func ReplaceKnowledgeDocumentChunks(ctx context.Context, documentID string, chunks []KnowledgeChunkInsert) error {
	exec, err := db.EngineFor(ctx, "app")
	if err != nil {
		return fmt.Errorf("database unavailable: %w", err)
	}
	var rowIDs []int64
	if err := exec.SelectP(&rowIDs, `
	SELECT er.vector_rowid
	FROM kb_embedding_rows er
	JOIN kb_chunks c ON c.id = er.chunk_id
	WHERE c.document_id = ?`, documentID); err != nil {
		return fmt.Errorf("select vector row ids: %w", err)
	}
	for _, rowID := range rowIDs {
		if _, err := exec.ExecP(`DELETE FROM kb_chunk_embedding_vec WHERE rowid = ?`, rowID); err != nil {
			return fmt.Errorf("delete from chunk embedding vec: %w", err)
		}
	}
	if _, err := exec.ExecP(`DELETE FROM kb_chunks WHERE document_id = ?`, documentID); err != nil {
		return fmt.Errorf("delete kb chunks: %w", err)
	}

	now := timefmt.NowSQLiteDateTime()
	for _, chunk := range chunks {
		if _, err := exec.ExecP(`
	INSERT INTO kb_chunks (
	  id, source_id, document_id, chunk_index, content, content_hash, token_count, char_count,
	  embedding_model_code, embedding_provider_model_id, embedding_dimensions, embedding_status,
	  enabled, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, ?, ?)`,
			chunk.ID, chunk.SourceID, chunk.DocumentID, chunk.ChunkIndex, chunk.Content, chunk.ContentHash,
			chunk.TokenCount, chunk.CharCount, chunk.EmbeddingModelCode, chunk.EmbeddingProviderModelID,
			chunk.EmbeddingDimensions, KBEmbeddingStatusEmbedded, now, now); err != nil {
			return fmt.Errorf("insert kb chunk: %w", err)
		}
		result, err := exec.ExecP(`INSERT INTO kb_embedding_rows (chunk_id, created_at) VALUES (?, ?)`, chunk.ID, now)
		if err != nil {
			return fmt.Errorf("insert kb embedding row: %w", err)
		}
		vectorRowID, err := result.LastInsertId()
		if err != nil {
			return fmt.Errorf("read vector row id: %w", err)
		}
		if _, err := exec.ExecP(`
	INSERT INTO kb_chunk_embeddings (
	  id, chunk_id, vector_rowid, embedding_json, dimensions, model_code, provider_model_id, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			uuid.Must(uuid.NewV7()).String(), chunk.ID, vectorRowID, chunk.EmbeddingJSON,
			chunk.EmbeddingDimensions, chunk.EmbeddingModelCode, chunk.EmbeddingProviderModelID, now); err != nil {
			return fmt.Errorf("insert kb chunk embedding: %w", err)
		}
		if _, err := exec.ExecP(`INSERT INTO kb_chunk_embedding_vec(rowid, embedding) VALUES (?, ?)`, vectorRowID, chunk.EmbeddingJSON); err != nil {
			return fmt.Errorf("insert kb chunk embedding vec: %w", err)
		}
	}
	return nil
}

func SearchKnowledgeChunks(ctx context.Context, embeddingJSON string, limit int) ([]KnowledgeSearchResult, error) {
	if limit <= 0 {
		limit = 5
	}
	// sqlite-vec requires the MATCH/k constraints to stay inside the direct vec0
	// scan; wrap it in a subquery before joining and post-filtering business rows.
	candidateLimit := limit
	if candidateLimit < 20 {
		candidateLimit = 20
	}
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return nil, fmt.Errorf("database unavailable: %w", err)
	}
	var rows []KnowledgeSearchResult
	err = d.SelectP(&rows, `
	SELECT
	  c.id AS chunk_id, c.source_id, c.document_id, s.title, s.source_type, s.source_url,
	  c.content, v.distance, c.embedding_model_code, c.embedding_provider_model_id
	FROM (
	  SELECT rowid, distance
	  FROM kb_chunk_embedding_vec
	  WHERE embedding MATCH ?
	    AND k = ?
	) v
	JOIN kb_embedding_rows er ON er.vector_rowid = v.rowid
	JOIN kb_chunks c ON c.id = er.chunk_id
	JOIN kb_sources s ON s.id = c.source_id
	JOIN kb_documents d ON d.id = c.document_id
	WHERE c.enabled = 1
	  AND d.enabled = 1
	  AND s.enabled = 1
	  AND c.embedding_status = ?
	ORDER BY v.distance
	LIMIT ?`, embeddingJSON, candidateLimit, KBEmbeddingStatusEmbedded, limit)
	return rows, err
}

func SearchKnowledgeChunksByText(ctx context.Context, terms []string, limit int) ([]KnowledgeSearchResult, error) {
	if limit <= 0 {
		limit = 5
	}
	searchTerms := uniqueKnowledgeSearchTerms(terms)
	if len(searchTerms) == 0 {
		return []KnowledgeSearchResult{}, nil
	}

	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return nil, fmt.Errorf("database unavailable: %w", err)
	}

	results := make([]KnowledgeSearchResult, 0, limit)
	seen := map[string]bool{}
	for _, term := range searchTerms {
		if len(results) >= limit {
			break
		}
		pattern := "%" + escapeKnowledgeLikePattern(term) + "%"
		var rows []KnowledgeSearchResult
		err = d.SelectP(&rows, `
	SELECT
	  c.id AS chunk_id, c.source_id, c.document_id, s.title, s.source_type, s.source_url,
	  c.content,
	  CASE
	    WHEN s.title LIKE ? ESCAPE '\' THEN 0.0
	    WHEN d.title LIKE ? ESCAPE '\' THEN 0.01
	    WHEN s.title <> '' AND instr(?, s.title) > 0 THEN 0.02
	    WHEN d.title <> '' AND instr(?, d.title) > 0 THEN 0.03
	    ELSE 0.04
	  END AS distance,
	  c.embedding_model_code, c.embedding_provider_model_id
	FROM kb_chunks c
	JOIN kb_sources s ON s.id = c.source_id
	JOIN kb_documents d ON d.id = c.document_id
	WHERE c.enabled = 1
	  AND d.enabled = 1
	  AND s.enabled = 1
	  AND c.embedding_status = ?
	  AND (
	    s.title LIKE ? ESCAPE '\'
	    OR d.title LIKE ? ESCAPE '\'
	    OR c.content LIKE ? ESCAPE '\'
	    OR (s.title <> '' AND instr(?, s.title) > 0)
	    OR (d.title <> '' AND instr(?, d.title) > 0)
	  )
	ORDER BY distance ASC, c.chunk_index ASC, c.created_at ASC
	LIMIT ?`,
			pattern, pattern, term, term, KBEmbeddingStatusEmbedded, pattern, pattern, pattern, term, term, limit)
		if err != nil {
			return nil, fmt.Errorf("search knowledge chunks by text: %w", err)
		}
		for _, row := range rows {
			if seen[row.ChunkID] {
				continue
			}
			seen[row.ChunkID] = true
			results = append(results, row)
			if len(results) >= limit {
				break
			}
		}
	}

	return results, nil
}

func uniqueKnowledgeSearchTerms(terms []string) []string {
	unique := make([]string, 0, len(terms))
	seen := map[string]bool{}
	for _, term := range terms {
		term = strings.TrimSpace(term)
		if term == "" || seen[term] {
			continue
		}
		seen[term] = true
		unique = append(unique, term)
	}
	return unique
}

func escapeKnowledgeLikePattern(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return replacer.Replace(value)
}

func GetOrCreateSupportConversation(ctx context.Context, tokenHash, ipHash, sourcePage, referrer string) (SupportConversation, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return SupportConversation{}, fmt.Errorf("database unavailable: %w", err)
	}
	var conversation SupportConversation
	err = d.GetP(&conversation, `
	SELECT id, visitor_token_hash, visitor_ip_hash, source_page, referrer, status, lead_capture_state,
	       detected_intent, summary, message_count, COALESCE(last_message_at, '') AS last_message_at,
	       created_at, updated_at
	FROM support_conversations
	WHERE visitor_token_hash = ? AND status IN (?, ?)
	ORDER BY updated_at DESC
	LIMIT 1`, tokenHash, SupportConversationStatusOpen, SupportConversationStatusLeadCaptured)
	if err == nil {
		return conversation, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return SupportConversation{}, fmt.Errorf("get support conversation: %w", err)
	}

	now := timefmt.NowSQLiteDateTime()
	id := uuid.Must(uuid.NewV7()).String()
	if _, err := d.ExecP(`
	INSERT INTO support_conversations (
	  id, visitor_token_hash, visitor_ip_hash, source_page, referrer, status, lead_capture_state,
	  detected_intent, summary, message_count, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, '', '', 0, ?, ?)`,
		id, tokenHash, ipHash, sourcePage, referrer, SupportConversationStatusOpen,
		SupportLeadCaptureIdle, now, now); err != nil {
		return SupportConversation{}, fmt.Errorf("insert support conversation: %w", err)
	}
	return GetSupportConversation(ctx, id)
}

func GetSupportConversation(ctx context.Context, id string) (SupportConversation, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return SupportConversation{}, fmt.Errorf("database unavailable: %w", err)
	}
	var conversation SupportConversation
	err = d.GetP(&conversation, `
	SELECT id, visitor_token_hash, visitor_ip_hash, source_page, referrer, status, lead_capture_state,
	       detected_intent, summary, message_count, COALESCE(last_message_at, '') AS last_message_at,
	       created_at, updated_at
	FROM support_conversations
	WHERE id = ?`, strings.TrimSpace(id))
	if errors.Is(err, sql.ErrNoRows) {
		return SupportConversation{}, modelerror.ErrNotFound
	}
	return conversation, err
}

func AddSupportMessage(ctx context.Context, conversationID, role, content, retrievalStatus string) (SupportMessage, error) {
	now := timefmt.NowSQLiteDateTime()
	messageID := uuid.Must(uuid.NewV7()).String()
	exec, err := db.EngineFor(ctx, "app")
	if err != nil {
		return SupportMessage{}, fmt.Errorf("database unavailable: %w", err)
	}
	if _, err := exec.ExecP(`
	INSERT INTO support_messages (id, conversation_id, role, content, retrieval_status, created_at)
	VALUES (?, ?, ?, ?, ?, ?)`, messageID, conversationID, role, content, retrievalStatus, now); err != nil {
		return SupportMessage{}, fmt.Errorf("insert support message: %w", err)
	}
	if _, err := exec.ExecP(`
	UPDATE support_conversations
	SET message_count = message_count + 1, last_message_at = ?, updated_at = ?
	WHERE id = ?`, now, now, conversationID); err != nil {
		return SupportMessage{}, fmt.Errorf("update support conversation message count: %w", err)
	}
	return GetSupportMessage(ctx, messageID)
}

func GetSupportMessage(ctx context.Context, id string) (SupportMessage, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return SupportMessage{}, fmt.Errorf("database unavailable: %w", err)
	}
	var message SupportMessage
	err = d.GetP(&message, `
	SELECT id, conversation_id, role, content, retrieval_status, created_at
	FROM support_messages
	WHERE id = ?`, strings.TrimSpace(id))
	if errors.Is(err, sql.ErrNoRows) {
		return SupportMessage{}, modelerror.ErrNotFound
	}
	return message, err
}

func AddSupportCitations(ctx context.Context, messageID, conversationID string, citations []KnowledgeSearchResult) error {
	exec, err := db.EngineFor(ctx, "app")
	if err != nil {
		return fmt.Errorf("database unavailable: %w", err)
	}
	now := timefmt.NowSQLiteDateTime()
	for _, citation := range citations {
		if _, err := exec.ExecP(`
	INSERT INTO support_answer_citations (
	  id, message_id, conversation_id, chunk_id, source_id, document_id, snippet, distance, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			uuid.Must(uuid.NewV7()).String(), messageID, conversationID, citation.ChunkID, citation.SourceID,
			citation.DocumentID, citation.Content, citation.Distance, now); err != nil {
			return fmt.Errorf("insert support citation: %w", err)
		}
	}
	return nil
}

func ListSupportConversations(ctx context.Context) ([]SupportConversation, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return nil, fmt.Errorf("database unavailable: %w", err)
	}
	var conversations []SupportConversation
	err = d.SelectP(&conversations, `
	SELECT id, visitor_token_hash, visitor_ip_hash, source_page, referrer, status, lead_capture_state,
	       detected_intent, summary, message_count, COALESCE(last_message_at, '') AS last_message_at,
	       created_at, updated_at
	FROM support_conversations
	ORDER BY updated_at DESC
	LIMIT 200`)
	return conversations, err
}

func ListSupportConversationMessages(ctx context.Context, conversationID string) ([]SupportMessage, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return nil, fmt.Errorf("database unavailable: %w", err)
	}
	var messages []SupportMessage
	err = d.SelectP(&messages, `
	SELECT id, conversation_id, role, content, retrieval_status, created_at
	FROM support_messages
	WHERE conversation_id = ?
	ORDER BY created_at ASC`, strings.TrimSpace(conversationID))
	return messages, err
}

func ListSupportMessageCitations(ctx context.Context, conversationID string) ([]SupportCitation, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return nil, fmt.Errorf("database unavailable: %w", err)
	}
	var citations []SupportCitation
	err = d.SelectP(&citations, `
	SELECT ac.id, ac.message_id, ac.conversation_id, ac.chunk_id, ac.source_id, ac.document_id,
	       ac.snippet, ac.distance, ac.created_at, s.title, s.source_type, s.source_url
	FROM support_answer_citations ac
	JOIN kb_sources s ON s.id = ac.source_id
	WHERE ac.conversation_id = ?
	ORDER BY ac.created_at ASC`, strings.TrimSpace(conversationID))
	return citations, err
}

func CountRecentSupportVisitorMessagesByIP(ctx context.Context, ipHash, since string) (int, error) {
	if strings.TrimSpace(ipHash) == "" {
		return 0, nil
	}
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return 0, fmt.Errorf("database unavailable: %w", err)
	}
	var count int
	err = d.GetP(&count, `
	SELECT COUNT(1)
	FROM support_messages m
	JOIN support_conversations c ON c.id = m.conversation_id
	WHERE c.visitor_ip_hash = ? AND m.role = ? AND m.created_at >= ?`,
		ipHash, SupportMessageRoleVisitor, since)
	return count, err
}

func CreateSupportLead(ctx context.Context, lead SupportLead) (SupportLead, error) {
	now := timefmt.NowSQLiteDateTime()
	id := strings.TrimSpace(lead.ID)
	if id == "" {
		id = uuid.Must(uuid.NewV7()).String()
	}
	exec, err := db.EngineFor(ctx, "app")
	if err != nil {
		return SupportLead{}, fmt.Errorf("database unavailable: %w", err)
	}
	if _, err := exec.ExecP(`
	INSERT INTO support_leads (
	  id, conversation_id, contact_email, contact_phone, name, company, need_description,
	  source_page, detected_intent, conversation_summary, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, lead.ConversationID, lead.ContactEmail, lead.ContactPhone, lead.Name, lead.Company,
		lead.NeedDescription, lead.SourcePage, lead.DetectedIntent, lead.ConversationSummary, now); err != nil {
		return SupportLead{}, fmt.Errorf("insert support lead: %w", err)
	}
	if _, err := exec.ExecP(`
	UPDATE support_conversations
	SET status = ?, lead_capture_state = ?, updated_at = ?
	WHERE id = ?`, SupportConversationStatusLeadCaptured, SupportLeadCaptureCaptured, now, lead.ConversationID); err != nil {
		return SupportLead{}, fmt.Errorf("update support conversation lead status: %w", err)
	}
	return GetSupportLead(ctx, id)
}

func GetSupportLead(ctx context.Context, id string) (SupportLead, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return SupportLead{}, fmt.Errorf("database unavailable: %w", err)
	}
	var lead SupportLead
	err = d.GetP(&lead, `
	SELECT id, conversation_id, contact_email, contact_phone, name, company, need_description,
	       source_page, detected_intent, conversation_summary, created_at
	FROM support_leads
	WHERE id = ?`, strings.TrimSpace(id))
	if errors.Is(err, sql.ErrNoRows) {
		return SupportLead{}, modelerror.ErrNotFound
	}
	return lead, err
}

func ListSupportLeads(ctx context.Context) ([]SupportLead, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return nil, fmt.Errorf("database unavailable: %w", err)
	}
	var leads []SupportLead
	err = d.SelectP(&leads, `
	SELECT id, conversation_id, contact_email, contact_phone, name, company, need_description,
	       source_page, detected_intent, conversation_summary, created_at
	FROM support_leads
	ORDER BY created_at DESC
	LIMIT 200`)
	return leads, err
}

func UpdateSupportConversationIntent(ctx context.Context, conversationID, state, intent string) error {
	exec, err := db.EngineFor(ctx, "app")
	if err != nil {
		return fmt.Errorf("database unavailable: %w", err)
	}
	_, err = exec.ExecP(`
	UPDATE support_conversations
	SET lead_capture_state = ?, detected_intent = ?, updated_at = ?
	WHERE id = ?`, state, intent, timefmt.NowSQLiteDateTime(), conversationID)
	if err != nil {
		return fmt.Errorf("update support conversation intent: %w", err)
	}
	return nil
}

// --- KB Document CRUD (standalone, not joined with sources) ---

type KBDocument struct {
	ID             string `db:"id"`
	SourceID       string `db:"source_id"`
	Title          string `db:"title"`
	Content        string `db:"content"`
	ExtractedText  string `db:"extracted_text"`
	ContentHash    string `db:"content_hash"`
	Version        int    `db:"version"`
	Enabled        int    `db:"enabled"`
	IndexStatus    string `db:"index_status"`
	LastIndexedAt  string `db:"last_indexed_at"`
	LastIndexError string `db:"last_index_error"`
	CreatedAt      string `db:"created_at"`
	UpdatedAt      string `db:"updated_at"`
}

type KBChunk struct {
	ID                       string `db:"id"`
	SourceID                 string `db:"source_id"`
	DocumentID               string `db:"document_id"`
	ChunkIndex               int    `db:"chunk_index"`
	Content                  string `db:"content"`
	ContentHash              string `db:"content_hash"`
	TokenCount               int    `db:"token_count"`
	CharCount                int    `db:"char_count"`
	EmbeddingModelCode       string `db:"embedding_model_code"`
	EmbeddingProviderModelID string `db:"embedding_provider_model_id"`
	EmbeddingDimensions      int    `db:"embedding_dimensions"`
	EmbeddingStatus          string `db:"embedding_status"`
	Enabled                  int    `db:"enabled"`
	CreatedAt                string `db:"created_at"`
	UpdatedAt                string `db:"updated_at"`
}

func ListKBDocumentsBySourceID(ctx context.Context, sourceID string) ([]KBDocument, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return nil, fmt.Errorf("database unavailable: %w", err)
	}
	var docs []KBDocument
	err = d.SelectP(&docs, `
		SELECT id, source_id, title, content, extracted_text, content_hash, version,
		       enabled, index_status, COALESCE(last_indexed_at, '') AS last_indexed_at,
		       last_index_error, created_at, updated_at
		FROM kb_documents
		WHERE source_id = ?
		ORDER BY created_at DESC`, strings.TrimSpace(sourceID))
	return docs, err
}

func GetKBDocumentByID(ctx context.Context, id string) (KBDocument, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return KBDocument{}, fmt.Errorf("database unavailable: %w", err)
	}
	var doc KBDocument
	err = d.GetP(&doc, `
		SELECT id, source_id, title, content, extracted_text, content_hash, version,
		       enabled, index_status, COALESCE(last_indexed_at, '') AS last_indexed_at,
		       last_index_error, created_at, updated_at
		FROM kb_documents
		WHERE id = ?`, strings.TrimSpace(id))
	if errors.Is(err, sql.ErrNoRows) {
		return KBDocument{}, modelerror.ErrNotFound
	}
	return doc, err
}

func CreateKBDocument(ctx context.Context, cmd SaveKBDocumentCmd) (KBDocument, error) {
	now := timefmt.NowSQLiteDateTime()
	sourceID := strings.TrimSpace(cmd.SourceID)
	if sourceID == "" {
		return KBDocument{}, modelerror.ErrNotFound
	}

	if _, err := GetKnowledgeSourceByID(ctx, sourceID); err != nil {
		return KBDocument{}, err
	}

	documentID := uuid.Must(uuid.NewV7()).String()
	contentHash := strings.TrimSpace(cmd.ContentHash)
	exec, err := db.EngineFor(ctx, "app")
	if err != nil {
		return KBDocument{}, fmt.Errorf("database unavailable: %w", err)
	}
	if _, err := exec.ExecP(`
		UPDATE kb_sources
		SET index_status = ?, last_index_error = '', updated_at = ?
		WHERE id = ?`,
		KBIndexStatusPending, now, sourceID); err != nil {
		return KBDocument{}, fmt.Errorf("update kb source status: %w", err)
	}
	if _, err := exec.ExecP(`
		INSERT INTO kb_documents (
		  id, source_id, title, content, extracted_text, content_hash, version, enabled, index_status,
		  last_index_error, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, 1, ?, ?, '', ?, ?)`,
		documentID, sourceID, strings.TrimSpace(cmd.Title), cmd.Content, cmd.ExtractedText, contentHash,
		boolToInt(cmd.Enabled), KBIndexStatusPending, now, now); err != nil {
		return KBDocument{}, fmt.Errorf("insert kb document: %w", err)
	}
	return GetKBDocumentByID(ctx, documentID)
}

func SetKBDocumentEnabled(ctx context.Context, id string, enabled bool) (KBDocument, error) {
	now := timefmt.NowSQLiteDateTime()
	exec, err := db.EngineFor(ctx, "app")
	if err != nil {
		return KBDocument{}, fmt.Errorf("database unavailable: %w", err)
	}
	res, err := exec.ExecP(`
		UPDATE kb_documents SET enabled = ?, updated_at = ? WHERE id = ?`,
		boolToInt(enabled), now, strings.TrimSpace(id))
	if err != nil {
		return KBDocument{}, fmt.Errorf("update kb document enabled: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return KBDocument{}, fmt.Errorf("read affected rows: %w", err)
	}
	if affected == 0 {
		return KBDocument{}, modelerror.ErrNotFound
	}
	return GetKBDocumentByID(ctx, id)
}

func SetKBDocumentStatus(ctx context.Context, id string, status, errMsg string) error {
	now := timefmt.NowSQLiteDateTime()
	lastIndexedAt := ""
	if status == KBIndexStatusIndexed {
		lastIndexedAt = now
	}
	exec, err := db.EngineFor(ctx, "app")
	if err != nil {
		return fmt.Errorf("database unavailable: %w", err)
	}
	res, err := exec.ExecP(`
		UPDATE kb_documents
		SET index_status = ?, last_index_error = ?, last_indexed_at = NULLIF(?, ''), updated_at = ?
		WHERE id = ?`, status, errMsg, lastIndexedAt, now, strings.TrimSpace(id))
	if err != nil {
		return fmt.Errorf("update kb document status: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("read affected rows: %w", err)
	}
	if affected == 0 {
		return modelerror.ErrNotFound
	}
	return nil
}

func DeleteKBChunksByDocumentID(ctx context.Context, documentID string) error {
	exec, err := db.EngineFor(ctx, "app")
	if err != nil {
		return fmt.Errorf("database unavailable: %w", err)
	}
	var rowIDs []int64
	if err := exec.SelectP(&rowIDs, `
		SELECT er.vector_rowid
		FROM kb_embedding_rows er
		JOIN kb_chunks c ON c.id = er.chunk_id
		WHERE c.document_id = ?`, documentID); err != nil {
		return fmt.Errorf("select vector row ids: %w", err)
	}
	for _, rowID := range rowIDs {
		if _, err := exec.ExecP(`DELETE FROM kb_chunk_embedding_vec WHERE rowid = ?`, rowID); err != nil {
			return fmt.Errorf("delete from chunk embedding vec: %w", err)
		}
	}
	if _, err := exec.ExecP(`DELETE FROM kb_chunks WHERE document_id = ?`, documentID); err != nil {
		return fmt.Errorf("delete kb chunks: %w", err)
	}
	return nil
}

func DeleteKBDocument(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return modelerror.ErrNotFound
	}
	doc, err := GetKBDocumentByID(ctx, id)
	if err != nil {
		return err
	}
	if err := DeleteKBChunksByDocumentID(ctx, id); err != nil {
		return err
	}
	exec, err := db.EngineFor(ctx, "app")
	if err != nil {
		return fmt.Errorf("database unavailable: %w", err)
	}
	res, err := exec.ExecP(`DELETE FROM kb_documents WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete kb document: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("read affected rows: %w", err)
	}
	if affected == 0 {
		return modelerror.ErrNotFound
	}
	if err := RefreshKnowledgeSourceIndexStatus(ctx, doc.SourceID); err != nil {
		return fmt.Errorf("refresh kb source index status: %w", err)
	}
	return nil
}

func RefreshKnowledgeSourceIndexStatus(ctx context.Context, sourceID string) error {
	sourceID = strings.TrimSpace(sourceID)
	if sourceID == "" {
		return modelerror.ErrNotFound
	}
	exec, err := db.EngineFor(ctx, "app")
	if err != nil {
		return fmt.Errorf("database unavailable: %w", err)
	}
	var remainingCount int
	if err := exec.GetP(&remainingCount, `SELECT COUNT(1) FROM kb_documents WHERE source_id = ?`, sourceID); err != nil {
		return fmt.Errorf("count kb documents: %w", err)
	}
	if remainingCount == 0 {
		res, err := exec.ExecP(`
			UPDATE kb_sources
			SET index_status = ?, last_index_error = '', last_indexed_at = NULL, updated_at = ?
			WHERE id = ?`, KBIndexStatusPending, timefmt.NowSQLiteDateTime(), sourceID)
		if err != nil {
			return fmt.Errorf("update empty kb source status: %w", err)
		}
		affected, err := res.RowsAffected()
		if err != nil {
			return fmt.Errorf("read affected rows: %w", err)
		}
		if affected == 0 {
			return modelerror.ErrNotFound
		}
		return nil
	}

	var status struct {
		IndexStatus    string `db:"index_status"`
		LastIndexedAt  string `db:"last_indexed_at"`
		LastIndexError string `db:"last_index_error"`
	}
	if err := exec.GetP(&status, `
		SELECT index_status, COALESCE(last_indexed_at, '') AS last_indexed_at, last_index_error
		FROM kb_documents
		WHERE source_id = ?
		ORDER BY
			CASE index_status
				WHEN 'failed' THEN 0
				WHEN 'processing' THEN 1
				WHEN 'pending' THEN 2
				WHEN 'indexed' THEN 3
				ELSE 4
			END,
			updated_at DESC,
			id DESC
		LIMIT 1`, sourceID); err != nil {
		return fmt.Errorf("load kb source aggregate status: %w", err)
	}
	res, err := exec.ExecP(`
		UPDATE kb_sources
		SET index_status = ?, last_index_error = ?, last_indexed_at = NULLIF(?, ''), updated_at = ?
		WHERE id = ?`, status.IndexStatus, status.LastIndexError, status.LastIndexedAt, timefmt.NowSQLiteDateTime(), sourceID)
	if err != nil {
		return fmt.Errorf("update kb source aggregate status: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("read affected rows: %w", err)
	}
	if affected == 0 {
		return modelerror.ErrNotFound
	}
	return nil
}

func UpdateKBDocumentContent(ctx context.Context, id, title, content, contentHash string) (KBDocument, error) {
	now := timefmt.NowSQLiteDateTime()
	exec, err := db.EngineFor(ctx, "app")
	if err != nil {
		return KBDocument{}, fmt.Errorf("database unavailable: %w", err)
	}
	res, err := exec.ExecP(`
		UPDATE kb_documents
		SET title = ?, content = ?, content_hash = ?, version = version + 1,
		    index_status = ?, last_index_error = '', updated_at = ?
		WHERE id = ?`,
		strings.TrimSpace(title), content, strings.TrimSpace(contentHash),
		KBIndexStatusPending, now, strings.TrimSpace(id))
	if err != nil {
		return KBDocument{}, fmt.Errorf("update kb document content: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return KBDocument{}, fmt.Errorf("read affected rows: %w", err)
	}
	if affected == 0 {
		return KBDocument{}, modelerror.ErrNotFound
	}
	return GetKBDocumentByID(ctx, id)
}

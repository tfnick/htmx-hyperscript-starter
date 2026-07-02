CREATE TABLE IF NOT EXISTS kb_sources (
  id TEXT PRIMARY KEY,
  title TEXT NOT NULL,
  source_type TEXT NOT NULL CHECK (source_type IN ('manual', 'markdown', 'url')),
  category TEXT NOT NULL DEFAULT '',
  tags TEXT NOT NULL DEFAULT '',
  source_url TEXT NOT NULL DEFAULT '',
  enabled INTEGER NOT NULL DEFAULT 1,
  index_status TEXT NOT NULL DEFAULT 'pending' CHECK (index_status IN ('pending', 'processing', 'indexed', 'failed')),
  version INTEGER NOT NULL DEFAULT 1,
  content_hash TEXT NOT NULL DEFAULT '',
  last_indexed_at DATETIME,
  last_index_error TEXT NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS kb_documents (
  id TEXT PRIMARY KEY,
  source_id TEXT NOT NULL,
  title TEXT NOT NULL,
  content TEXT NOT NULL DEFAULT '',
  extracted_text TEXT NOT NULL DEFAULT '',
  content_hash TEXT NOT NULL DEFAULT '',
  version INTEGER NOT NULL DEFAULT 1,
  enabled INTEGER NOT NULL DEFAULT 1,
  index_status TEXT NOT NULL DEFAULT 'pending' CHECK (index_status IN ('pending', 'processing', 'indexed', 'failed')),
  last_indexed_at DATETIME,
  last_index_error TEXT NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (source_id) REFERENCES kb_sources(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS kb_source_files (
  id TEXT PRIMARY KEY,
  source_id TEXT NOT NULL,
  document_id TEXT NOT NULL,
  file_name TEXT NOT NULL DEFAULT '',
  file_mime_type TEXT NOT NULL DEFAULT '',
  file_size INTEGER NOT NULL DEFAULT 0,
  file_text TEXT NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (source_id) REFERENCES kb_sources(id) ON DELETE CASCADE,
  FOREIGN KEY (document_id) REFERENCES kb_documents(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS kb_chunks (
  id TEXT PRIMARY KEY,
  source_id TEXT NOT NULL,
  document_id TEXT NOT NULL,
  chunk_index INTEGER NOT NULL,
  content TEXT NOT NULL,
  content_hash TEXT NOT NULL DEFAULT '',
  token_count INTEGER NOT NULL DEFAULT 0,
  char_count INTEGER NOT NULL DEFAULT 0,
  embedding_model_code TEXT NOT NULL DEFAULT '',
  embedding_provider_model_id TEXT NOT NULL DEFAULT '',
  embedding_dimensions INTEGER NOT NULL DEFAULT 0,
  embedding_status TEXT NOT NULL DEFAULT 'pending' CHECK (embedding_status IN ('pending', 'embedded', 'failed')),
  enabled INTEGER NOT NULL DEFAULT 1,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (source_id) REFERENCES kb_sources(id) ON DELETE CASCADE,
  FOREIGN KEY (document_id) REFERENCES kb_documents(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS kb_embedding_rows (
  vector_rowid INTEGER PRIMARY KEY AUTOINCREMENT,
  chunk_id TEXT NOT NULL UNIQUE,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (chunk_id) REFERENCES kb_chunks(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS kb_chunk_embeddings (
  id TEXT PRIMARY KEY,
  chunk_id TEXT NOT NULL UNIQUE,
  vector_rowid INTEGER NOT NULL UNIQUE,
  embedding_json TEXT NOT NULL,
  dimensions INTEGER NOT NULL,
  model_code TEXT NOT NULL DEFAULT '',
  provider_model_id TEXT NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (chunk_id) REFERENCES kb_chunks(id) ON DELETE CASCADE,
  FOREIGN KEY (vector_rowid) REFERENCES kb_embedding_rows(vector_rowid) ON DELETE CASCADE
);

CREATE VIRTUAL TABLE IF NOT EXISTS kb_chunk_embedding_vec USING vec0(
  embedding float[64]
);

CREATE TABLE IF NOT EXISTS support_conversations (
  id TEXT PRIMARY KEY,
  visitor_token_hash TEXT NOT NULL,
  visitor_ip_hash TEXT NOT NULL DEFAULT '',
  source_page TEXT NOT NULL DEFAULT '',
  referrer TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'lead_captured', 'closed')),
  lead_capture_state TEXT NOT NULL DEFAULT 'idle' CHECK (lead_capture_state IN ('idle', 'requested', 'captured')),
  detected_intent TEXT NOT NULL DEFAULT '',
  summary TEXT NOT NULL DEFAULT '',
  message_count INTEGER NOT NULL DEFAULT 0,
  last_message_at DATETIME,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS support_messages (
  id TEXT PRIMARY KEY,
  conversation_id TEXT NOT NULL,
  role TEXT NOT NULL CHECK (role IN ('visitor', 'assistant', 'system')),
  content TEXT NOT NULL,
  retrieval_status TEXT NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (conversation_id) REFERENCES support_conversations(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS support_answer_citations (
  id TEXT PRIMARY KEY,
  message_id TEXT NOT NULL,
  conversation_id TEXT NOT NULL,
  chunk_id TEXT NOT NULL,
  source_id TEXT NOT NULL,
  document_id TEXT NOT NULL,
  snippet TEXT NOT NULL DEFAULT '',
  distance REAL NOT NULL DEFAULT 0,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (message_id) REFERENCES support_messages(id) ON DELETE CASCADE,
  FOREIGN KEY (conversation_id) REFERENCES support_conversations(id) ON DELETE CASCADE,
  FOREIGN KEY (chunk_id) REFERENCES kb_chunks(id) ON DELETE CASCADE,
  FOREIGN KEY (source_id) REFERENCES kb_sources(id) ON DELETE CASCADE,
  FOREIGN KEY (document_id) REFERENCES kb_documents(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS support_feedback (
  id TEXT PRIMARY KEY,
  conversation_id TEXT NOT NULL,
  message_id TEXT NOT NULL,
  rating TEXT NOT NULL DEFAULT '',
  comment TEXT NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (conversation_id) REFERENCES support_conversations(id) ON DELETE CASCADE,
  FOREIGN KEY (message_id) REFERENCES support_messages(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS support_leads (
  id TEXT PRIMARY KEY,
  conversation_id TEXT NOT NULL,
  contact_email TEXT NOT NULL DEFAULT '',
  contact_phone TEXT NOT NULL DEFAULT '',
  name TEXT NOT NULL DEFAULT '',
  company TEXT NOT NULL DEFAULT '',
  need_description TEXT NOT NULL DEFAULT '',
  source_page TEXT NOT NULL DEFAULT '',
  detected_intent TEXT NOT NULL DEFAULT '',
  conversation_summary TEXT NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (conversation_id) REFERENCES support_conversations(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_kb_sources_enabled_status ON kb_sources(enabled, index_status);
CREATE INDEX IF NOT EXISTS idx_kb_documents_source ON kb_documents(source_id);
CREATE INDEX IF NOT EXISTS idx_kb_chunks_document ON kb_chunks(document_id, chunk_index);
CREATE INDEX IF NOT EXISTS idx_kb_chunks_enabled ON kb_chunks(enabled, embedding_status);
CREATE INDEX IF NOT EXISTS idx_support_conversations_visitor ON support_conversations(visitor_token_hash, status);
CREATE INDEX IF NOT EXISTS idx_support_conversations_updated ON support_conversations(updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_support_messages_conversation ON support_messages(conversation_id, created_at);
CREATE INDEX IF NOT EXISTS idx_support_citations_message ON support_answer_citations(message_id);
CREATE INDEX IF NOT EXISTS idx_support_leads_created ON support_leads(created_at DESC);

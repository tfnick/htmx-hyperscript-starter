package kb

import "context"

// Chunk represents a retrieved knowledge-base chunk with its metadata.
type Chunk struct {
	ChunkID    string
	SourceID   string
	DocumentID string
	SourceName string
	Content    string
	Score      float64
}

// Retriever searches for relevant chunks given a query embedding.
type Retriever interface {
	Search(ctx context.Context, queryEmbedding []float32, topK int, minScore float64) ([]Chunk, error)
}

// Indexer handles indexing a document into the knowledge base.
type Indexer interface {
	IndexDocument(ctx context.Context, documentID string) error
}

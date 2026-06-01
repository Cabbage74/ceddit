package rag

import "context"

// ChunkMeta holds metadata for a single chunk in the vector store.
type ChunkMeta struct {
	PostID      int64  `json:"post_id"`
	ChunkID     string `json:"chunk_id"` // e.g. "123#0"
	Position    int    `json:"position"`
	Title       string `json:"title"` // nearest parent heading
	ContentURL  string `json:"content_url,omitempty"`
	ContentSHA  string `json:"content_sha256,omitempty"`
	ContentETag string `json:"content_etag,omitempty"`
}

// Document is a chunk of text with its embedding vector and metadata.
type Document struct {
	Text      string    `json:"text"`
	Embedding []float64 `json:"embedding"`
	Meta      ChunkMeta `json:"metadata"`
}

// SearchResult is returned by similarity search.
type SearchResult struct {
	Doc   Document `json:"doc"`
	Score float64  `json:"score"`
}

// IndexMeta tracks the indexing state for a single post.
type IndexMeta struct {
	PostID      int64
	ContentSHA  string
	ContentETag string
	ChunkCount  int
}

// VectorStore is the interface for vector storage and similarity search.
// Implementations: MemoryVectorStore (testing), ESVectorStore (production).
type VectorStore interface {
	// Add inserts documents into the store.
	Add(ctx context.Context, docs []Document) error

	// DeleteByPostID removes all documents belonging to a post.
	DeleteByPostID(ctx context.Context, postID int64) error

	// Search performs similarity search, returning topK results filtered by postID.
	Search(ctx context.Context, queryEmbedding []float64, topK int, postID int64) ([]SearchResult, error)

	// HasPost checks if any documents exist for a post.
	HasPost(ctx context.Context, postID int64) (bool, error)

	// GetAnyDocForPost returns one document for a post (used for fingerprint checks).
	GetAnyDocForPost(ctx context.Context, postID int64) (*Document, error)
}

// ESDoc is the JSON structure stored in Elasticsearch.
type ESDoc struct {
	Text      string    `json:"text"`
	Embedding []float64 `json:"embedding"`
	Metadata  ChunkMeta `json:"metadata"`
}

// ESSearchHit is a single hit from an ES KNN search.
type ESSearchHit struct {
	Index  string  `json:"_index"`
	ID     string  `json:"_id"`
	Score  float64 `json:"_score"`
	Source ESDoc   `json:"_source"`
}

// ESSearchResponse is the response from an ES search.
type ESSearchResponse struct {
	Hits struct {
		Hits []ESSearchHit `json:"hits"`
	} `json:"hits"`
}

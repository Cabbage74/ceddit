package rag

import (
	"context"
	"math"
	"sort"
	"sync"
)

// MemoryVectorStore is an in-memory vector database with cosine similarity search.
// It implements the VectorStore interface. Useful for testing and development.
type MemoryVectorStore struct {
	mu   sync.RWMutex
	docs []Document
}

// NewMemoryVectorStore creates a new empty in-memory vector store.
func NewMemoryVectorStore() *MemoryVectorStore {
	return &MemoryVectorStore{
		docs: make([]Document, 0),
	}
}

// Add inserts documents into the store.
func (vs *MemoryVectorStore) Add(_ context.Context, docs []Document) error {
	if len(docs) == 0 {
		return nil
	}
	vs.mu.Lock()
	defer vs.mu.Unlock()
	vs.docs = append(vs.docs, docs...)
	return nil
}

// DeleteByPostID removes all documents belonging to a post.
func (vs *MemoryVectorStore) DeleteByPostID(_ context.Context, postID int64) error {
	vs.mu.Lock()
	defer vs.mu.Unlock()

	var kept []Document
	for _, d := range vs.docs {
		if d.Meta.PostID != postID {
			kept = append(kept, d)
		}
	}
	vs.docs = kept
	return nil
}

// Search performs cosine similarity search filtered by postID.
func (vs *MemoryVectorStore) Search(_ context.Context, queryEmbedding []float64, topK int, postID int64) ([]SearchResult, error) {
	vs.mu.RLock()
	defer vs.mu.RUnlock()

	type scored struct {
		doc   Document
		score float64
	}

	var candidates []scored

	for _, d := range vs.docs {
		if d.Meta.PostID != postID {
			continue
		}

		var score float64
		if len(queryEmbedding) > 0 && len(d.Embedding) > 0 {
			score = cosineSimilarity(queryEmbedding, d.Embedding)
		} else {
			score = 1.0 // no embedding, return all (caller should filter)
		}
		candidates = append(candidates, scored{doc: d, score: score})
	}

	// Sort by score descending.
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})

	if topK > len(candidates) {
		topK = len(candidates)
	}

	results := make([]SearchResult, topK)
	for i := 0; i < topK; i++ {
		results[i] = SearchResult{
			Doc:   candidates[i].doc,
			Score: candidates[i].score,
		}
	}

	return results, nil
}

// HasPost checks if any documents exist for a post.
func (vs *MemoryVectorStore) HasPost(_ context.Context, postID int64) (bool, error) {
	vs.mu.RLock()
	defer vs.mu.RUnlock()
	for _, d := range vs.docs {
		if d.Meta.PostID == postID {
			return true, nil
		}
	}
	return false, nil
}

// GetAnyDocForPost returns one document for a post (used for fingerprint checks).
func (vs *MemoryVectorStore) GetAnyDocForPost(_ context.Context, postID int64) (*Document, error) {
	vs.mu.RLock()
	defer vs.mu.RUnlock()
	for _, d := range vs.docs {
		if d.Meta.PostID == postID {
			doc := d // copy
			return &doc, nil
		}
	}
	return nil, nil
}

// DocCount returns the total number of documents in the store (testing helper, not in VectorStore interface).
func (vs *MemoryVectorStore) DocCount() int {
	vs.mu.RLock()
	defer vs.mu.RUnlock()
	return len(vs.docs)
}

// DocCountForPost returns the number of documents for a specific post (testing helper).
func (vs *MemoryVectorStore) DocCountForPost(postID int64) int {
	vs.mu.RLock()
	defer vs.mu.RUnlock()
	count := 0
	for _, d := range vs.docs {
		if d.Meta.PostID == postID {
			count++
		}
	}
	return count
}

// cosineSimilarity computes the cosine similarity between two vectors.
func cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) {
		return 0
	}

	var dot, normA, normB float64
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

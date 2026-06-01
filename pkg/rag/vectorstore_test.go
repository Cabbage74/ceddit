package rag

import (
	"context"
	"testing"
)

func TestVectorStoreAddAndCount(t *testing.T) {
	vs := NewMemoryVectorStore()
	if vs.DocCount() != 0 {
		t.Errorf("expected 0 docs, got %d", vs.DocCount())
	}

	docs := []Document{
		{
			Text:      "chunk one",
			Embedding: []float64{1.0, 0.0, 0.0},
			Meta:      ChunkMeta{PostID: 1, ChunkID: "1#0", Position: 0},
		},
		{
			Text:      "chunk two",
			Embedding: []float64{0.0, 1.0, 0.0},
			Meta:      ChunkMeta{PostID: 1, ChunkID: "1#1", Position: 1},
		},
	}

	if err := vs.Add(context.Background(), docs); err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	if vs.DocCount() != 2 {
		t.Errorf("expected 2 docs, got %d", vs.DocCount())
	}
	if vs.DocCountForPost(1) != 2 {
		t.Errorf("expected 2 docs for post 1, got %d", vs.DocCountForPost(1))
	}
}

func TestVectorStoreDeleteByPostID(t *testing.T) {
	vs := NewMemoryVectorStore()

	docs := []Document{
		{Text: "post 1 chunk 1", Meta: ChunkMeta{PostID: 1, ChunkID: "1#0"}},
		{Text: "post 1 chunk 2", Meta: ChunkMeta{PostID: 1, ChunkID: "1#1"}},
		{Text: "post 2 chunk 1", Meta: ChunkMeta{PostID: 2, ChunkID: "2#0"}},
	}
	vs.Add(context.Background(), docs)

	if err := vs.DeleteByPostID(context.Background(), 1); err != nil {
		t.Fatalf("DeleteByPostID failed: %v", err)
	}
	if vs.DocCount() != 1 {
		t.Errorf("expected 1 doc remaining, got %d", vs.DocCount())
	}
	if vs.DocCountForPost(1) != 0 {
		t.Errorf("expected 0 docs for post 1, got %d", vs.DocCountForPost(1))
	}
	if vs.DocCountForPost(2) != 1 {
		t.Errorf("expected 1 doc for post 2, got %d", vs.DocCountForPost(2))
	}
}

func TestVectorStoreDeleteNonexistent(t *testing.T) {
	vs := NewMemoryVectorStore()
	err := vs.DeleteByPostID(context.Background(), 999)
	if err != nil {
		t.Errorf("DeleteByPostID should not error for nonexistent post, got: %v", err)
	}
}

func TestVectorStoreSearch(t *testing.T) {
	vs := NewMemoryVectorStore()

	docs := []Document{
		{
			Text:      "aspirin pain relief",
			Embedding: []float64{1.0, 0.0, 0.0},
			Meta:      ChunkMeta{PostID: 1, ChunkID: "1#0", Position: 0},
		},
		{
			Text:      "aspirin side effects",
			Embedding: []float64{0.9, 0.1, 0.0},
			Meta:      ChunkMeta{PostID: 1, ChunkID: "1#1", Position: 1},
		},
		{
			Text:      "car engine maintenance",
			Embedding: []float64{0.0, 0.0, 1.0},
			Meta:      ChunkMeta{PostID: 1, ChunkID: "1#2", Position: 2},
		},
	}
	vs.Add(context.Background(), docs)

	queryVec := []float64{1.0, 0.0, 0.0}
	results, err := vs.Search(context.Background(), queryVec, 2, 1)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	if results[0].Score < results[1].Score {
		t.Errorf("first result should have higher score: %.4f vs %.4f",
			results[0].Score, results[1].Score)
	}

	t.Logf("search results:")
	for _, r := range results {
		t.Logf("  score=%.4f text=%s", r.Score, r.Doc.Text)
	}
}

func TestVectorStoreSearchFilterByPostID(t *testing.T) {
	vs := NewMemoryVectorStore()

	docs := []Document{
		{Text: "post 1", Embedding: []float64{1.0, 0.0}, Meta: ChunkMeta{PostID: 1, ChunkID: "1#0"}},
		{Text: "post 2", Embedding: []float64{1.0, 0.0}, Meta: ChunkMeta{PostID: 2, ChunkID: "2#0"}},
	}
	vs.Add(context.Background(), docs)

	queryVec := []float64{1.0, 0.0}
	results, err := vs.Search(context.Background(), queryVec, 10, 1)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result for post 1, got %d", len(results))
	}
	if results[0].Doc.Meta.PostID != 1 {
		t.Errorf("expected post 1, got post %d", results[0].Doc.Meta.PostID)
	}
}

func TestVectorStoreSearchTopKLimiting(t *testing.T) {
	vs := NewMemoryVectorStore()

	docs := make([]Document, 10)
	for i := 0; i < 10; i++ {
		docs[i] = Document{
			Text:      "chunk",
			Embedding: []float64{float64(i) * 0.1},
			Meta:      ChunkMeta{PostID: 1, ChunkID: FormatChunkID(1, i), Position: i},
		}
	}
	vs.Add(context.Background(), docs)

	results, err := vs.Search(context.Background(), []float64{1.0}, 3, 1)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
}

func TestHasPost(t *testing.T) {
	vs := NewMemoryVectorStore()

	ok, err := vs.HasPost(context.Background(), 1)
	if err != nil {
		t.Fatalf("HasPost failed: %v", err)
	}
	if ok {
		t.Error("expected HasPost(1) to be false for empty store")
	}

	vs.Add(context.Background(), []Document{
		{Text: "test", Meta: ChunkMeta{PostID: 1, ChunkID: "1#0"}},
	})

	ok, err = vs.HasPost(context.Background(), 1)
	if err != nil {
		t.Fatalf("HasPost failed: %v", err)
	}
	if !ok {
		t.Error("expected HasPost(1) to be true")
	}

	ok, err = vs.HasPost(context.Background(), 2)
	if err != nil {
		t.Fatalf("HasPost failed: %v", err)
	}
	if ok {
		t.Error("expected HasPost(2) to be false")
	}
}

func TestCosineSimilarity(t *testing.T) {
	sim := cosineSimilarity([]float64{1.0, 2.0, 3.0}, []float64{1.0, 2.0, 3.0})
	if sim < 0.9999 {
		t.Errorf("expected similarity ~1.0, got %.6f", sim)
	}

	sim = cosineSimilarity([]float64{1.0, 0.0}, []float64{0.0, 1.0})
	if sim > 0.0001 {
		t.Errorf("expected similarity ~0.0, got %.6f", sim)
	}

	sim = cosineSimilarity([]float64{1.0, 0.0}, []float64{-1.0, 0.0})
	if sim > -0.9999 || sim < -1.0001 {
		t.Errorf("expected similarity ~-1.0, got %.6f", sim)
	}

	sim = cosineSimilarity([]float64{1.0}, []float64{1.0, 2.0})
	if sim != 0 {
		t.Errorf("expected 0 for mismatched lengths, got %.6f", sim)
	}
}

func TestGetAnyDocForPost(t *testing.T) {
	vs := NewMemoryVectorStore()

	doc, err := vs.GetAnyDocForPost(context.Background(), 1)
	if err != nil {
		t.Fatalf("GetAnyDocForPost failed: %v", err)
	}
	if doc != nil {
		t.Error("expected nil doc for empty store")
	}

	vs.Add(context.Background(), []Document{
		{
			Text: "test content",
			Meta: ChunkMeta{
				PostID:      1,
				ChunkID:     "1#0",
				ContentSHA:  "abc123",
				ContentETag: "etag1",
			},
		},
	})

	doc, err = vs.GetAnyDocForPost(context.Background(), 1)
	if err != nil {
		t.Fatalf("GetAnyDocForPost failed: %v", err)
	}
	if doc == nil {
		t.Fatal("expected non-nil doc for existing post")
	}
	if doc.Meta.ContentSHA != "abc123" {
		t.Errorf("expected SHA 'abc123', got '%s'", doc.Meta.ContentSHA)
	}
}

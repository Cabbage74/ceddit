package rag

import (
	"context"
	"fmt"
	"testing"
)

// mockFetcher returns content for a given object key.
func mockFetcher(objectKey string) ([]byte, error) {
	if objectKey == "" {
		return nil, fmt.Errorf("empty object key")
	}

	content := `# 测试文档标题

这是用于测试的内容。

## 第一部分

这是第一部分的内容，用于验证索引构建逻辑。

## 第二部分

这是第二部分的内容，包含更多测试数据。

阿司匹林是一种常用的解热镇痛药。它的主要作用包括解热、镇痛和抗血小板。
`
	return []byte(content), nil
}

func TestIndexerEnsureIndexed(t *testing.T) {
	vs := NewMemoryVectorStore()
	emb := GetEmbedder() // may be nil in test
	idx := NewIndexer(vs, emb, mockFetcher)

	info := PostInfo{
		PostID:        100,
		ContentURL:    "posts/100/content.md",
		ContentSHA256: "sha256_abc123",
		ContentETag:   "etag_abc123",
		Status:        "published",
	}

	ctx := context.Background()
	n, err := idx.EnsureIndexed(ctx, info)
	if err != nil {
		t.Fatalf("EnsureIndexed failed: %v", err)
	}
	if n == 0 {
		t.Fatal("expected non-zero chunks indexed")
	}
	t.Logf("indexed %d chunks for post 100", n)

	ok, _ := vs.HasPost(ctx, 100)
	if !ok {
		t.Error("expected post 100 to be in vector store")
	}
}

func TestIndexerEnsureIndexedEmptyContentURL(t *testing.T) {
	vs := NewMemoryVectorStore()
	idx := NewIndexer(vs, nil, mockFetcher)

	info := PostInfo{
		PostID:     200,
		ContentURL: "",
		Status:     "published",
	}

	n, err := idx.EnsureIndexed(context.Background(), info)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0 chunks for empty content URL, got %d", n)
	}
}

func TestIndexerEnsureIndexedNonPublished(t *testing.T) {
	vs := NewMemoryVectorStore()
	idx := NewIndexer(vs, nil, mockFetcher)

	info := PostInfo{
		PostID:     300,
		ContentURL: "posts/300/content.md",
		Status:     "draft",
	}

	n, err := idx.EnsureIndexed(context.Background(), info)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0 chunks for draft post, got %d", n)
	}
	ok, _ := vs.HasPost(context.Background(), 300)
	if ok {
		t.Error("draft post should not be indexed")
	}
}

func TestIndexerFingerprintSkip(t *testing.T) {
	vs := NewMemoryVectorStore()
	idx := NewIndexer(vs, nil, mockFetcher)

	info := PostInfo{
		PostID:        400,
		ContentURL:    "posts/400/content.md",
		ContentSHA256: "sha256_unchanged",
		ContentETag:   "etag_unchanged",
		Status:        "published",
	}

	ctx := context.Background()

	// First call should index.
	n1, err := idx.EnsureIndexed(ctx, info)
	if err != nil {
		t.Fatalf("first indexing failed: %v", err)
	}
	if n1 == 0 {
		t.Fatal("expected non-zero chunks on first index")
	}

	// Second call with same fingerprint should skip.
	n2, err := idx.EnsureIndexed(ctx, info)
	if err != nil {
		t.Fatalf("second indexing failed: %v", err)
	}
	if n2 != 0 {
		t.Errorf("expected 0 chunks on second index (fingerprint match), got %d", n2)
	}
}

func TestIndexerRebuildOnChange(t *testing.T) {
	vs := NewMemoryVectorStore()
	idx := NewIndexer(vs, nil, mockFetcher)

	ctx := context.Background()

	// First index.
	info1 := PostInfo{
		PostID:        500,
		ContentURL:    "posts/500/content.md",
		ContentSHA256: "sha256_v1",
		ContentETag:   "etag_v1",
		Status:        "published",
	}
	n1, _ := idx.EnsureIndexed(ctx, info1)

	// Second index with different fingerprint should rebuild.
	info2 := PostInfo{
		PostID:        500,
		ContentURL:    "posts/500/content.md",
		ContentSHA256: "sha256_v2_changed",
		ContentETag:   "etag_v2_changed",
		Status:        "published",
	}

	n2, err := idx.EnsureIndexed(ctx, info2)
	if err != nil {
		t.Fatalf("rebuild indexing failed: %v", err)
	}
	if n2 == 0 {
		t.Fatal("expected non-zero chunks on rebuild")
	}

	if vs.DocCountForPost(500) != n2 {
		t.Errorf("expected %d chunks after rebuild, got %d (n1=%d)",
			n2, vs.DocCountForPost(500), n1)
	}
}

func TestSearchContexts(t *testing.T) {
	vs := NewMemoryVectorStore()
	idx := NewIndexer(vs, nil, mockFetcher)
	qa := NewQAService(vs, idx)

	ctx := context.Background()

	// Index a post first.
	info := PostInfo{
		PostID:     600,
		ContentURL: "posts/600/content.md",
		Status:     "published",
	}
	idx.EnsureIndexed(ctx, info)

	results, err := qa.SearchContexts(ctx, 600, "阿司匹林的作用", 3)
	if err != nil {
		t.Fatalf("SearchContexts failed: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected search results")
	}
	if len(results) > 3 {
		t.Errorf("expected at most 3 results, got %d", len(results))
	}

	t.Logf("search returned %d results", len(results))
	for i, r := range results {
		t.Logf("  result %d: score=%.4f pos=%d", i, r.Score, r.Doc.Meta.Position)
	}
}

func TestBuildContext(t *testing.T) {
	qa := &QAService{}

	results := []SearchResult{
		{
			Doc: Document{
				Text: "这是第一段上下文。",
				Meta: ChunkMeta{Title: "简介", Position: 0},
			},
			Score: 0.95,
		},
		{
			Doc: Document{
				Text: "这是第二段上下文。",
				Meta: ChunkMeta{Title: "方法", Position: 1},
			},
			Score: 0.80,
		},
	}

	ctx := qa.BuildContext(results)
	if ctx == "" {
		t.Fatal("context should not be empty")
	}

	t.Logf("built context:\n%s", ctx)
}

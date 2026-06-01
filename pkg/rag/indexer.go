package rag

import (
	"context"
	"fmt"

	"go.uber.org/zap"
)

// ContentFetcher retrieves post content by its COS object key.
type ContentFetcher func(objectKey string) ([]byte, error)

// PostInfo contains the metadata needed for indexing a post.
type PostInfo struct {
	PostID        int64
	ContentURL    string // COS object key
	ContentSHA256 string
	ContentETag   string
	Status        string
}

// Indexer manages building and updating the vector index for posts.
type Indexer struct {
	store    VectorStore
	embedder *Embedder
	fetch    ContentFetcher
}

// NewIndexer creates a new Indexer.
func NewIndexer(store VectorStore, embedder *Embedder, fetch ContentFetcher) *Indexer {
	return &Indexer{
		store:    store,
		embedder: embedder,
		fetch:    fetch,
	}
}

// EnsureIndexed makes sure a post has been indexed in the vector store.
// It checks fingerprints (SHA256/ETag) to avoid redundant rebuilds.
// Returns the number of chunks indexed (0 if already up-to-date or skipped).
func (idx *Indexer) EnsureIndexed(ctx context.Context, info PostInfo) (int, error) {
	// Only index published posts.
	if info.Status != "published" {
		zap.L().Debug("skipping non-published post for indexing",
			zap.Int64("post_id", info.PostID),
			zap.String("status", info.Status))
		return 0, nil
	}

	// Skip if no content URL.
	if info.ContentURL == "" {
		zap.L().Warn("post has no content URL, skipping index",
			zap.Int64("post_id", info.PostID))
		return 0, nil
	}

	// Check fingerprint: if already indexed with same content, skip.
	upToDate, err := idx.isUpToDate(ctx, info)
	if err != nil {
		zap.L().Warn("fingerprint check failed, will rebuild",
			zap.Int64("post_id", info.PostID),
			zap.Error(err))
	}
	if upToDate {
		zap.L().Debug("post index is up to date, skipping",
			zap.Int64("post_id", info.PostID))
		return 0, nil
	}

	// Content changed — rebuild index.
	zap.L().Info("rebuilding index for post",
		zap.Int64("post_id", info.PostID))

	// Fetch content.
	data, err := idx.fetch(info.ContentURL)
	if err != nil {
		return 0, fmt.Errorf("fetch content for post %d: %w", info.PostID, err)
	}

	text := string(data)
	if text == "" {
		zap.L().Warn("post content empty, skipping index",
			zap.Int64("post_id", info.PostID))
		return 0, nil
	}

	// Chunk the markdown.
	chunks := ChunkMarkdown(text)
	if len(chunks) == 0 {
		zap.L().Warn("no chunks produced for post",
			zap.Int64("post_id", info.PostID))
		return 0, nil
	}

	// Extract headings for metadata.
	headings := CollectHeadings(text)
	firstTitle := ExtractTitle(text)

	// Generate embeddings.
	var docs []Document
	if idx.embedder != nil {
		embeddings, err := idx.embedder.EmbedBatch(ctx, chunks)
		if err != nil {
			zap.L().Warn("embedding generation failed, storing without vectors",
				zap.Int64("post_id", info.PostID),
				zap.Error(err))
		} else {
			for i, chunk := range chunks {
				var emb []float64
				if i < len(embeddings) {
					emb = embeddings[i]
				}
				title := firstTitle
				if i < len(headings) {
					title = NearestHeading(headings, i, len(chunks))
				}
				docs = append(docs, Document{
					Text:      chunk,
					Embedding: emb,
					Meta: ChunkMeta{
						PostID:      info.PostID,
						ChunkID:     FormatChunkID(info.PostID, i),
						Position:    i,
						Title:       title,
						ContentURL:  info.ContentURL,
						ContentSHA:  info.ContentSHA256,
						ContentETag: info.ContentETag,
					},
				})
			}
		}
	}

	// If embedding failed and no docs created, create docs without embeddings.
	if len(docs) == 0 {
		for i, chunk := range chunks {
			title := firstTitle
			if i < len(headings) {
				title = NearestHeading(headings, i, len(chunks))
			}
			docs = append(docs, Document{
				Text: chunk,
				Meta: ChunkMeta{
					PostID:      info.PostID,
					ChunkID:     FormatChunkID(info.PostID, i),
					Position:    i,
					Title:       title,
					ContentURL:  info.ContentURL,
					ContentSHA:  info.ContentSHA256,
					ContentETag: info.ContentETag,
				},
			})
		}
	}

	// Delete old chunks and insert new ones (idempotent).
	if err := idx.store.DeleteByPostID(ctx, info.PostID); err != nil {
		return 0, fmt.Errorf("delete old chunks for post %d: %w", info.PostID, err)
	}
	if err := idx.store.Add(ctx, docs); err != nil {
		return 0, fmt.Errorf("add chunks for post %d: %w", info.PostID, err)
	}

	zap.L().Info("indexed post",
		zap.Int64("post_id", info.PostID),
		zap.Int("chunks", len(docs)))

	return len(docs), nil
}

// isUpToDate checks whether the existing index matches the current content.
func (idx *Indexer) isUpToDate(ctx context.Context, info PostInfo) (bool, error) {
	doc, err := idx.store.GetAnyDocForPost(ctx, info.PostID)
	if err != nil {
		return false, err
	}
	if doc == nil {
		return false, nil
	}

	// Compare SHA256 first, then ETag.
	if info.ContentSHA256 != "" && doc.Meta.ContentSHA != "" {
		return info.ContentSHA256 == doc.Meta.ContentSHA, nil
	}
	if info.ContentETag != "" && doc.Meta.ContentETag != "" {
		return info.ContentETag == doc.Meta.ContentETag, nil
	}

	return false, nil
}

// GetStore returns the underlying vector store.
func (idx *Indexer) GetStore() VectorStore {
	return idx.store
}

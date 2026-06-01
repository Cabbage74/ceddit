package rag

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"
)

// ESVectorStore implements VectorStore using Elasticsearch REST API.
// Uses net/http directly to avoid the heavy go-elasticsearch dependency,
// keeping compilation fast and memory usage low on small servers.
//
// ES index: dense_vector (1536-dim, cosine similarity, HNSW).
type ESVectorStore struct {
	baseURL   string
	indexName string
	embedDim  int
	client    *http.Client
}

// NewESVectorStore creates an ES-backed vector store. Call InitializeIndex after creation.
func NewESVectorStore(baseURL, indexName string, embedDim int) *ESVectorStore {
	return &ESVectorStore{
		baseURL:   strings.TrimRight(baseURL, "/"),
		indexName: indexName,
		embedDim:  embedDim,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// do sends an HTTP request to ES and returns the response.
func (es *ESVectorStore) do(ctx context.Context, method, path string, body []byte) (*http.Response, error) {
	url := es.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	return es.client.Do(req)
}

// doNoBody sends a request without a body.
func (es *ESVectorStore) doNoBody(ctx context.Context, method, path string) (*http.Response, error) {
	url := es.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	return es.client.Do(req)
}

// Ping checks ES connectivity by hitting the root endpoint.
func (es *ESVectorStore) Ping(ctx context.Context) error {
	resp, err := es.doNoBody(ctx, http.MethodGet, "/")
	if err != nil {
		return fmt.Errorf("ping ES: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("ES ping failed (status=%d): %s", resp.StatusCode, string(body))
	}
	return nil
}

// InitializeIndex creates the ES index with dense_vector mapping if it doesn't exist.
func (es *ESVectorStore) InitializeIndex(ctx context.Context) error {
	// Check if index already exists.
	resp, err := es.doNoBody(ctx, http.MethodHead, "/"+es.indexName)
	if err != nil {
		return fmt.Errorf("check index exists: %w", err)
	}
	resp.Body.Close()

	if resp.StatusCode == 200 {
		zap.L().Info("ES index already exists", zap.String("index", es.indexName))
		return nil
	}

	// Create index.
	mapping := map[string]any{
		"mappings": map[string]any{
			"properties": map[string]any{
				"text": map[string]any{"type": "text"},
				"embedding": map[string]any{
					"type":       "dense_vector",
					"dims":       es.embedDim,
					"similarity": "cosine",
					"index":      true,
					"index_options": map[string]any{
						"type":            "hnsw",
						"m":               16,
						"ef_construction": 200,
					},
				},
				"metadata": map[string]any{
					"properties": map[string]any{
						"post_id":        map[string]any{"type": "long"},
						"chunk_id":       map[string]any{"type": "keyword"},
						"position":       map[string]any{"type": "integer"},
						"title":          map[string]any{"type": "text"},
						"content_url":    map[string]any{"type": "keyword"},
						"content_sha256": map[string]any{"type": "keyword"},
						"content_etag":   map[string]any{"type": "keyword"},
					},
				},
			},
		},
	}

	body, _ := json.Marshal(mapping)
	resp, err = es.do(ctx, http.MethodPut, "/"+es.indexName, body)
	if err != nil {
		return fmt.Errorf("create index: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("create index failed (status=%d): %s", resp.StatusCode, string(respBody))
	}

	zap.L().Info("ES index created",
		zap.String("index", es.indexName),
		zap.Int("embedding_dim", es.embedDim))
	return nil
}

// Add bulk-indexes documents into the ES index.
func (es *ESVectorStore) Add(ctx context.Context, docs []Document) error {
	if len(docs) == 0 {
		return nil
	}

	var buf bytes.Buffer
	for _, doc := range docs {
		action := fmt.Sprintf(`{"index":{"_index":"%s","_id":"%s"}}`, es.indexName, doc.Meta.ChunkID)
		buf.WriteString(action)
		buf.WriteByte('\n')

		esDoc := ESDoc{Text: doc.Text, Embedding: doc.Embedding, Metadata: doc.Meta}
		docBytes, err := json.Marshal(esDoc)
		if err != nil {
			return fmt.Errorf("marshal doc %s: %w", doc.Meta.ChunkID, err)
		}
		buf.Write(docBytes)
		buf.WriteByte('\n')
	}

	resp, err := es.do(ctx, http.MethodPost, "/"+es.indexName+"/_bulk", buf.Bytes())
	if err != nil {
		return fmt.Errorf("bulk index: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("bulk index failed (status=%d): %s", resp.StatusCode, string(respBody))
	}

	var bulkResp struct {
		Errors bool `json:"errors"`
	}
	json.NewDecoder(resp.Body).Decode(&bulkResp)
	if bulkResp.Errors {
		return fmt.Errorf("bulk index had errors (check ES logs)")
	}

	return nil
}

// DeleteByPostID removes all documents for a given post using delete-by-query.
func (es *ESVectorStore) DeleteByPostID(ctx context.Context, postID int64) error {
	query := fmt.Sprintf(`{"query":{"term":{"metadata.post_id":%d}}}`, postID)

	resp, err := es.do(ctx, http.MethodPost, "/"+es.indexName+"/_delete_by_query?refresh=true",
		[]byte(query))
	if err != nil {
		return fmt.Errorf("delete by query: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete by query failed (status=%d): %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// Search performs KNN vector similarity search filtered by postID.
func (es *ESVectorStore) Search(ctx context.Context, queryEmbedding []float64, topK int, postID int64) ([]SearchResult, error) {
	if len(queryEmbedding) == 0 {
		return es.searchByPostID(ctx, topK, postID)
	}

	numCandidates := topK * 3
	if numCandidates < 20 {
		numCandidates = 20
	}

	// Build KNN query JSON manually for clarity and zero allocations.
	vecJSON, _ := json.Marshal(queryEmbedding)
	query := fmt.Sprintf(
		`{"knn":{"field":"embedding","query_vector":%s,"k":%d,"num_candidates":%d,`+
			`"filter":{"term":{"metadata.post_id":%d}}}}`,
		string(vecJSON), topK, numCandidates, postID)

	return es.doSearch(ctx, []byte(query))
}

func (es *ESVectorStore) searchByPostID(ctx context.Context, topK int, postID int64) ([]SearchResult, error) {
	query := fmt.Sprintf(`{"query":{"term":{"metadata.post_id":%d}},"size":%d}`, postID, topK)
	return es.doSearch(ctx, []byte(query))
}

func (es *ESVectorStore) doSearch(ctx context.Context, body []byte) ([]SearchResult, error) {
	resp, err := es.do(ctx, http.MethodPost, "/"+es.indexName+"/_search", body)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("search failed (status=%d): %s", resp.StatusCode, string(respBody))
	}

	return parseSearchResponse(resp.Body)
}

// HasPost checks if any documents exist for a post.
func (es *ESVectorStore) HasPost(ctx context.Context, postID int64) (bool, error) {
	query := fmt.Sprintf(`{"query":{"term":{"metadata.post_id":%d}},"size":0}`, postID)

	resp, err := es.do(ctx, http.MethodPost, "/"+es.indexName+"/_search", []byte(query))
	if err != nil {
		return false, fmt.Errorf("has post search: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Hits struct {
			Total struct {
				Value int `json:"value"`
			} `json:"total"`
		} `json:"hits"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, nil
	}

	return result.Hits.Total.Value > 0, nil
}

// GetAnyDocForPost returns one document for a post (used for fingerprint checks).
func (es *ESVectorStore) GetAnyDocForPost(ctx context.Context, postID int64) (*Document, error) {
	query := fmt.Sprintf(`{"query":{"term":{"metadata.post_id":%d}},"size":1}`, postID)

	resp, err := es.do(ctx, http.MethodPost, "/"+es.indexName+"/_search", []byte(query))
	if err != nil {
		return nil, fmt.Errorf("get any doc: %w", err)
	}
	defer resp.Body.Close()

	results, err := parseSearchResponse(resp.Body)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, nil
	}
	return &results[0].Doc, nil
}

// parseSearchResponse parses an ES search response into SearchResult slice.
func parseSearchResponse(body io.Reader) ([]SearchResult, error) {
	var esResp ESSearchResponse
	if err := json.NewDecoder(body).Decode(&esResp); err != nil {
		return nil, fmt.Errorf("decode search response: %w", err)
	}

	results := make([]SearchResult, len(esResp.Hits.Hits))
	for i, hit := range esResp.Hits.Hits {
		results[i] = SearchResult{
			Doc: Document{
				Text:      hit.Source.Text,
				Embedding: hit.Source.Embedding,
				Meta:      hit.Source.Metadata,
			},
			Score: hit.Score,
		}
	}
	return results, nil
}

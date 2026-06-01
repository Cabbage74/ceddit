package rag

import (
	"context"
	"fmt"
	"sync"

	"github.com/sashabaranov/go-openai"
	"github.com/spf13/viper"
)

// Embedder generates embedding vectors for text using an OpenAI-compatible API.
// Configured via rag.embedding.* in config.yaml (base_url, api_key, model, dimensions).
type Embedder struct {
	client *openai.Client
	model  string
	dims   int
	mu     sync.Mutex
}

var defaultEmbedder *Embedder

// InitEmbedder initializes the global embedder from config.
// Only activates if rag.embedding.base_url is explicitly configured.
// DeepSeek does NOT provide embeddings, so we never fall back to its config.
func InitEmbedder() {
	baseURL := viper.GetString("rag.embedding.base_url")
	if baseURL == "" {
		// No embedding provider configured — keyword-based retrieval will be used.
		defaultEmbedder = nil
		return
	}

	apiKey := viper.GetString("rag.embedding.api_key")
	model := viper.GetString("rag.embedding.model")
	if model == "" {
		model = "ecnu-embedding-small"
	}

	dims := viper.GetInt("rag.embedding.dimensions")
	if dims <= 0 {
		dims = 1024 // ecnu-embedding-small default
	}

	cfg := openai.DefaultConfig(apiKey)
	cfg.BaseURL = baseURL
	defaultEmbedder = &Embedder{
		client: openai.NewClientWithConfig(cfg),
		model:  model,
		dims:   dims,
	}
}

// GetEmbedder returns the global embedder, or nil if not initialized.
func GetEmbedder() *Embedder {
	return defaultEmbedder
}

// Embed generates an embedding vector for a single text.
func (e *Embedder) Embed(ctx context.Context, text string) ([]float64, error) {
	vecs, err := e.EmbedBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(vecs) == 0 {
		return nil, fmt.Errorf("embedder returned empty result")
	}
	return vecs[0], nil
}

// EmbedBatch generates embedding vectors for multiple texts in one API call.
func (e *Embedder) EmbedBatch(ctx context.Context, texts []string) ([][]float64, error) {
	if e == nil {
		return nil, fmt.Errorf("embedder not initialized")
	}
	if len(texts) == 0 {
		return nil, nil
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	resp, err := e.client.CreateEmbeddings(ctx, openai.EmbeddingRequest{
		Input: texts,
		Model: openai.EmbeddingModel(e.model),
	})
	if err != nil {
		return nil, fmt.Errorf("create embeddings: %w", err)
	}

	vecs := make([][]float64, len(resp.Data))
	for i, d := range resp.Data {
		v := make([]float64, len(d.Embedding))
		for j, f := range d.Embedding {
			v[j] = float64(f)
		}
		vecs[i] = v
	}

	return vecs, nil
}

// Dims returns the dimension of the embedding vectors.
func (e *Embedder) Dims() int {
	if e == nil {
		return 0
	}
	return e.dims
}

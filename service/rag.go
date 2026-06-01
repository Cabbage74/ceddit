package service

import (
	"ceddit/models"
	"ceddit/pkg/cos"
	"ceddit/pkg/rag"
	"ceddit/repository/mysql"
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/spf13/viper"
	"go.uber.org/zap"
)

// Global RAG components — initialized by InitRAG during app startup.
var (
	ragStore   rag.VectorStore
	ragIndexer *rag.Indexer
	ragQA      *rag.QAService
)

// InitRAG initializes the RAG subsystem. Must be called after MySQL, COS, and
// DeepSeek are initialized, and before any RAG requests are served.
//
// It first tries to connect to Elasticsearch (configured in config.yaml).
// If ES is unavailable, it falls back to an in-memory vector store.
func InitRAG() {
	// Initialize embedder (reads from config).
	rag.InitEmbedder()

	// Create content fetcher that reads from COS.
	fetcher := func(objectKey string) ([]byte, error) {
		return cos.GetObject(objectKey)
	}

	// Try to initialize Elasticsearch-backed vector store.
	store, err := initESStore()
	if err != nil {
		zap.L().Warn("Elasticsearch unavailable, falling back to in-memory vector store",
			zap.Error(err))
		store = rag.NewMemoryVectorStore()
	}

	ragStore = store

	// Create indexer.
	ragIndexer = rag.NewIndexer(ragStore, rag.GetEmbedder(), fetcher)

	// Create Q&A service.
	ragQA = rag.NewQAService(ragStore, ragIndexer)

	zap.L().Info("RAG subsystem initialized")
}

// initESStore creates and initializes an ES-backed vector store using raw HTTP.
func initESStore() (rag.VectorStore, error) {
	esHost := viper.GetString("elasticsearch.host")
	esPort := viper.GetInt("elasticsearch.port")
	if esPort == 0 {
		esPort = 9200
	}

	baseURL := fmt.Sprintf("http://%s:%d", esHost, esPort)
	indexName := viper.GetString("elasticsearch.index")
	if indexName == "" {
		indexName = "zhiguang-ai-index"
	}

	embedDim := 1536 // text-embedding-v4 dimension
	if emb := rag.GetEmbedder(); emb != nil {
		embedDim = emb.Dims()
	}

	store := rag.NewESVectorStore(baseURL, indexName, embedDim)

	// Quick connectivity check with a short timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := store.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ES ping: %w", err)
	}

	// Initialize the index with dense_vector mapping.
	if err := store.InitializeIndex(context.Background()); err != nil {
		return nil, fmt.Errorf("initialize ES index: %w", err)
	}

	zap.L().Info("Elasticsearch vector store initialized",
		zap.String("base_url", baseURL),
		zap.String("index", indexName),
		zap.Int("embedding_dim", embedDim))

	return store, nil
}

// EnsurePostIndexed builds or refreshes the vector index for a post if needed.
// Safe to call multiple times — uses fingerprint check to skip if unchanged.
func EnsurePostIndexed(postID int64) (int, error) {
	if ragIndexer == nil {
		return 0, fmt.Errorf("RAG indexer not initialized")
	}

	post, err := mysql.GetPostByID(postID)
	if err != nil {
		return 0, fmt.Errorf("get post %d: %w", postID, err)
	}

	info := rag.PostInfo{
		PostID: postID,
		Status: post.Status,
	}

	if post.ContentObjectKey != nil {
		info.ContentURL = *post.ContentObjectKey
	}
	if post.ContentSHA256 != nil {
		info.ContentSHA256 = *post.ContentSHA256
	}
	if post.ContentETag != nil {
		info.ContentETag = *post.ContentETag
	}

	return ragIndexer.EnsureIndexed(context.Background(), info)
}

// StreamPostAnswer generates a streaming RAG answer for a question about a post.
// Returns channels for content tokens and errors.
func StreamPostAnswer(ctx context.Context, postID int64, question string, topK, maxTokens int) (<-chan string, <-chan error) {
	if ragQA == nil {
		errCh := make(chan error, 1)
		contentCh := make(chan string)
		errCh <- fmt.Errorf("RAG service not initialized")
		close(errCh)
		close(contentCh)
		return contentCh, errCh
	}

	return ragQA.StreamAnswer(ctx, postID, question, topK, maxTokens)
}

// PostDetailForRAG fetches post detail needed for the RAG Q&A page context.
func PostDetailForRAG(postID int64) (*models.PostDetail, error) {
	return GetPost(postID)
}

// Ensure net/http is available.
var _ = http.MethodGet

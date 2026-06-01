package rag

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/sashabaranov/go-openai"
	"github.com/spf13/viper"
)

// QAService handles RAG-based question answering over indexed posts.
type QAService struct {
	store   VectorStore
	indexer *Indexer
	client  *openai.Client
	model   string
}

// NewQAService creates a new QAService.
func NewQAService(store VectorStore, indexer *Indexer) *QAService {
	cfg := openai.DefaultConfig(viper.GetString("deepseek.api_key"))
	cfg.BaseURL = viper.GetString("deepseek.base_url")

	return &QAService{
		store:   store,
		indexer: indexer,
		client:  openai.NewClientWithConfig(cfg),
		model:   viper.GetString("deepseek.model"),
	}
}

// SearchContexts retrieves relevant text chunks for a question.
// Uses wider recall (fetchK = topK * 3, min 20), then returns top topK results.
func (qa *QAService) SearchContexts(ctx context.Context, postID int64, question string, topK int) ([]SearchResult, error) {
	if topK <= 0 {
		topK = 5
	}

	// Get query embedding if embedder is available.
	var queryVec []float64
	if emb := GetEmbedder(); emb != nil {
		vec, err := emb.Embed(ctx, question)
		if err == nil {
			queryVec = vec
		}
	}

	// Wider recall first, then truncate to topK.
	fetchK := topK * 3
	if fetchK < 20 {
		fetchK = 20
	}

	results, err := qa.store.Search(ctx, queryVec, fetchK, postID)
	if err != nil {
		return nil, fmt.Errorf("search contexts: %w", err)
	}

	// Return top K.
	if len(results) > topK {
		results = results[:topK]
	}

	return results, nil
}

// BuildContext assembles search results into a single context string.
func (qa *QAService) BuildContext(results []SearchResult) string {
	if len(results) == 0 {
		return ""
	}

	var parts []string
	for i, r := range results {
		title := r.Doc.Meta.Title
		header := fmt.Sprintf("[片段 %d]", i+1)
		if title != "" {
			header = fmt.Sprintf("[片段 %d: %s]", i+1, title)
		}
		parts = append(parts, header+"\n"+r.Doc.Text)
	}

	return strings.Join(parts, "\n\n---\n\n")
}

// StreamAnswer generates a streaming answer using DeepSeek chat API.
// It returns a channel that receives token-by-token content.
// The channel is closed when the stream ends.
func (qa *QAService) StreamAnswer(ctx context.Context, postID int64, question string, topK, maxTokens int) (<-chan string, <-chan error) {
	contentCh := make(chan string, 64)
	errCh := make(chan error, 1)

	go func() {
		defer close(contentCh)
		defer close(errCh)

		// Retrieve relevant contexts.
		results, err := qa.SearchContexts(ctx, postID, question, topK)
		if err != nil {
			errCh <- fmt.Errorf("search contexts: %w", err)
			return
		}
		contextText := qa.BuildContext(results)

		// Build system prompt.
		system := "你是中文知识助手。只能依据提供的知文上下文回答用户问题；如果上下文中没有给出答案，请直接说明'不确定'或'没有相关信息'，不要编造内容。"

		// Build user prompt with context.
		var user string
		if contextText != "" {
			user = fmt.Sprintf("问题：%s\n\n上下文如下（可能不完整）：\n%s\n\n请基于以上上下文作答。", question, contextText)
		} else {
			user = fmt.Sprintf("问题：%s\n\n（没有找到相关上下文，请如实告知用户。）", question)
		}

		// Create streaming chat completion.
		stream, err := qa.client.CreateChatCompletionStream(ctx, openai.ChatCompletionRequest{
			Model: qa.model,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleSystem,
					Content: system,
				},
				{
					Role:    openai.ChatMessageRoleUser,
					Content: user,
				},
			},
			Temperature: 0.2,
			MaxTokens:   maxTokens,
			Stream:      true,
		})
		if err != nil {
			errCh <- fmt.Errorf("create chat stream: %w", err)
			return
		}
		defer stream.Close()

		// Read tokens from stream and send to channel.
		for {
			select {
			case <-ctx.Done():
				errCh <- ctx.Err()
				return
			default:
			}

			resp, err := stream.Recv()
			if err == io.EOF {
				return // stream completed normally
			}
			if err != nil {
				errCh <- fmt.Errorf("stream recv: %w", err)
				return
			}

			if len(resp.Choices) > 0 {
				delta := resp.Choices[0].Delta.Content
				if delta != "" {
					select {
					case contentCh <- delta:
					case <-ctx.Done():
						errCh <- ctx.Err()
						return
					}
				}
			}
		}
	}()

	return contentCh, errCh
}

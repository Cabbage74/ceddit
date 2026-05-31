package deepseek

import (
	"context"
	"fmt"
	"strings"

	"github.com/sashabaranov/go-openai"
	"github.com/spf13/viper"
)

var client *openai.Client

func Init() {
	config := openai.DefaultConfig(viper.GetString("deepseek.api_key"))
	config.BaseURL = viper.GetString("deepseek.base_url")

	client = openai.NewClientWithConfig(config)
}

func GenerateSummary(ctx context.Context, content string) (string, error) {
	if client == nil {
		return "", fmt.Errorf("DeepSeek client not initialized")
	}

	resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: viper.GetString("deepseek.model"),
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleUser,
				Content: fmt.Sprintf("请用50字以内概括以下文章的内容，只返回摘要文字，不要带任何前缀或解释：\n\n%s", content),
			},
		},
		MaxTokens: 100,
	})
	if err != nil {
		return "", fmt.Errorf("DeepSeek API call: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("DeepSeek returned empty response")
	}

	summary := strings.TrimSpace(resp.Choices[0].Message.Content)
	return summary, nil
}

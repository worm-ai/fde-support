package model

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"fde-support/internal/registry"
)

// modelCostPerToken is an approximate cost per token for basic usage estimation.
// For production, model-specific pricing should be configured via the manifest.
const modelCostPerToken = 0.000002
// OpenAIProvider implements Provider using the OpenAI-compatible chat completions API.
type OpenAIProvider struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

// NewOpenAIProvider creates an OpenAI-compatible provider.
func NewOpenAIProvider(apiKey string) *OpenAIProvider {
	return &OpenAIProvider{
		BaseURL: "https://api.openai.com/v1",
		APIKey:  apiKey,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// NewOpenAIProviderWithBase creates an OpenAI-compatible provider with a custom base URL.
func NewOpenAIProviderWithBase(baseURL, apiKey string) *OpenAIProvider {
	return &OpenAIProvider{
		BaseURL: baseURL,
		APIKey:  apiKey,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

type openaiChatRequest struct {
	Model     string          `json:"model"`
	Messages  []openaiMessage `json:"messages"`
	MaxTokens int             `json:"max_tokens,omitempty"`
}

type openaiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openaiChatResponse struct {
	ID      string         `json:"id"`
	Model   string         `json:"model"`
	Choices []openaiChoice `json:"choices"`
	Usage   openaiUsage    `json:"usage"`
}

type openaiChoice struct {
	Message openaiMessage `json:"message"`
}

type openaiUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

func (p *OpenAIProvider) Generate(ctx context.Context, req registry.ModelGenerateRequest) (registry.ModelGenerateResponse, error) {
	if err := ctx.Err(); err != nil {
		return registry.ModelGenerateResponse{}, err
	}

	msgs := make([]openaiMessage, len(req.Messages))
	for i, m := range req.Messages {
		msgs[i] = openaiMessage{Role: m.Role, Content: m.Content}
	}

	body := openaiChatRequest{
		Model:    req.Model,
		Messages: msgs,
		MaxTokens: req.MaxTokens,
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return registry.ModelGenerateResponse{}, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.BaseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return registry.ModelGenerateResponse{}, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.APIKey)

	resp, err := p.HTTPClient.Do(httpReq)
	if err != nil {
		return registry.ModelGenerateResponse{}, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errBody bytes.Buffer
		_, _ = errBody.ReadFrom(io.LimitReader(resp.Body, 4096))
		return registry.ModelGenerateResponse{}, fmt.Errorf("openai returned status %d: %s", resp.StatusCode, errBody.String())
	}

	var result openaiChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return registry.ModelGenerateResponse{}, fmt.Errorf("decode response: %w", err)
	}

	content := ""
	if len(result.Choices) > 0 {
		content = result.Choices[0].Message.Content
	}

	cost := float64(result.Usage.TotalTokens) * modelCostPerToken

	return registry.ModelGenerateResponse{
		Model:   result.Model,
		Content: content,
		Usage: registry.ModelUsageSummary{
			PromptTokens:     result.Usage.PromptTokens,
			CompletionTokens: result.Usage.CompletionTokens,
			TotalTokens:      result.Usage.TotalTokens,
			CostUSD:          cost,
		},
	}, nil
}

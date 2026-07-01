package model

import (
	"context"
	"strings"

	"fde-support/internal/registry"
)

// modelCostPerToken should match the constant in openai.go.
const mockCostPerToken = 0.000002
// MockProvider is a deterministic model provider for testing and CI.
type MockProvider struct {
	Responses map[string]string
}

// NewMockProvider creates a mock provider with default canned responses.
func NewMockProvider() *MockProvider {
	return &MockProvider{
		Responses: map[string]string{},
	}
}

func (p *MockProvider) Generate(ctx context.Context, req registry.ModelGenerateRequest) (registry.ModelGenerateResponse, error) {
	if err := ctx.Err(); err != nil {
		return registry.ModelGenerateResponse{}, err
	}

	promptTokens := 0
	userContent := ""
	for _, msg := range req.Messages {
		promptTokens += len(msg.Content) / 4
		if msg.Role == "user" {
			userContent = strings.ToLower(msg.Content)
		}
	}

	content := "This is a mock response from the mock model provider."
	for key, resp := range p.Responses {
		if strings.Contains(userContent, strings.ToLower(key)) {
			content = resp
			break
		}
	}

	completionTokens := len(content) / 4
	totalTokens := promptTokens + completionTokens
	cost := float64(totalTokens) * mockCostPerToken

	return registry.ModelGenerateResponse{
		Model:   req.Model,
		Content: content,
		Usage: registry.ModelUsageSummary{
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			TotalTokens:      totalTokens,
			CostUSD:          cost,
		},
	}, nil
}

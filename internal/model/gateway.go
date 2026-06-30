package model

import (
	"context"
	"fmt"
	"time"

	"fde-support/internal/registry"
)

// Gateway implements registry.ModelGateway with provider selection and fallback.
type Gateway struct {
	primary       Provider
	fallback      Provider
	defaultModel  string
	fallbackModel string
	maxLatencyMs  int
}

// Provider abstracts a model backend (OpenAI, mock, etc.).
type Provider interface {
	Generate(ctx context.Context, req registry.ModelGenerateRequest) (registry.ModelGenerateResponse, error)
}

// NewGateway creates a Gateway that implements registry.ModelGateway.
func NewGateway(primary Provider, fallback Provider, defaultModel, fallbackModel string, maxLatencyMs int) *Gateway {
	return &Gateway{
		primary:       primary,
		fallback:      fallback,
		defaultModel:  defaultModel,
		fallbackModel: fallbackModel,
		maxLatencyMs:  maxLatencyMs,
	}
}

// NewMockGateway returns a gateway backed by the mock provider (for testing/CI).
func NewMockGateway() registry.ModelGateway {
	return &Gateway{
		primary:      NewMockProvider(),
		defaultModel: "mock-model",
		maxLatencyMs: 30000,
	}
}

func (g *Gateway) Generate(ctx context.Context, req registry.ModelGenerateRequest) (registry.ModelGenerateResponse, error) {
	if req.Model == "" {
		req.Model = g.defaultModel
	}
	timeout := time.Duration(g.maxLatencyMs) * time.Millisecond
	if g.maxLatencyMs <= 0 {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	resp, err := g.primary.Generate(ctx, req)
	if err == nil {
		return resp, nil
	}
	if g.fallback != nil {
		fallbackReq := req
		if g.fallbackModel != "" {
			fallbackReq.Model = g.fallbackModel
		}
		return g.fallback.Generate(ctx, fallbackReq)
	}
	return registry.ModelGenerateResponse{}, fmt.Errorf("model generation failed: %w", err)
}

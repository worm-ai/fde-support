package app

import (
	"fmt"

	"fde-support/internal/environment"
	"fde-support/internal/model"
	"fde-support/internal/registry"
)

func buildModelGateway(env environment.ResolvedEnvironment, allowMock bool) (registry.ModelGateway, error) {
	if env.DefaultModel == "" {
		if allowMock {
			return model.NewMockGateway(), nil
		}
		return nil, fmt.Errorf("runtime.modelPolicy.defaultModel is required")
	}
	keyRef := env.ModelKeyRef
	if keyRef == "" {
		keyRef = "env:OPENAI_API_KEY"
	}
	apiKey, ok := env.ResolveSecret(keyRef)
	if !ok || apiKey == "" {
		if allowMock {
			return model.NewMockGateway(), nil
		}
		return nil, fmt.Errorf("model key not configured: %s", keyRef)
	}
	return model.NewGateway(
		model.NewOpenAIProvider(apiKey),
		nil,
		env.DefaultModel,
		env.FallbackModel,
		env.MaxLatencyMs,
	), nil
}

package app

import (
	"strings"
	"testing"

	"fde-support/internal/environment"
)

func TestBuildModelGatewayFailsWithoutDefaultModel(t *testing.T) {
	_, err := buildModelGateway(environment.ResolvedEnvironment{}, false)
	if err == nil || !strings.Contains(err.Error(), "runtime.modelPolicy.defaultModel is required") {
		t.Fatalf("buildModelGateway() error = %v, want default model error", err)
	}
}

func TestBuildModelGatewayFailsWithoutModelKey(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	_, err := buildModelGateway(environment.ResolvedEnvironment{DefaultModel: "gpt-4.1"}, false)
	if err == nil || !strings.Contains(err.Error(), "model key not configured") {
		t.Fatalf("buildModelGateway() error = %v, want model key error", err)
	}
}

func TestBuildModelGatewayAllowsMockWhenExplicitlyAllowed(t *testing.T) {
	gateway, err := buildModelGateway(environment.ResolvedEnvironment{}, true)
	if err != nil {
		t.Fatalf("buildModelGateway() error = %v", err)
	}
	if gateway == nil {
		t.Fatalf("buildModelGateway() gateway = nil")
	}
}

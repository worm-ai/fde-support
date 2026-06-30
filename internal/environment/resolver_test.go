package environment

import (
	"testing"

	"fde-support/internal/manifest"
)

func TestResolve_DefaultValues(t *testing.T) {
	m := &manifest.SolutionManifest{
		BaseDir: "/tmp/test",
		Runtime: manifest.RuntimeSpec{
			ModelPolicy: manifest.ModelPolicySpec{
				DefaultModel:  "gpt-4.1",
				FallbackModel: "gpt-4.1-mini",
				MaxLatencyMs:  8000,
			},
			Observability: manifest.ObservabilitySpec{
				RetainDays: 7,
			},
		},
		Delivery: manifest.DeliverySpec{
			Environments: []manifest.EnvironmentSpec{
				{Name: "poc", Type: "shared_sandbox", Config: map[string]any{}},
			},
		},
	}

	env, err := Resolve(m, "poc")
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	if env.EnvironmentName != "poc" {
		t.Errorf("EnvironmentName = %q, want %q", env.EnvironmentName, "poc")
	}
	if env.EnvironmentType != "shared_sandbox" {
		t.Errorf("EnvironmentType = %q, want %q", env.EnvironmentType, "shared_sandbox")
	}
	if env.DefaultModel != "gpt-4.1" {
		t.Errorf("DefaultModel = %q, want %q", env.DefaultModel, "gpt-4.1")
	}
	if env.MaxLatencyMs != 8000 {
		t.Errorf("MaxLatencyMs = %d, want %d", env.MaxLatencyMs, 8000)
	}
	if env.RetainDays != 7 {
		t.Errorf("RetainDays = %d, want %d", env.RetainDays, 7)
	}
	if env.TracePath == "" {
		t.Error("TracePath should not be empty")
	}
}

func TestResolve_EnvironmentOverride(t *testing.T) {
	m := &manifest.SolutionManifest{
		BaseDir: "/tmp/test",
		Runtime: manifest.RuntimeSpec{
			ModelPolicy: manifest.ModelPolicySpec{
				DefaultModel: "gpt-4.1",
				MaxLatencyMs: 8000,
			},
			Observability: manifest.ObservabilitySpec{
				RetainDays: 7,
			},
		},
		Delivery: manifest.DeliverySpec{
			Environments: []manifest.EnvironmentSpec{
				{Name: "production", Type: "dedicated", Config: map[string]any{
					"defaultModel": "gpt-4.1-mini",
					"maxLatencyMs": 12000,
					"retainDays":   30,
				}},
			},
		},
	}

	env, err := Resolve(m, "production")
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	if env.DefaultModel != "gpt-4.1-mini" {
		t.Errorf("DefaultModel = %q, want %q", env.DefaultModel, "gpt-4.1-mini")
	}
	if env.MaxLatencyMs != 12000 {
		t.Errorf("MaxLatencyMs = %d, want %d", env.MaxLatencyMs, 12000)
	}
	if env.RetainDays != 30 {
		t.Errorf("RetainDays = %d, want %d", env.RetainDays, 30)
	}
}

func TestResolve_MissingEnvironment(t *testing.T) {
	m := &manifest.SolutionManifest{
		Delivery: manifest.DeliverySpec{
			Environments: []manifest.EnvironmentSpec{
				{Name: "poc", Type: "shared_sandbox"},
			},
		},
	}
	_, err := Resolve(m, "production")
	if err == nil {
		t.Fatal("expected error for missing environment")
	}
}

func TestResolveSecret(t *testing.T) {
	t.Setenv("TEST_KEY", "test-value")
	env := ResolvedEnvironment{}
	val, ok := env.ResolveSecret("env:TEST_KEY")
	if !ok {
		t.Fatal("expected secret to resolve")
	}
	if val != "test-value" {
		t.Errorf("ResolveSecret = %q, want %q", val, "test-value")
	}
}

func TestResolveSecret_NotEnvRef(t *testing.T) {
	env := ResolvedEnvironment{}
	_, ok := env.ResolveSecret("plaintext-token")
	if ok {
		t.Error("plaintext token should not resolve")
	}
}

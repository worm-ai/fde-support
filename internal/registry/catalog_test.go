package registry

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestBuiltinComponentRegistryFromRootLoadsDescriptorsAndFactories(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeRegistryFile(t, root, "components/registry/intent/beverage-router/1.0.0/component.yaml", `ref: registry.intent.beverage-router@1.0.0
category: processor
factory: intent_classifier
configSchema:
  intents: array?
inputSchema:
  message: string
outputSchema:
  intent: string
  confidence: number
`)

	reg, err := NewBuiltinComponentRegistryFromRoot(root)
	if err != nil {
		t.Fatalf("NewBuiltinComponentRegistryFromRoot() error = %v", err)
	}

	desc, err := reg.Resolve("registry.intent.beverage-router@1.0.0")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if desc.Factory != "intent_classifier" {
		t.Fatalf("Factory = %q, want intent_classifier", desc.Factory)
	}
	if desc.Category != CategoryProcessor {
		t.Fatalf("Category = %q, want %q", desc.Category, CategoryProcessor)
	}

	component, err := reg.Instantiate("intent_classifier", desc.Ref, map[string]any{"intents": []any{"troubleshooting"}})
	if err != nil {
		t.Fatalf("Instantiate() error = %v", err)
	}
	out, err := component.Run(context.Background(), map[string]any{"message": "Need help with the pump"}, nil)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got := out["intent"]; got != "troubleshooting" {
		t.Fatalf("intent = %#v, want troubleshooting", got)
	}
}

func writeRegistryFile(t *testing.T, root, rel, content string) {
	t.Helper()

	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%s) error = %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", path, err)
	}
}

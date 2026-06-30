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

func TestBuiltinRegistryContainsM2Components(t *testing.T) {
	t.Parallel()

	reg := NewBuiltinComponentRegistry()
	refs := []string{
		"registry.processor.llm-extractor@1.0.0",
		"registry.processor.data-query@1.0.0",
		"registry.processor.rule-evaluator@1.0.0",
		"registry.action.http-caller@1.0.0",
	}
	for _, ref := range refs {
		ref := ref
		t.Run(ref, func(t *testing.T) {
			desc, err := reg.Resolve(ref)
			if err != nil {
				t.Fatalf("Resolve(%s) error = %v", ref, err)
			}
			if desc.Factory == "" {
				t.Fatalf("Resolve(%s).Factory is empty", ref)
			}
			if _, err := reg.Instantiate("component", ref, minimalM2Config(ref)); err != nil {
				t.Fatalf("Instantiate(%s) error = %v", ref, err)
			}
		})
	}
}

func TestHTTPCallerUsesRuntimeHTTPCapability(t *testing.T) {
	t.Parallel()

	caller := &recordingHTTPCaller{}
	component := newHTTPCaller("notify", map[string]any{
		"url":          "https://example.test/{{id}}",
		"method":       "POST",
		"bodyTemplate": `{"id":"{{id}}"}`,
	})
	output, err := component.Run(context.Background(), map[string]any{"id": "A-1"}, stubRegistryRuntime{http: caller})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !caller.called {
		t.Fatalf("http caller was not invoked")
	}
	if caller.req.URL != "https://example.test/A-1" {
		t.Fatalf("URL = %q, want templated URL", caller.req.URL)
	}
	if caller.req.Body != `{"id":"A-1"}` {
		t.Fatalf("Body = %q, want templated body", caller.req.Body)
	}
	if output["status"] != "ok" || output["statusCode"] != float64(201) {
		t.Fatalf("output = %#v, want ok 201", output)
	}
}

func minimalM2Config(ref string) map[string]any {
	switch ref {
	case "registry.processor.llm-extractor@1.0.0":
		return map[string]any{"schema": map[string]any{}}
	case "registry.processor.data-query@1.0.0":
		return map[string]any{"source": "product_data"}
	case "registry.processor.rule-evaluator@1.0.0":
		return map[string]any{"rules": []any{}}
	case "registry.action.http-caller@1.0.0":
		return map[string]any{"url": "https://example.test"}
	default:
		return nil
	}
}

type recordingHTTPCaller struct {
	called bool
	req    HTTPCallRequest
}

func (c *recordingHTTPCaller) Call(ctx context.Context, req HTTPCallRequest) (HTTPCallResponse, error) {
	c.called = true
	c.req = req
	return HTTPCallResponse{StatusCode: 201, Body: `{"ok":true}`}, nil
}

type stubRegistryRuntime struct {
	http HTTPCaller
}

func (r stubRegistryRuntime) Environment() string             { return "test" }
func (r stubRegistryRuntime) Knowledge() KnowledgeReader      { return nil }
func (r stubRegistryRuntime) Request() RuntimeRequestMetadata { return RuntimeRequestMetadata{} }
func (r stubRegistryRuntime) Error() *RuntimeErrorSummary     { return nil }
func (r stubRegistryRuntime) Actions() []ActionSummary        { return nil }
func (r stubRegistryRuntime) Model() ModelGateway             { return nil }
func (r stubRegistryRuntime) HTTP() HTTPCaller                { return r.http }
func (r stubRegistryRuntime) Logger() Logger                  { return nil }

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

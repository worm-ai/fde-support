package runtimecore

import (
	"context"
	"fmt"
	"testing"

	"fde-support/internal/environment"
	"fde-support/internal/manifest"
	"fde-support/internal/registry"
	"fde-support/internal/trace"
)

func TestExecutorInjectsHTTPGatewayIntoRuntimeContext(t *testing.T) {
	m := &manifest.SolutionManifest{
		Metadata: manifest.MetadataSpec{Name: "s", Version: "1"},
		Components: []manifest.ComponentSpec{{
			ID:       "http_checker",
			Category: "processor",
			Ref:      "registry.test.http-checker@1.0.0",
		}},
		Workflow: manifest.WorkflowSpec{
			Entrypoint: "check",
			InputMapping: map[string]map[string]string{
				"chat": {"message": "request.message"},
			},
			Nodes: []manifest.WorkflowNodeSpec{{
				ID:        "check",
				Component: "http_checker",
				Inputs:    map[string]string{"message": "inputs.message"},
			}},
		},
	}
	executor, err := NewExecutor(
		m,
		environment.ResolvedEnvironment{EnvironmentName: "poc", MaxLatencyMs: 1000},
		httpCheckRegistry{},
		nil,
		trace.NewFileTraceWriter(t.TempDir()),
		nil,
		stubRuntimeHTTPCaller{},
	)
	if err != nil {
		t.Fatalf("NewExecutor() error = %v", err)
	}

	response, _, err := executor.Execute(context.Background(), RuntimeRequest{
		Trigger: TriggerSpec{Type: "chat"},
		Request: map[string]any{"message": "hello"},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if response["answer"] != "ok" {
		t.Fatalf("answer = %#v, want ok", response["answer"])
	}
}

type httpCheckRegistry struct{}

func (httpCheckRegistry) Resolve(ref string) (registry.ComponentDescriptor, error) {
	if ref != "registry.test.http-checker@1.0.0" {
		return registry.ComponentDescriptor{}, fmt.Errorf("unknown ref %s", ref)
	}
	return registry.ComponentDescriptor{
		Ref:          ref,
		Category:     registry.CategoryProcessor,
		Factory:      "http_checker",
		InputSchema:  map[string]string{"message": "string"},
		OutputSchema: map[string]string{"answer": "string"},
	}, nil
}

func (httpCheckRegistry) Instantiate(id string, ref string, config map[string]any) (registry.Component, error) {
	return httpCheckComponent{id: id}, nil
}

type httpCheckComponent struct {
	id string
}

func (c httpCheckComponent) ID() string {
	return c.id
}

func (c httpCheckComponent) Category() registry.ComponentCategory {
	return registry.CategoryProcessor
}

func (c httpCheckComponent) Run(ctx context.Context, input map[string]any, runtime registry.RuntimeContext) (map[string]any, error) {
	if runtime.HTTP() == nil {
		return nil, fmt.Errorf("http gateway missing")
	}
	return map[string]any{"answer": "ok"}, nil
}

type stubRuntimeHTTPCaller struct{}

func (stubRuntimeHTTPCaller) Call(ctx context.Context, req registry.HTTPCallRequest) (registry.HTTPCallResponse, error) {
	return registry.HTTPCallResponse{StatusCode: 200}, nil
}

package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"fde-support/internal/api"
	"fde-support/internal/environment"
	"fde-support/internal/knowledge"
	"fde-support/internal/manifest"
	"fde-support/internal/registry"
	"fde-support/internal/runtimecore"
	"fde-support/internal/trace"
	"fde-support/internal/w2a"
)

type ValidateResult struct {
	Status string                     `json:"status"`
	Errors []manifest.ValidationError `json:"errors,omitempty"`
}

type RuntimeApp struct {
	Manifest    *manifest.SolutionManifest
	Environment environment.ResolvedEnvironment
	Knowledge   *knowledge.Store
	TraceWriter *trace.FileTraceWriter
	Executor    *runtimecore.Executor
	HTTPServer  *api.Server
}

func ValidateManifestFile(path string) (*ValidateResult, error) {
	m, err := manifest.LoadFile(path)
	if err != nil {
		return nil, err
	}
	errs := manifest.NewValidator(registry.NewBuiltinComponentRegistry(), w2a.NewBuiltinSensorRegistry()).Validate(m)
	if len(errs) > 0 {
		return &ValidateResult{Status: "failed", Errors: errs}, nil
	}
	return &ValidateResult{Status: "ok"}, nil
}

func BuildRuntime(ctx context.Context, manifestPath string, envName string) (*RuntimeApp, error) {
	m, err := manifest.LoadFile(manifestPath)
	if err != nil {
		return nil, err
	}
	componentRegistry, err := registry.NewBuiltinComponentRegistryFromRoot(m.BaseDir)
	if err != nil {
		return nil, err
	}
	sensorRegistry := w2a.NewBuiltinSensorRegistry()
	errs := manifest.NewValidator(componentRegistry, sensorRegistry).Validate(m)
	if len(errs) > 0 {
		bytes, _ := json.Marshal(errs)
		return nil, fmt.Errorf("manifest validation failed: %s", string(bytes))
	}
	resolvedEnv, err := environment.Resolve(m, envName)
	if err != nil {
		return nil, err
	}
	knowledgeStore, _, err := knowledge.Load(ctx, m, resolvedEnv)
	if err != nil {
		return nil, err
	}
	traceWriter := trace.NewFileTraceWriter(resolvedEnv.TracePath)
	executor, err := runtimecore.NewExecutor(m, resolvedEnv, componentRegistry, knowledgeStore, traceWriter)
	if err != nil {
		return nil, err
	}
	server := api.NewServer(m, resolvedEnv, executor, w2a.NewMemorySignalIdempotencyStore(), traceWriter)
	return &RuntimeApp{
		Manifest:    m,
		Environment: resolvedEnv,
		Knowledge:   knowledgeStore,
		TraceWriter: traceWriter,
		Executor:    executor,
		HTTPServer:  server,
	}, nil
}

func RunHTTP(ctx context.Context, manifestPath, envName, addr string) error {
	runtimeApp, err := BuildRuntime(ctx, manifestPath, envName)
	if err != nil {
		return err
	}
	server := &http.Server{
		Addr:              addr,
		Handler:           runtimeApp.HTTPServer.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ListenAndServe()
	}()
	fmt.Printf("solution runtime listening on http://%s\n", addr)
	fmt.Printf("environment=%s tracePath=%s\n", runtimeApp.Environment.EnvironmentName, runtimeApp.Environment.TracePath)
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
		return ctx.Err()
	case err := <-errCh:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}

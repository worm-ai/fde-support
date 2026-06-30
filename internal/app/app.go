package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"fde-support/internal/api"
	"fde-support/internal/delivery"
	"fde-support/internal/environment"
	"fde-support/internal/evaluation"
	"fde-support/internal/knowledge"
	"fde-support/internal/manifest"
	"fde-support/internal/model"
	"fde-support/internal/registry"
	"fde-support/internal/release"
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

// defaultHTTPCaller is a basic HTTP caller implementation.
type defaultHTTPCaller struct{}

func (d *defaultHTTPCaller) Call(ctx context.Context, req registry.HTTPCallRequest) (registry.HTTPCallResponse, error) {
	method := req.Method
	if method == "" {
		method = http.MethodGet
	}
	httpReq, err := http.NewRequestWithContext(ctx, method, req.URL, strings.NewReader(req.Body))
	if err != nil {
		return registry.HTTPCallResponse{}, err
	}
	for k, v := range req.Headers {
		httpReq.Header.Set(k, v)
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return registry.HTTPCallResponse{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return registry.HTTPCallResponse{}, err
	}
	headers := map[string]string{}
	for key, values := range resp.Header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}
	return registry.HTTPCallResponse{StatusCode: resp.StatusCode, Body: string(body), Headers: headers}, nil
}

func SignalContext() context.Context {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-ctx.Done()
		stop()
	}()
	return ctx
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
	modelGateway := model.NewMockGateway()
	httpGateway := &defaultHTTPCaller{}
	executor, err := runtimecore.NewExecutor(m, resolvedEnv, componentRegistry, knowledgeStore, traceWriter, modelGateway, httpGateway)
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

func IngestManifestFile(path string) (*knowledge.IngestReport, error) {
	m, err := manifest.LoadFile(path)
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
	resolvedEnv, err := environment.Resolve(m, "poc")
	if err != nil {
		return nil, err
	}
	return knowledge.Ingest(context.Background(), m, resolvedEnv)
}

func EvaluateManifestFile(path string) (*evaluation.EvalReport, error) {
	m, err := manifest.LoadFile(path)
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
	resolvedEnv, err := environment.Resolve(m, "poc")
	if err != nil {
		return nil, err
	}
	knowledgeStore, _, err := knowledge.Load(context.Background(), m, resolvedEnv)
	if err != nil {
		return nil, err
	}
	traceWriter := trace.NewFileTraceWriter(resolvedEnv.TracePath)
	modelGateway := model.NewMockGateway()
	httpGateway := &defaultHTTPCaller{}
	executor, err := runtimecore.NewExecutor(m, resolvedEnv, componentRegistry, knowledgeStore, traceWriter, modelGateway, httpGateway)
	if err != nil {
		return nil, err
	}
	metricRegistry := evaluation.NewMetricRegistry()
	runner := evaluation.NewRunner(executor, m, metricRegistry)
	ctx := context.Background()
	for _, ds := range m.Evaluation.Datasets {
		if ds.URI == "" {
			continue
		}
		resolved := ds.URI
		if !filepath.IsAbs(resolved) {
			resolved = filepath.Join(filepath.Dir(path), resolved)
		}
		report, err := runner.Run(ctx, resolved, m.Evaluation.Gates)
		if err != nil {
			return nil, err
		}
		return report, nil
	}
	return nil, fmt.Errorf("no evaluation datasets configured")
}

func ReleaseManifestFile(path, envName string) (*release.ReleaseReport, error) {
	m, err := manifest.LoadFile(path)
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
	knowledgeStore, _, err := knowledge.Load(context.Background(), m, resolvedEnv)
	if err != nil {
		return nil, err
	}
	traceWriter := trace.NewFileTraceWriter(resolvedEnv.TracePath)
	executor, err := runtimecore.NewExecutor(m, resolvedEnv, componentRegistry, knowledgeStore, traceWriter, model.NewMockGateway(), &defaultHTTPCaller{})
	if err != nil {
		return nil, err
	}
	runner := evaluation.NewRunner(executor, m, evaluation.NewMetricRegistry())
	checker := release.NewCheckerWithEvaluator(m, resolvedEnv, runner)
	report, err := checker.Run(context.Background())
	if err != nil {
		return nil, err
	}
	if report.Passed {
		outputDir := filepath.Join(filepath.Dir(path), "deploy", envName)
		if err := delivery.GenerateDockerCompose(m, resolvedEnv, outputDir); err != nil {
			return nil, fmt.Errorf("generate deployment artifacts: %w", err)
		}
		fmt.Fprintf(os.Stderr, "deployment artifacts generated at %s\n", outputDir)
	}
	return report, nil
}

func PublishComponentFile(componentDir string) (string, error) {
	outputDir := filepath.Join(componentDir, "..", "published")
	return registry.PublishComponent(componentDir, outputDir)
}

func ResolveTemplatePath(name string) (string, error) {
	templateDir := filepath.Join(findProjectRoot(), "templates")
	path := filepath.Join(templateDir, name+".yaml")
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("template %q not found at %s", name, path)
	}
	return path, nil
}

func findProjectRoot() string {
	if wd, err := os.Getwd(); err == nil {
		return wd
	}
	return "."
}

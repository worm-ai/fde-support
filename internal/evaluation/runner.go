package evaluation

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"fde-support/internal/manifest"
	"fde-support/internal/runtimecore"
)

// Runner executes golden cases against an in-process executor.
type Runner struct {
	executor runtimecore.ExecutorLike
	manifest *manifest.SolutionManifest
	registry *MetricRegistry
}

// NewRunner creates an evaluation runner.
func NewRunner(exec runtimecore.ExecutorLike, m *manifest.SolutionManifest, registry *MetricRegistry) *Runner {
	return &Runner{
		executor: exec,
		manifest: m,
		registry: registry,
	}
}

// Run executes all golden cases from a JSONL file and produces an evaluation report.
func (r *Runner) Run(ctx context.Context, datasetURI string, gates []manifest.EvaluationGateSpec) (*EvalReport, error) {
	cases, err := loadGoldenCases(datasetURI)
	if err != nil {
		return nil, err
	}
	if len(cases) == 0 {
		return nil, fmt.Errorf("evaluation dataset is empty, at least one golden case is required")
	}

	report := &EvalReport{
		Solution:   r.manifest.Metadata.Name,
		Version:    r.manifest.Metadata.Version,
		DatasetURI: datasetURI,
		TotalCases: len(cases),
	}

	metricCounts := map[string]int{}
	for _, gc := range cases {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		result := r.runCase(ctx, gc)
		report.Results = append(report.Results, result)
		if result.Passed {
			report.PassedCases++
		}
		for name, value := range result.Metrics {
			if prev, ok := report.Metrics[name]; ok {
				report.Metrics[name] = prev + value
			} else {
				report.Metrics[name] = value
			}
			metricCounts[name]++
		}
	}

	// Average the metric values
	for name, total := range report.Metrics {
		if count := metricCounts[name]; count > 0 {
			report.Metrics[name] = total / float64(count)
		}
	}

	// Check gates
	for _, gate := range gates {
		actual, ok := report.Metrics[gate.Metric]
		if !ok {
			report.GateResults = append(report.GateResults, GateResult{
				Metric:   gate.Metric,
				Min:      gate.Min,
				Actual:   0,
				Severity: gate.Severity,
				Schedule: gate.Schedule,
				Passed:   false,
			})
			if gate.Severity == "block" && gate.Schedule == "onRelease" {
				report.Warnings = append(report.Warnings, fmt.Sprintf("gate %s failed: metric not found", gate.Metric))
			}
			continue
		}
		passed := actual >= gate.Min
		report.GateResults = append(report.GateResults, GateResult{
			Metric:   gate.Metric,
			Min:      gate.Min,
			Actual:   actual,
			Severity: gate.Severity,
			Schedule: gate.Schedule,
			Passed:   passed,
		})
		if !passed && gate.Severity == "block" {
			report.Warnings = append(report.Warnings,
				fmt.Sprintf("gate %s failed: %.2f < %.2f (severity: %s)", gate.Metric, actual, gate.Min, gate.Severity))
		}
	}

	return report, nil
}

func (r *Runner) runCase(ctx context.Context, gc GoldenCase) EvalResult {
	result := EvalResult{
		CaseID:  gc.ID,
		Metrics: map[string]float64{},
	}

	req := runtimecore.RuntimeRequest{
		Trigger: gc.Trigger,
	}
	if gc.Trigger.Type == "w2a_signal" {
		req.Signal = gc.RawPayload
		req.RawPayload = gc.RawPayload
	} else {
		req.Request = gc.Request
		req.RawPayload = gc.RawPayload
	}

	response, traceRecord, err := r.executor.Execute(ctx, req)
	if err != nil {
		result.Failures = append(result.Failures, fmt.Sprintf("execution error: %v", err))
		return result
	}
	if traceRecord != nil {
		result.TraceID = traceRecord.TraceID
	}

	result.ActualAnswer, _ = response["answer"].(string)
	result.ActualIntent, _ = response["intent"].(string)
	if citations, ok := response["citations"].([]any); ok {
		result.ActualCitations = citations
	}
	if actions, ok := response["actions"].([]any); ok {
		result.ActualActions = actions
	}

	// Check intent match
	if gc.Expected.Intent != "" && result.ActualIntent != gc.Expected.Intent {
		result.Failures = append(result.Failures,
			fmt.Sprintf("intent mismatch: expected %q, got %q", gc.Expected.Intent, result.ActualIntent))
	}

	// Run metrics
	allPassed := true
	metricsRun := 0
	for _, name := range r.metricNames() {
		fn := r.registry.Get(name)
		if fn == nil {
			continue
		}
		value, passed := fn(gc, result)
		result.Metrics[name] = value
		metricsRun++
		if !passed {
			allPassed = false
		}
	}

	// Case passes if intent matches, at least one metric was run, and all metrics pass
	if len(result.Failures) == 0 && metricsRun > 0 {
		result.Passed = allPassed
	}

	return result
}

func (r *Runner) metricNames() []string {
	if len(r.manifest.Evaluation.Metrics) > 0 {
		return r.manifest.Evaluation.Metrics
	}
	return []string{"citation_coverage", "answer_accuracy"}
}

func loadGoldenCases(path string) ([]GoldenCase, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open golden cases file: %w", err)
	}
	defer file.Close()

	var cases []GoldenCase
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var gc GoldenCase
		if err := json.Unmarshal([]byte(line), &gc); err != nil {
			return nil, fmt.Errorf("line %d: invalid golden case JSON: %w", lineNo, err)
		}
		if gc.ID == "" {
			return nil, fmt.Errorf("line %d: golden case id is required", lineNo)
		}
		if gc.Trigger.Type == "" {
			return nil, fmt.Errorf("line %d: trigger.type is required", lineNo)
		}
		if gc.Expected.Intent == "" {
			return nil, fmt.Errorf("line %d: expected.intent is required", lineNo)
		}
		cases = append(cases, gc)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read golden cases: %w", err)
	}
	return cases, nil
}

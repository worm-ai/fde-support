package evaluation

import (
	"fde-support/internal/manifest"
	"fde-support/internal/runtimecore"
)

// GoldenCase is a single evaluation case.
type GoldenCase struct {
	ID         string                  `json:"id"`
	Trigger    runtimecore.TriggerSpec `json:"trigger"`
	Request    map[string]any          `json:"request"`
	RawPayload map[string]any          `json:"raw_payload,omitempty"`
	Expected   GoldenCaseExpected      `json:"expected"`
}

// GoldenCaseExpected defines the expected outputs for a golden case.
type GoldenCaseExpected struct {
	Intent         string   `json:"intent"`
	MustCite       bool     `json:"mustCite,omitempty"`
	AnswerContains []string `json:"answerContains,omitempty"`
}

// EvalResult holds the result of evaluating a single golden case.
type EvalResult struct {
	CaseID          string             `json:"caseId"`
	Passed          bool               `json:"passed"`
	Metrics         map[string]float64 `json:"metrics"`
	TraceID         string             `json:"traceId,omitempty"`
	ActualAnswer    string             `json:"actualAnswer,omitempty"`
	ActualIntent    string             `json:"actualIntent,omitempty"`
	ActualCitations []any              `json:"actualCitations,omitempty"`
	Failures        []string           `json:"failures,omitempty"`
}

// EvalReport is the full evaluation report.
type EvalReport struct {
	Solution    string             `json:"solution"`
	Version     string             `json:"version"`
	DatasetURI  string             `json:"datasetUri"`
	TotalCases  int                `json:"totalCases"`
	PassedCases int                `json:"passedCases"`
	Metrics     map[string]float64 `json:"metrics"`
	GateResults []GateResult       `json:"gateResults"`
	Results     []EvalResult       `json:"results"`
	Warnings    []string           `json:"warnings,omitempty"`
}

// GateResult represents the outcome of a single evaluation gate.
type GateResult struct {
	Metric   string  `json:"metric"`
	Min      float64 `json:"min"`
	Actual   float64 `json:"actual"`
	Severity string  `json:"severity"`
	Schedule string  `json:"schedule"`
	Passed   bool    `json:"passed"`
}

// MetricFunc computes a metric value for a single evaluation result.
type MetricFunc func(case_ GoldenCase, result EvalResult) (float64, bool)

// MetricRegistry maps metric names to their implementations.
type MetricRegistry struct {
	metrics map[string]MetricFunc
}

// NewMetricRegistry creates a metric registry with built-in metrics.
func NewMetricRegistry() *MetricRegistry {
	return &MetricRegistry{
		metrics: map[string]MetricFunc{
			"citation_coverage": evalCitationCoverage,
			"answer_accuracy":   evalAnswerAccuracy,
		},
	}
}

// Register adds or overrides a metric function.
func (r *MetricRegistry) Register(name string, fn MetricFunc) {
	r.metrics[name] = fn
}

// Get returns the metric function for a given name, or nil if not found.
func (r *MetricRegistry) Get(name string) MetricFunc {
	return r.metrics[name]
}

// Names returns all registered metric names.
func (r *MetricRegistry) Names() []string {
	names := make([]string, 0, len(r.metrics))
	for name := range r.metrics {
		names = append(names, name)
	}
	return names
}

// BuildManifest creates a minimal manifest from evaluation metadata for gate execution.
func BuildEvalManifest(name, version string, gates []manifest.EvaluationGateSpec) *manifest.SolutionManifest {
	return &manifest.SolutionManifest{
		Metadata: manifest.MetadataSpec{
			Name:    name,
			Version: version,
		},
		Evaluation: manifest.EvaluationSpec{
			Gates: gates,
		},
	}
}

package release

import (
	"context"
	"testing"

	"fde-support/internal/environment"
	"fde-support/internal/evaluation"
	"fde-support/internal/manifest"
)

type stubEvalRunner struct {
	report *evaluation.EvalReport
	err    error
	called bool
}

func (s *stubEvalRunner) Run(ctx context.Context, datasetURI string, gates []manifest.EvaluationGateSpec) (*evaluation.EvalReport, error) {
	s.called = true
	return s.report, s.err
}

func TestCheckEvalGatesFailsOnOnReleaseBlockFailure(t *testing.T) {
	m := &manifest.SolutionManifest{
		Metadata: manifest.MetadataSpec{Name: "s", Version: "1"},
		Evaluation: manifest.EvaluationSpec{
			Datasets: []manifest.EvaluationDatasetSpec{{ID: "golden", URI: "golden.jsonl"}},
			Gates: []manifest.EvaluationGateSpec{{
				Metric: "citation_coverage", Min: 0.95, Severity: "block", Schedule: "onRelease",
			}},
		},
	}
	runner := &stubEvalRunner{report: &evaluation.EvalReport{
		GateResults: []evaluation.GateResult{{
			Metric: "citation_coverage", Min: 0.95, Actual: 0.5, Severity: "block", Schedule: "onRelease", Passed: false,
		}},
	}}
	checker := NewCheckerWithEvaluator(m, environment.ResolvedEnvironment{EnvironmentName: "production"}, runner)
	result := checker.checkEvalGates(context.Background())
	if result.Passed {
		t.Fatalf("expected eval_gates_passed to fail")
	}
	if !runner.called {
		t.Fatalf("expected evaluation runner to be called")
	}
}

package evaluation

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"fde-support/internal/manifest"
	"fde-support/internal/runtimecore"
	"fde-support/internal/trace"
)

func TestRunnerCapturesActualCitationsFromResponse(t *testing.T) {
	runner := NewRunner(
		&stubEvalExecutor{response: map[string]any{
			"answer":    "A grounded answer",
			"intent":    "support",
			"citations": []any{map[string]any{"source": "manual", "ref": "1"}},
		}},
		&manifest.SolutionManifest{Metadata: manifest.MetadataSpec{Name: "s", Version: "1"}},
		NewMetricRegistry(),
	)

	result := runner.runCase(context.Background(), GoldenCase{
		ID:      "case-1",
		Trigger: runtimecore.TriggerSpec{Type: "chat"},
		Expected: GoldenCaseExpected{
			Intent:   "support",
			MustCite: true,
		},
	})

	if len(result.ActualCitations) != 1 {
		t.Fatalf("ActualCitations len = %d, want 1", len(result.ActualCitations))
	}
	if !result.Passed {
		t.Fatalf("expected case to pass, failures=%v metrics=%v", result.Failures, result.Metrics)
	}
}

func TestRunnerCreatesFailedGateResultWhenBlockMetricMissing(t *testing.T) {
	dataset := writeGoldenCases(t, []string{`{"id":"c1","trigger":{"type":"chat"},"request":{"message":"hi"},"expected":{"intent":"support"}}`})
	m := &manifest.SolutionManifest{
		Metadata: manifest.MetadataSpec{Name: "s", Version: "1"},
		Evaluation: manifest.EvaluationSpec{
			Metrics: []string{"custom_quality_metric"},
			Gates: []manifest.EvaluationGateSpec{{
				Metric: "custom_quality_metric", Min: 0.9, Severity: "block", Schedule: "onRelease",
			}},
		},
	}
	runner := NewRunner(
		&stubEvalExecutor{response: map[string]any{"intent": "support", "answer": "ok"}},
		m,
		NewMetricRegistry(),
	)

	report, err := runner.Run(context.Background(), dataset, m.Evaluation.Gates)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(report.GateResults) != 1 {
		t.Fatalf("GateResults len = %d, want 1", len(report.GateResults))
	}
	if report.GateResults[0].Passed {
		t.Fatalf("missing block metric must fail gate")
	}
}

type stubEvalExecutor struct {
	response map[string]any
}

func (e *stubEvalExecutor) Execute(ctx context.Context, req runtimecore.RuntimeRequest) (map[string]any, *trace.TraceRecord, error) {
	return e.response, &trace.TraceRecord{TraceID: "trace-1"}, nil
}

func writeGoldenCases(t *testing.T, lines []string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "golden.jsonl")
	content := ""
	for _, line := range lines {
		content += line + "\n"
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write golden cases: %v", err)
	}
	return path
}

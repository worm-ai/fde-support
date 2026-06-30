package evaluation

import (
	"context"
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

type stubEvalExecutor struct {
	response map[string]any
}

func (e *stubEvalExecutor) Execute(ctx context.Context, req runtimecore.RuntimeRequest) (map[string]any, *trace.TraceRecord, error) {
	return e.response, &trace.TraceRecord{TraceID: "trace-1"}, nil
}

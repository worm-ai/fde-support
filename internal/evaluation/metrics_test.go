package evaluation

import "testing"

func TestCitationCoverageRequiresActualCitations(t *testing.T) {
	gc := GoldenCase{Expected: GoldenCaseExpected{MustCite: true}}
	result := EvalResult{ActualAnswer: "A non-empty answer", ActualCitations: nil}
	value, passed := evalCitationCoverage(gc, result)
	if passed || value != 0 {
		t.Fatalf("expected citation coverage to fail without citations, got value=%v passed=%v", value, passed)
	}
}

func TestCitationCoveragePassesWithActualCitations(t *testing.T) {
	gc := GoldenCase{Expected: GoldenCaseExpected{MustCite: true}}
	result := EvalResult{
		ActualAnswer:    "A grounded answer",
		ActualCitations: []any{map[string]any{"source": "manual", "ref": "1"}},
	}
	value, passed := evalCitationCoverage(gc, result)
	if !passed || value != 1 {
		t.Fatalf("expected citation coverage to pass, got value=%v passed=%v", value, passed)
	}
}

func TestResultAccuracyPassesWhenExpectedFragmentsAppear(t *testing.T) {
	gc := GoldenCase{Expected: GoldenCaseExpected{AnswerContains: []string{"stock", "42"}}}
	result := EvalResult{ActualAnswer: "Current stock is 42 units."}
	value, applicable := evalResultAccuracy(gc, result)
	if !applicable || value != 1 {
		t.Fatalf("expected result accuracy to pass, got value=%v applicable=%v", value, applicable)
	}
}

func TestResultAccuracyFailsWhenExpectedFragmentMissing(t *testing.T) {
	gc := GoldenCase{Expected: GoldenCaseExpected{AnswerContains: []string{"stock", "42"}}}
	result := EvalResult{ActualAnswer: "Current stock is unknown."}
	value, applicable := evalResultAccuracy(gc, result)
	if !applicable || value != 0 {
		t.Fatalf("expected result accuracy to fail, got value=%v applicable=%v", value, applicable)
	}
}

func TestEscalationPrecisionPassesForExpectedHandoffIntent(t *testing.T) {
	gc := GoldenCase{Expected: GoldenCaseExpected{Intent: "human_handoff"}}
	result := EvalResult{ActualIntent: "human_handoff"}
	value, applicable := evalEscalationPrecision(gc, result)
	if !applicable || value != 1 {
		t.Fatalf("expected escalation precision to pass, got value=%v applicable=%v", value, applicable)
	}
}

func TestEscalationPrecisionPassesForHandoffAction(t *testing.T) {
	gc := GoldenCase{Expected: GoldenCaseExpected{Intent: "complaint"}}
	result := EvalResult{
		ActualIntent: "complaint",
		ActualActions: []any{
			map[string]any{"node": "human_handoff", "output": map[string]any{"status": "queued"}},
		},
	}
	value, applicable := evalEscalationPrecision(gc, result)
	if !applicable || value != 1 {
		t.Fatalf("expected escalation precision to pass, got value=%v applicable=%v", value, applicable)
	}
}

func TestEscalationPrecisionFailsWhenExpectedHandoffDoesNotHandoff(t *testing.T) {
	gc := GoldenCase{Expected: GoldenCaseExpected{Intent: "complaint"}}
	result := EvalResult{ActualIntent: "troubleshooting"}
	value, applicable := evalEscalationPrecision(gc, result)
	if !applicable || value != 0 {
		t.Fatalf("expected escalation precision to fail, got value=%v applicable=%v", value, applicable)
	}
}

func TestMetricRegistryIncludesM2Metrics(t *testing.T) {
	registry := NewMetricRegistry()
	for _, name := range []string{"result_accuracy", "escalation_precision"} {
		if registry.Get(name) == nil {
			t.Fatalf("metric %q is not registered", name)
		}
	}
}

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

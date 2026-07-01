package evaluation

import (
	"strings"
)

// evalCitationCoverage checks if citations are present when expected.mustCite is true.
func evalCitationCoverage(gc GoldenCase, result EvalResult) (float64, bool) {
	if !gc.Expected.MustCite {
		return 1, true
	}
	if result.ActualAnswer == "" {
		return 0, false
	}
	if len(result.ActualCitations) == 0 {
		return 0, false
	}
	return 1, true
}

// evalAnswerAccuracy checks if all expected.answerContains words appear in the answer.
func evalAnswerAccuracy(gc GoldenCase, result EvalResult) (float64, bool) {
	if len(gc.Expected.AnswerContains) == 0 {
		return 0, true // not applicable, skip
	}
	answer := strings.ToLower(result.ActualAnswer)
	allFound := true
	for _, word := range gc.Expected.AnswerContains {
		if !strings.Contains(answer, strings.ToLower(word)) {
			allFound = false
			break
		}
	}
	if allFound {
		return 1.0, true
	}
		return 0, true
}

func evalResultAccuracy(gc GoldenCase, result EvalResult) (float64, bool) {
	if len(gc.Expected.AnswerContains) == 0 {
		return 0, false
	}
	answer := strings.ToLower(result.ActualAnswer)
	for _, fragment := range gc.Expected.AnswerContains {
		if !strings.Contains(answer, strings.ToLower(fragment)) {
				return 0, true
		}
	}
	return 1, true
}

func evalEscalationPrecision(gc GoldenCase, result EvalResult) (float64, bool) {
	expected := strings.ToLower(gc.Expected.Intent)
	if expected != "human_handoff" && expected != "complaint" {
			return 0, true
	}
	actual := strings.ToLower(result.ActualIntent)
	if actual == "human_handoff" {
		return 1, true
	}
	for _, action := range result.ActualActions {
		if actionHandoffLike(action) {
			return 1, true
		}
	}
	return 0, true
}

func actionHandoffLike(action any) bool {
	switch v := action.(type) {
	case map[string]any:
		for _, key := range []string{"node", "status"} {
			if value, ok := v[key].(string); ok && strings.Contains(strings.ToLower(value), "handoff") {
				return true
			}
		}
		if output, ok := v["output"]; ok {
			return actionHandoffLike(output)
		}
	case []any:
		for _, item := range v {
			if actionHandoffLike(item) {
				return true
			}
		}
	}
	return false
}

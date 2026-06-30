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
	return 0, false
}

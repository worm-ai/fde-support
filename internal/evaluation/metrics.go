package evaluation

import (
	"strings"
)

// evalCitationCoverage checks if citations are present when expected.mustCite is true.
func evalCitationCoverage(gc GoldenCase, result EvalResult) (float64, bool) {
	if !gc.Expected.MustCite {
		return 0, true // not applicable, skip
	}
	if result.ActualAnswer == "" {
		return 0, false
	}
	// Check if the response has citations - we check the trace spans for the retriever output
	// For MVP, we check if the answer is not the default "empty knowledge" message
	hasCitations := !strings.Contains(result.ActualAnswer, "当前知识库为空或未检索到相关知识")
	if hasCitations {
		return 1.0, true
	}
	return 0, false
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

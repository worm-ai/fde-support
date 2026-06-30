package trace

import (
	"regexp"
	"strings"
)

var phoneLikePattern = regexp.MustCompile(`\b1[3-9]\d{9}\b`)
var emailLikePattern = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)
var inlineSensitivePattern = regexp.MustCompile(`(?i)\b((?:api[_ -]?key)|token|password|secret|authorization|cookie)\b(\s*(?:is|=|:)\s*)([^\s,;]+)`)

func RedactValue(value any) any {
	switch v := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, item := range v {
			if shouldOmitTraceKey(key) {
				continue
			}
			if isSensitiveTraceKey(key) {
				out[key] = "[REDACTED]"
				continue
			}
			out[key] = RedactValue(item)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = RedactValue(item)
		}
		return out
	case string:
		s := phoneLikePattern.ReplaceAllString(v, "[REDACTED_PHONE]")
		s = emailLikePattern.ReplaceAllString(s, "[REDACTED_EMAIL]")
		s = inlineSensitivePattern.ReplaceAllString(s, "${1}${2}[REDACTED]")
		if strings.HasPrefix(s, "env:") {
			return s
		}
		if len(s) > 2048 {
			return s[:2048] + "...[TRUNCATED]"
		}
		return s
	default:
		return v
	}
}

func RedactMap(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	return RedactValue(in).(map[string]any)
}

func shouldOmitTraceKey(key string) bool {
	return strings.EqualFold(key, "raw_payload")
}

func isSensitiveTraceKey(key string) bool {
	k := strings.ToLower(key)
	return strings.Contains(k, "password") ||
		strings.Contains(k, "token") ||
		strings.Contains(k, "secret") ||
		strings.Contains(k, "apikey") ||
		strings.Contains(k, "api_key") ||
		strings.Contains(k, "authorization") ||
		strings.Contains(k, "cookie")
}

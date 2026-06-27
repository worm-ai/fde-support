package shared

import (
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
)

func IsEnvSecretRef(value string) bool {
	if !strings.HasPrefix(value, "env:") {
		return false
	}
	name := strings.TrimPrefix(value, "env:")
	if name == "" {
		return false
	}
	for _, r := range name {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			continue
		}
		return false
	}
	return true
}

func IsSensitiveRefKey(key string) bool {
	k := strings.ToLower(key)
	if !strings.HasSuffix(k, "ref") {
		return false
	}
	return strings.Contains(k, "token") ||
		strings.Contains(k, "secret") ||
		strings.Contains(k, "apikey") ||
		strings.Contains(k, "api_key") ||
		strings.Contains(k, "key")
}

func MatchesPrimitive(value any, typ string) bool {
	typ = strings.TrimSuffix(typ, "?")
	if value == nil {
		return false
	}
	switch typ {
	case "string":
		_, ok := value.(string)
		return ok
	case "number":
		_, ok := ToFloat64(value)
		return ok
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "array":
		if _, ok := value.([]any); ok {
			return true
		}
		rv := reflect.ValueOf(value)
		return rv.IsValid() && rv.Kind() == reflect.Slice
	case "object":
		if _, ok := value.(map[string]any); ok {
			return true
		}
		rv := reflect.ValueOf(value)
		return rv.IsValid() && rv.Kind() == reflect.Map
	default:
		return false
	}
}

func ToFloat64(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return 0, false
		}
		return v, true
	case float32:
		f := float64(v)
		if math.IsNaN(f) || math.IsInf(f, 0) {
			return 0, false
		}
		return f, true
	case int:
		return float64(v), true
	case int8:
		return float64(v), true
	case int16:
		return float64(v), true
	case int32:
		return float64(v), true
	case int64:
		return float64(v), true
	case uint:
		return float64(v), true
	case uint8:
		return float64(v), true
	case uint16:
		return float64(v), true
	case uint32:
		return float64(v), true
	case uint64:
		return float64(v), true
	case string:
		f, err := strconv.ParseFloat(v, 64)
		return f, err == nil
	default:
		return 0, false
	}
}

func ValidatePrimitiveMap(schema map[string]string, values map[string]any, path string) *AppError {
	for key, typ := range schema {
		optional := strings.HasSuffix(typ, "?")
		value, ok := values[key]
		if !ok {
			if optional {
				continue
			}
			return BadRequest("INPUT_REQUIRED_FIELD", path+"."+key, fmt.Sprintf("required input %q is missing", key))
		}
		if !MatchesPrimitive(value, typ) {
			return BadRequest("INPUT_TYPE_MISMATCH", path+"."+key, fmt.Sprintf("expected %s for %q", strings.TrimSuffix(typ, "?"), key))
		}
	}
	return nil
}

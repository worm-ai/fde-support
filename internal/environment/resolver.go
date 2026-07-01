package environment

import (
	"fmt"
	"os"
	"path/filepath"

	"fde-support/internal/manifest"
)

type ResolvedEnvironment struct {
	EnvironmentName  string
	EnvironmentType  string
	ModelKeyRef      string
	DefaultModel     string
	FallbackModel    string
	MaxLatencyMs     int
	MaxCostPerRunUsd float64
	TracePath        string
	RetainDays       int
}

func Resolve(m *manifest.SolutionManifest, envName string) (ResolvedEnvironment, error) {
	var selected *manifest.EnvironmentSpec
	for i := range m.Delivery.Environments {
		if m.Delivery.Environments[i].Name == envName {
			selected = &m.Delivery.Environments[i]
			break
		}
	}
	if selected == nil {
		return ResolvedEnvironment{}, fmt.Errorf("environment %q not found", envName)
	}
	cfg := selected.Config
	out := ResolvedEnvironment{
		EnvironmentName:  selected.Name,
		EnvironmentType:  selected.Type,
		DefaultModel:     m.Runtime.ModelPolicy.DefaultModel,
		FallbackModel:    m.Runtime.ModelPolicy.FallbackModel,
		MaxLatencyMs:     m.Runtime.ModelPolicy.MaxLatencyMs,
		MaxCostPerRunUsd: m.Runtime.ModelPolicy.MaxCostPerRunUsd,
		RetainDays:       m.Runtime.Observability.RetainDays,
	}
	if out.MaxLatencyMs == 0 {
		out.MaxLatencyMs = 8000
	}
	if out.RetainDays == 0 {
		out.RetainDays = 7
	}
	if v, ok := cfg["modelKeyRef"].(string); ok {
		out.ModelKeyRef = v
	}
	if v, ok := cfg["defaultModel"].(string); ok {
		out.DefaultModel = v
	}
	if v, ok := cfg["fallbackModel"].(string); ok {
		out.FallbackModel = v
	}
	if v, ok := numberAsInt(cfg["maxLatencyMs"]); ok {
		out.MaxLatencyMs = v
	}
	if v, ok := numberAsFloat(cfg["maxCostPerRunUsd"]); ok {
		out.MaxCostPerRunUsd = v
	}
	if v, ok := cfg["tracePath"].(string); ok {
		out.TracePath = resolvePath(m.BaseDir, v)
	}
	if v, ok := numberAsInt(cfg["retainDays"]); ok {
		out.RetainDays = v
	}
	if out.TracePath == "" {
		out.TracePath = filepath.Join(m.BaseDir, ".solution", "traces")
	}
	return out, nil
}

func (e ResolvedEnvironment) ResolveSecret(ref string) (string, bool) {
	const prefix = "env:"
	if len(ref) <= len(prefix) || ref[:len(prefix)] != prefix {
		return "", false
	}
	value, ok := os.LookupEnv(ref[len(prefix):])
	return value, ok && value != ""
}

func (e ResolvedEnvironment) ReportPath() string {
	dir := filepath.Dir(e.TracePath)
	if dir == "." || dir == "" {
		dir = "."
	}
	return filepath.Join(dir, "reports", "knowledge-quality.json")
}

func resolvePath(baseDir, value string) string {
	if filepath.IsAbs(value) {
		return filepath.Clean(value)
	}
	return filepath.Clean(filepath.Join(baseDir, value))
}

func numberAsInt(value any) (int, bool) {
	switch v := value.(type) {
	case int:
		return v, true
	case int64:
		return int(v), true
	case float64:
		// Warn on truncation for values that lose precision as int
		if float64(int(v)) != v {
			fmt.Fprintf(os.Stderr, "WARNING: numberAsInt truncating %v to %d\n", v, int(v))
		}
		return int(v), true
	default:
		return 0, false
	}
}

func numberAsFloat(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	default:
		return 0, false
	}
}

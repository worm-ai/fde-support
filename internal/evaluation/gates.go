package evaluation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"fde-support/internal/manifest"
)

// GateChecker evaluates manifest gates against metric results.
type GateChecker struct {
	manifest *manifest.SolutionManifest
}

// NewGateChecker creates a gate checker.
func NewGateChecker(m *manifest.SolutionManifest) *GateChecker {
	return &GateChecker{manifest: m}
}

// Check evaluates all configured gates against the given metrics.
// Returns the gate results and whether any block gate failed.
func (c *GateChecker) Check(metrics map[string]float64, schedule string) ([]GateResult, bool, []string) {
	var results []GateResult
	blockFailed := false
	var warnings []string

	for _, gate := range c.manifest.Evaluation.Gates {
		if gate.Schedule != "" && gate.Schedule != schedule {
			continue
		}
		actual, ok := metrics[gate.Metric]
		if !ok {
			warnings = append(warnings, fmt.Sprintf("gate metric %s not found in evaluation results", gate.Metric))
			continue
		}
		passed := actual >= gate.Min
		result := GateResult{
			Metric:   gate.Metric,
			Min:      gate.Min,
			Actual:   actual,
			Severity: gate.Severity,
			Schedule: gate.Schedule,
			Passed:   passed,
		}
		results = append(results, result)

		if !passed {
			if gate.Severity == "block" {
				blockFailed = true
			}
			warnings = append(warnings,
				fmt.Sprintf("gate %s failed: %.4f < %.4f (severity: %s, schedule: %s)",
					gate.Metric, actual, gate.Min, gate.Severity, gate.Schedule))
		}
	}

	return results, blockFailed, warnings
}

// ComputeFingerprint generates a fingerprint for the manifest + dataset combination.
// This is used to determine if evaluation results can be cached.
func ComputeFingerprint(m *manifest.SolutionManifest, datasetURI string) string {
	hash := sha256.New()
	// Include manifest metadata
	hash.Write([]byte(m.Metadata.Name))
	hash.Write([]byte(m.Metadata.Version))
	// Include model policy
	if bytes, err := json.Marshal(m.Runtime.ModelPolicy); err == nil {
		hash.Write(bytes)
	} else {
		hash.Write([]byte("model_policy_marshal_error"))
	}
	// Include knowledge sources for data drift detection
	for _, src := range m.Knowledge.Sources {
		hash.Write([]byte(src.ID))
		hash.Write([]byte(src.URI))
	}
	// Include component configs
	for _, comp := range m.Components {
		hash.Write([]byte(comp.Ref))
		if bytes, err := json.Marshal(comp.Config); err == nil {
			hash.Write(bytes)
		} else {
			hash.Write([]byte("component_config_marshal_error"))
		}
	}
	// Include dataset URI
	hash.Write([]byte(datasetURI))
	return hex.EncodeToString(hash.Sum(nil))
}

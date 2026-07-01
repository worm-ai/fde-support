package release

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"fde-support/internal/environment"
	"fde-support/internal/evaluation"
	"fde-support/internal/knowledge"
	"fde-support/internal/manifest"
	"fde-support/internal/shared"
)

// CheckResult is the result of a single release check.
type CheckResult struct {
	Name     string `json:"name"`
	Passed   bool   `json:"passed"`
	Severity string `json:"severity"`
	Message  string `json:"message,omitempty"`
}

// ReleaseReport is the full release check report.
type ReleaseReport struct {
	Solution string        `json:"solution"`
	Version  string        `json:"version"`
	Env      string        `json:"environment"`
	Passed   bool          `json:"passed"`
	Checks   []CheckResult `json:"checks"`
}

type EvaluationRunner interface {
	Run(ctx context.Context, datasetURI string, gates []manifest.EvaluationGateSpec) (*evaluation.EvalReport, error)
}

// Checker runs all release checks against a manifest and environment.
type Checker struct {
	manifest      *manifest.SolutionManifest
	env           environment.ResolvedEnvironment
	evaluator     EvaluationRunner
	evalCache     *evaluation.EvalReport
	evalCacheFp   string
}

var supportedReleaseChecks = map[string]bool{
	"model_credentials_configured":  true,
	"sensor_credentials_configured": true,
	"action_credentials_configured": true,
	"signal_ingress_reachable":      true,
	"knowledge_quality_passed":      true,
	"eval_gates_passed":             true,
	"observability_enabled":         true,
	"security_baseline_passed":      true,
}

var mandatoryReleaseChecks = map[string]bool{
	"knowledge_quality_passed":      true,
	"eval_gates_passed":             true,
}

// NewChecker creates a release checker.
func NewChecker(m *manifest.SolutionManifest, env environment.ResolvedEnvironment) *Checker {
	return &Checker{manifest: m, env: env}
}

func NewCheckerWithEvaluator(m *manifest.SolutionManifest, env environment.ResolvedEnvironment, evaluator EvaluationRunner) *Checker {
	return &Checker{manifest: m, env: env, evaluator: evaluator}
}

// Run executes all configured release checks in order.
func (c *Checker) Run(ctx context.Context) (*ReleaseReport, error) {
	report := &ReleaseReport{
		Solution: c.manifest.Metadata.Name,
		Version:  c.manifest.Metadata.Version,
		Env:      c.env.EnvironmentName,
		Passed:   true,
	}

	checks := []struct {
		name string
		fn   func(context.Context) CheckResult
	}{
		{"model_credentials_configured", c.checkModelCredentials},
		{"sensor_credentials_configured", c.checkSensorCredentials},
		{"action_credentials_configured", c.checkActionCredentials},
		{"signal_ingress_reachable", c.checkSignalIngress},
		{"knowledge_quality_passed", c.checkKnowledgeQuality},
		{"eval_gates_passed", c.checkEvalGates},
		{"observability_enabled", c.checkObservability},
		{"security_baseline_passed", c.checkSecurityBaseline},
	}
	declared := map[string]bool{}
	for _, name := range c.manifest.Delivery.ReleaseChecks {
		if supportedReleaseChecks[name] {
			declared[name] = true
		}
	}

	for _, check := range checks {
		if len(declared) > 0 && !declared[check.name] && !mandatoryReleaseChecks[check.name] {
			continue
		}
		if err := ctx.Err(); err != nil {
			return report, err
		}
		result := check.fn(ctx)
		report.Checks = append(report.Checks, result)
		if !result.Passed && result.Severity == "block" {
			report.Passed = false
		}
	}

	return report, nil
}

func (c *Checker) checkModelCredentials(ctx context.Context) CheckResult {
	if c.env.DefaultModel == "" && c.manifest.Runtime.ModelPolicy.DefaultModel == "" {
		return CheckResult{Name: "model_credentials_configured", Passed: false, Severity: "block", Message: "runtime.modelPolicy.defaultModel is required"}
	}
	keyRef := c.env.ModelKeyRef
	if keyRef == "" {
		keyRef = "env:OPENAI_API_KEY"
	}
	if val, ok := c.env.ResolveSecret(keyRef); !ok || val == "" {
		return CheckResult{Name: "model_credentials_configured", Passed: false, Severity: "block", Message: "model key not configured"}
	}
	return CheckResult{Name: "model_credentials_configured", Passed: true, Severity: "block"}
}

func (c *Checker) checkSensorCredentials(ctx context.Context) CheckResult {
	for _, sensor := range c.manifest.Perception.Sensors {
		if tokenRef, ok := sensor.Config["authTokenRef"].(string); ok {
			if val, ok := c.env.ResolveSecret(tokenRef); !ok || val == "" {
				return CheckResult{Name: "sensor_credentials_configured", Passed: false, Severity: "block",
					Message: fmt.Sprintf("sensor %s: auth token not configured", sensor.ID)}
			}
		}
	}
	return CheckResult{Name: "sensor_credentials_configured", Passed: true, Severity: "block"}
}

func (c *Checker) checkActionCredentials(ctx context.Context) CheckResult {
	for _, comp := range c.manifest.Components {
		if comp.Category != "action" {
			continue
		}
		for key, val := range comp.Config {
			if shared.IsSensitiveRefKey(key) {
				if s, ok := val.(string); ok {
					if resolved, ok := c.env.ResolveSecret(s); !ok || resolved == "" {
						return CheckResult{Name: "action_credentials_configured", Passed: false, Severity: "block",
							Message: fmt.Sprintf("action %s: %s not configured", comp.ID, key)}
					}
				}
			}
		}
	}
	return CheckResult{Name: "action_credentials_configured", Passed: true, Severity: "block"}
}

func (c *Checker) checkSignalIngress(ctx context.Context) CheckResult {
	// For non-Docker Compose targets, only check endpoint registration
	for _, sensor := range c.manifest.Perception.Sensors {
		if _, ok := sensor.Config["endpointPath"]; !ok {
			return CheckResult{Name: "signal_ingress_reachable", Passed: false, Severity: "block",
				Message: fmt.Sprintf("sensor %s: endpointPath not configured", sensor.ID)}
		}
	}
	return CheckResult{Name: "signal_ingress_reachable", Passed: true, Severity: "block"}
}

func (c *Checker) checkKnowledgeQuality(ctx context.Context) CheckResult {
	reportPath := c.env.ReportPath()
	data, err := os.ReadFile(reportPath)
	if err != nil {
		if os.IsNotExist(err) {
			return CheckResult{Name: "knowledge_quality_passed", Passed: false, Severity: "block",
				Message: "knowledge quality report not found — run 'solution ingest' first"}
		}
		return CheckResult{Name: "knowledge_quality_passed", Passed: false, Severity: "block",
			Message: fmt.Sprintf("knowledge quality report not found: %v", err)}
	}
	if len(data) == 0 || strings.TrimSpace(string(data)) == "" {
		return CheckResult{Name: "knowledge_quality_passed", Passed: false, Severity: "block",
			Message: "knowledge quality report is empty — run 'solution ingest' first"}
	}
	var report struct {
		GeneratedAt                 time.Time                `json:"generatedAt"`
		ManifestFingerprint         string                   `json:"manifestFingerprint"`
		KnowledgeConfigFingerprint  string                   `json:"knowledgeConfigFingerprint"`
		KnowledgeSourcesFingerprint string                   `json:"knowledgeSourcesFingerprint"`
		Sources                     []knowledge.SourceReport `json:"sources"`
		Status                      string                   `json:"status"`
		Items                       []struct {
			Severity string `json:"severity"`
		} `json:"items"`
	}
	if err := json.Unmarshal(data, &report); err != nil {
		return CheckResult{Name: "knowledge_quality_passed", Passed: false, Severity: "block",
			Message: fmt.Sprintf("invalid quality report: %v", err)}
	}
	if report.GeneratedAt.IsZero() {
		return CheckResult{Name: "knowledge_quality_passed", Passed: false, Severity: "block", Message: "knowledge quality report is missing a generatedAt timestamp"}
	}
	if time.Since(report.GeneratedAt) > 24*time.Hour {
		return CheckResult{Name: "knowledge_quality_passed", Passed: false, Severity: "block", Message: "knowledge quality report is older than 24 hours"}
	}
	if report.ManifestFingerprint != knowledge.FingerprintManifest(c.manifest) {
		return CheckResult{Name: "knowledge_quality_passed", Passed: false, Severity: "block", Message: "knowledge quality report manifest fingerprint mismatch"}
	}
	if report.KnowledgeConfigFingerprint != knowledge.FingerprintKnowledgeConfig(c.manifest) {
		return CheckResult{Name: "knowledge_quality_passed", Passed: false, Severity: "block", Message: "knowledge quality report config fingerprint mismatch"}
	}
	if report.KnowledgeSourcesFingerprint == "" || report.KnowledgeSourcesFingerprint != knowledge.FingerprintSourceReports(report.Sources) {
		return CheckResult{Name: "knowledge_quality_passed", Passed: false, Severity: "block", Message: "knowledge quality report sources fingerprint mismatch"}
	}
	for _, item := range report.Items {
		if item.Severity == "block" {
			return CheckResult{Name: "knowledge_quality_passed", Passed: false, Severity: "block", Message: "knowledge quality report has block findings"}
		}
	}
	if report.Status == "blocked" {
		return CheckResult{Name: "knowledge_quality_passed", Passed: false, Severity: "block",
			Message: "knowledge quality report has block findings"}
	}
	return CheckResult{Name: "knowledge_quality_passed", Passed: true, Severity: "block"}
}

func (c *Checker) checkEvalGates(ctx context.Context) CheckResult {
	hasOnReleaseBlock := false
	for _, gate := range c.manifest.Evaluation.Gates {
		if gate.Schedule == "onRelease" && gate.Severity == "block" {
			hasOnReleaseBlock = true
			break
		}
	}
	if !hasOnReleaseBlock {
		return CheckResult{Name: "eval_gates_passed", Passed: true, Severity: "block"}
	}
	if c.evaluator == nil {
		return CheckResult{Name: "eval_gates_passed", Passed: false, Severity: "block", Message: "evaluation runner is not configured"}
	}
	ranDataset := false
	// Check eval cache: compute fingerprint of manifest and dataset URIs
	cacheKey := evaluation.ComputeFingerprint(c.manifest, "")
	if cacheKey == c.evalCacheFp && c.evalCache != nil {
		// Use cached result - verify gates are still satisfied
		for _, gate := range c.evalCache.GateResults {
			if gate.Schedule == "onRelease" && gate.Severity == "block" && !gate.Passed {
				return CheckResult{Name: "eval_gates_passed", Passed: false, Severity: "block", Message: "onRelease evaluation gate failed (cached): " + gate.Metric}
			}
		}
		return CheckResult{Name: "eval_gates_passed", Passed: true, Severity: "block"}
	}
	for _, dataset := range c.manifest.Evaluation.Datasets {
		if dataset.URI == "" {
			continue
		}
		ranDataset = true
		report, err := c.evaluator.Run(ctx, c.resolveDatasetURI(dataset.URI), c.manifest.Evaluation.Gates)
		if err != nil {
			return CheckResult{Name: "eval_gates_passed", Passed: false, Severity: "block", Message: err.Error()}
		}
		// Cache the result
		c.evalCache = report
		c.evalCacheFp = cacheKey
		for _, gate := range report.GateResults {
			if gate.Schedule == "onRelease" && gate.Severity == "block" && !gate.Passed {
				return CheckResult{Name: "eval_gates_passed", Passed: false, Severity: "block", Message: "onRelease evaluation gate failed: " + gate.Metric}
			}
		}
	}
	if !ranDataset {
		return CheckResult{Name: "eval_gates_passed", Passed: false, Severity: "block", Message: "evaluation dataset is not configured"}
	}
	return CheckResult{Name: "eval_gates_passed", Passed: true, Severity: "block"}
}

func (c *Checker) resolveDatasetURI(uri string) string {
	if filepath.IsAbs(uri) || c.manifest.BaseDir == "" {
		return uri
	}
	return filepath.Join(c.manifest.BaseDir, uri)
}

func (c *Checker) checkObservability(ctx context.Context) CheckResult {
	if c.manifest.Runtime.Observability.Trace == "" {
		return CheckResult{Name: "observability_enabled", Passed: true, Severity: "warn",
			Message: "observability trace is not configured; tracing may be disabled"}
	}
	if c.manifest.Runtime.Observability.Trace != "required" {
		return CheckResult{Name: "observability_enabled", Passed: false, Severity: "block",
			Message: "observability trace is not set to required"}
	}
	return CheckResult{Name: "observability_enabled", Passed: true, Severity: "block"}
}

func (c *Checker) checkSecurityBaseline(ctx context.Context) CheckResult {
	sec := c.manifest.Delivery.Security
	if sec.PIIDetection != "required" {
		return CheckResult{Name: "security_baseline_passed", Passed: false, Severity: "warn",
			Message: "PII detection is not required"}
	}
	if sec.PromptInjectionDefense != "required" {
		return CheckResult{Name: "security_baseline_passed", Passed: false, Severity: "warn",
			Message: "prompt injection defense is not required"}
	}
	return CheckResult{Name: "security_baseline_passed", Passed: true, Severity: "block"}
}

package release

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"fde-support/internal/environment"
	"fde-support/internal/evaluation"
	"fde-support/internal/knowledge"
	"fde-support/internal/manifest"
)

type stubEvalRunner struct {
	report *evaluation.EvalReport
	err    error
	called bool
}

func (s *stubEvalRunner) Run(ctx context.Context, datasetURI string, gates []manifest.EvaluationGateSpec) (*evaluation.EvalReport, error) {
	s.called = true
	return s.report, s.err
}

func TestCheckEvalGatesFailsOnOnReleaseBlockFailure(t *testing.T) {
	m := &manifest.SolutionManifest{
		Metadata: manifest.MetadataSpec{Name: "s", Version: "1"},
		Evaluation: manifest.EvaluationSpec{
			Datasets: []manifest.EvaluationDatasetSpec{{ID: "golden", URI: "golden.jsonl"}},
			Gates: []manifest.EvaluationGateSpec{{
				Metric: "citation_coverage", Min: 0.95, Severity: "block", Schedule: "onRelease",
			}},
		},
	}
	runner := &stubEvalRunner{report: &evaluation.EvalReport{
		GateResults: []evaluation.GateResult{{
			Metric: "citation_coverage", Min: 0.95, Actual: 0.5, Severity: "block", Schedule: "onRelease", Passed: false,
		}},
	}}
	checker := NewCheckerWithEvaluator(m, environment.ResolvedEnvironment{EnvironmentName: "production"}, runner)
	result := checker.checkEvalGates(context.Background())
	if result.Passed {
		t.Fatalf("expected eval_gates_passed to fail")
	}
	if !runner.called {
		t.Fatalf("expected evaluation runner to be called")
	}
}

func TestRunFailsWhenOnReleaseEvalGateFails(t *testing.T) {
	m := releaseQualityManifest()
	m.Runtime.ModelPolicy = manifest.ModelPolicySpec{DefaultModel: "gpt-4.1"}
	m.Runtime.Observability = manifest.ObservabilitySpec{Trace: "required"}
	m.Delivery.Security = manifest.SecuritySpec{
		PIIDetection:           "required",
		PromptInjectionDefense: "required",
	}
	m.Evaluation = manifest.EvaluationSpec{
		Datasets: []manifest.EvaluationDatasetSpec{{ID: "golden", URI: "golden.jsonl"}},
		Gates: []manifest.EvaluationGateSpec{{
			Metric: "citation_coverage", Min: 0.95, Severity: "block", Schedule: "onRelease",
		}},
	}
	env := releaseQualityEnv(t)
	t.Setenv("OPENAI_API_KEY", "test-key")
	writeKnowledgeQualityReport(t, env, matchingKnowledgeQualityReport(m))
	runner := &stubEvalRunner{report: &evaluation.EvalReport{
		GateResults: []evaluation.GateResult{{
			Metric: "citation_coverage", Min: 0.95, Actual: 0.5, Severity: "block", Schedule: "onRelease", Passed: false,
		}},
	}}
	checker := NewCheckerWithEvaluator(m, env, runner)

	report, err := checker.Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if report.Passed {
		t.Fatalf("expected release report to fail")
	}
	if !runner.called {
		t.Fatalf("expected evaluation runner to be called")
	}
	foundEvalCheck := false
	for _, check := range report.Checks {
		if check.Name == "eval_gates_passed" {
			foundEvalCheck = true
			if check.Passed {
				t.Fatalf("expected eval_gates_passed to fail")
			}
			continue
		}
		if !check.Passed {
			t.Fatalf("unexpected failed check %s: %s", check.Name, check.Message)
		}
	}
	if !foundEvalCheck {
		t.Fatalf("eval_gates_passed check not found: %#v", report.Checks)
	}
}

func TestCheckModelCredentialsFailsWithoutDefaultModel(t *testing.T) {
	m := releaseQualityManifest()
	env := releaseQualityEnv(t)
	result := NewChecker(m, env).checkModelCredentials(context.Background())
	if result.Passed {
		t.Fatalf("expected model credentials check to fail without default model")
	}
}

func TestRunDoesNotSkipMandatoryChecksWhenReleaseChecksProvided(t *testing.T) {
	m := releaseQualityManifest()
	m.Delivery.ReleaseChecks = []string{"observability_enabled"}
	env := releaseQualityEnv(t)

	report, err := NewChecker(m, env).Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(report.Checks) != len(mandatoryReleaseChecks) {
		t.Fatalf("checks len = %d, want %d: %#v", len(report.Checks), len(mandatoryReleaseChecks), report.Checks)
	}
	if !hasCheck(report, "knowledge_quality_passed") {
		t.Fatalf("expected mandatory knowledge check in report: %#v", report.Checks)
	}
}

func TestRunAlwaysExecutesMandatoryKnowledgeAndEvalChecks(t *testing.T) {
	m := releaseQualityManifest()
	m.Runtime.ModelPolicy.DefaultModel = "gpt-4.1"
	m.Delivery.ReleaseChecks = []string{"model_credentials_configured"}
	m.Evaluation = manifest.EvaluationSpec{
		Datasets: []manifest.EvaluationDatasetSpec{{ID: "golden", URI: "golden.jsonl"}},
		Gates: []manifest.EvaluationGateSpec{{
			Metric: "citation_coverage", Min: 0.95, Severity: "block", Schedule: "onRelease",
		}},
	}
	env := releaseQualityEnv(t)
	t.Setenv("OPENAI_API_KEY", "test-key")

	report, err := NewCheckerWithEvaluator(m, env, &stubEvalRunner{report: &evaluation.EvalReport{}}).Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if report.Passed {
		t.Fatalf("expected release to fail because mandatory knowledge report is missing")
	}
	if !hasCheck(report, "knowledge_quality_passed") {
		t.Fatalf("expected mandatory knowledge check in report: %#v", report.Checks)
	}
	if !hasCheck(report, "eval_gates_passed") {
		t.Fatalf("expected mandatory eval check in report: %#v", report.Checks)
	}
}

func TestCheckKnowledgeQualityFailsForStaleReport(t *testing.T) {
	m := releaseQualityManifest()
	env := releaseQualityEnv(t)
	report := matchingKnowledgeQualityReport(m)
	report["generatedAt"] = time.Now().Add(-25 * time.Hour).UTC()
	writeKnowledgeQualityReport(t, env, report)

	result := NewChecker(m, env).checkKnowledgeQuality(context.Background())
	if result.Passed {
		t.Fatalf("expected stale knowledge quality report to fail")
	}
}

func TestCheckKnowledgeQualityFailsWhenReportMissing(t *testing.T) {
	m := releaseQualityManifest()
	env := releaseQualityEnv(t)

	result := NewChecker(m, env).checkKnowledgeQuality(context.Background())
	if result.Passed {
		t.Fatalf("expected missing knowledge quality report to fail")
	}
	if _, err := os.Stat(env.ReportPath()); !os.IsNotExist(err) {
		t.Fatalf("release quality check must not create report, stat err = %v", err)
	}
}

func TestCheckKnowledgeQualityFailsForFingerprintMismatch(t *testing.T) {
	m := releaseQualityManifest()
	env := releaseQualityEnv(t)
	report := matchingKnowledgeQualityReport(m)
	report["knowledgeConfigFingerprint"] = "mismatch"
	writeKnowledgeQualityReport(t, env, report)

	result := NewChecker(m, env).checkKnowledgeQuality(context.Background())
	if result.Passed {
		t.Fatalf("expected mismatched knowledge config fingerprint to fail")
	}
}

func TestCheckKnowledgeQualityFailsForBlockFindings(t *testing.T) {
	m := releaseQualityManifest()
	env := releaseQualityEnv(t)
	report := matchingKnowledgeQualityReport(m)
	report["items"] = []map[string]any{{"severity": "block"}}
	writeKnowledgeQualityReport(t, env, report)

	result := NewChecker(m, env).checkKnowledgeQuality(context.Background())
	if result.Passed {
		t.Fatalf("expected block findings to fail")
	}
}

func TestCheckKnowledgeQualityPassesForFreshMatchingReport(t *testing.T) {
	m := releaseQualityManifest()
	env := releaseQualityEnv(t)
	writeKnowledgeQualityReport(t, env, matchingKnowledgeQualityReport(m))

	result := NewChecker(m, env).checkKnowledgeQuality(context.Background())
	if !result.Passed {
		t.Fatalf("expected matching knowledge quality report to pass: %#v", result)
	}
}

func releaseQualityManifest() *manifest.SolutionManifest {
	return &manifest.SolutionManifest{
		Metadata: manifest.MetadataSpec{Name: "s", Version: "1"},
		Knowledge: manifest.KnowledgeSpec{
			Sources: []manifest.KnowledgeSourceSpec{{ID: "faq", Type: "jsonl", URI: "knowledge.jsonl", Schema: "faq"}},
			Schemas: []manifest.KnowledgeSchemaSpec{{ID: "faq", Fields: []string{"question", "answer", "source_ref"}}},
			QualityGates: []manifest.QualityGateSpec{{
				Type:     "missing_required_fields",
				Severity: "block",
			}},
		},
		Runtime: manifest.RuntimeSpec{
			KnowledgeBindings: []manifest.KnowledgeBindingSpec{{Component: "retriever", Sources: []string{"faq"}}},
		},
	}
}

func matchingKnowledgeQualityReport(m *manifest.SolutionManifest) map[string]any {
	sources := []knowledge.SourceReport{{
		ID:          "faq",
		URI:         "knowledge.jsonl",
		ResolvedURI: "knowledge.jsonl",
		Records:     1,
	}}
	return map[string]any{
		"generatedAt":                 time.Now().UTC(),
		"manifestFingerprint":         knowledge.FingerprintManifest(m),
		"knowledgeConfigFingerprint":  knowledge.FingerprintKnowledgeConfig(m),
		"knowledgeSourcesFingerprint": knowledge.FingerprintSourceReports(sources),
		"sources":                     sources,
		"status":                      "passed",
		"items":                       []map[string]any{},
	}
}

func releaseQualityEnv(t *testing.T) environment.ResolvedEnvironment {
	t.Helper()
	return environment.ResolvedEnvironment{
		EnvironmentName: "production",
		TracePath:       filepath.Join(t.TempDir(), "traces"),
	}
}

func writeKnowledgeQualityReport(t *testing.T, env environment.ResolvedEnvironment, report map[string]any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(env.ReportPath()), 0o755); err != nil {
		t.Fatalf("mkdir report dir: %v", err)
	}
	bytes, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal report: %v", err)
	}
	if err := os.WriteFile(env.ReportPath(), bytes, 0o644); err != nil {
		t.Fatalf("write report: %v", err)
	}
}

func hasCheck(report *ReleaseReport, name string) bool {
	for _, check := range report.Checks {
		if check.Name == name {
			return true
		}
	}
	return false
}

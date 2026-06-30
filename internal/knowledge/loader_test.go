package knowledge

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"fde-support/internal/environment"
	"fde-support/internal/manifest"
)

func TestLoadWritesReleaseCompatibleQualityReport(t *testing.T) {
	m, env := qualityReportFixture(t)

	_, report, err := Load(context.Background(), m, env)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	assertReleaseCompatibleQualityReport(t, m, env, report)
}

func TestIngestWritesReleaseCompatibleQualityReport(t *testing.T) {
	m, env := qualityReportFixture(t)

	if _, err := Ingest(context.Background(), m, env); err != nil {
		t.Fatalf("Ingest() error = %v", err)
	}

	bytes, err := os.ReadFile(env.ReportPath())
	if err != nil {
		t.Fatalf("read quality report: %v", err)
	}
	var report QualityReport
	if err := json.Unmarshal(bytes, &report); err != nil {
		t.Fatalf("unmarshal quality report: %v", err)
	}
	assertReleaseCompatibleQualityReport(t, m, env, &report)
}

func qualityReportFixture(t *testing.T) (*manifest.SolutionManifest, environment.ResolvedEnvironment) {
	t.Helper()

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "data"), 0o755); err != nil {
		t.Fatalf("mkdir data: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "data", "knowledge.jsonl"), []byte(`{"question":"Q","answer":"A","source_ref":"faq#1"}`+"\n"), 0o644); err != nil {
		t.Fatalf("write knowledge source: %v", err)
	}
	m := &manifest.SolutionManifest{
		BaseDir: root,
		Metadata: manifest.MetadataSpec{
			Name:    "s",
			Version: "1",
		},
		Knowledge: manifest.KnowledgeSpec{
			Sources: []manifest.KnowledgeSourceSpec{{ID: "faq", Type: "jsonl", URI: "data/knowledge.jsonl", Schema: "faq"}},
			Schemas: []manifest.KnowledgeSchemaSpec{{ID: "faq", Fields: []string{"question", "answer", "source_ref"}}},
		},
		Runtime: manifest.RuntimeSpec{
			KnowledgeBindings: []manifest.KnowledgeBindingSpec{{Component: "retriever", Sources: []string{"faq"}}},
		},
	}
	env := environment.ResolvedEnvironment{
		EnvironmentName: "poc",
		TracePath:       filepath.Join(root, ".solution", "traces"),
	}
	return m, env
}

func assertReleaseCompatibleQualityReport(t *testing.T, m *manifest.SolutionManifest, env environment.ResolvedEnvironment, report *QualityReport) {
	t.Helper()

	if report.ManifestFingerprint != FingerprintManifest(m) {
		t.Fatalf("ManifestFingerprint = %q, want %q", report.ManifestFingerprint, FingerprintManifest(m))
	}
	if report.KnowledgeConfigFingerprint != FingerprintKnowledgeConfig(m) {
		t.Fatalf("KnowledgeConfigFingerprint = %q, want %q", report.KnowledgeConfigFingerprint, FingerprintKnowledgeConfig(m))
	}
	if report.KnowledgeSourcesFingerprint != FingerprintSourceReports(report.Sources) {
		t.Fatalf("KnowledgeSourcesFingerprint = %q, want %q", report.KnowledgeSourcesFingerprint, FingerprintSourceReports(report.Sources))
	}
	if report.Status != "passed" {
		t.Fatalf("Status = %q, want passed", report.Status)
	}
	if _, err := os.Stat(env.ReportPath()); err != nil {
		t.Fatalf("expected report to be written: %v", err)
	}
}

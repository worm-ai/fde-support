package delivery

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fde-support/internal/environment"
	"fde-support/internal/manifest"
)

func TestGenerateDockerComposeCopiesReferencedRuntimeInputs(t *testing.T) {
	baseDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(baseDir, "manifest.yaml"), []byte("apiVersion: solution.codex/v1\nkind: Solution\n"), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if err := os.Mkdir(filepath.Join(baseDir, "data"), 0o755); err != nil {
		t.Fatalf("mkdir data: %v", err)
	}
	if err := os.WriteFile(filepath.Join(baseDir, "data", "knowledge_units.jsonl"), []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("write data: %v", err)
	}

	m := &manifest.SolutionManifest{
		APIVersion: "solution.codex/v1",
		Kind:       "Solution",
		Metadata:   manifest.MetadataSpec{Name: "s", Version: "1.0.0"},
		BaseDir:    baseDir,
		Path:       filepath.Join(baseDir, "manifest.yaml"),
		Knowledge: manifest.KnowledgeSpec{
			Sources: []manifest.KnowledgeSourceSpec{{ID: "faq", Type: "jsonl", URI: "./data/knowledge_units.jsonl"}},
		},
	}
	outputDir := filepath.Join(t.TempDir(), "deploy", "poc")

	if err := GenerateDockerCompose(m, environment.ResolvedEnvironment{EnvironmentName: "poc"}, outputDir); err != nil {
		t.Fatalf("GenerateDockerCompose() error = %v", err)
	}

	for _, rel := range []string{"docker-compose.yaml", ".env.example", "README.md", "manifest.yaml", filepath.Join("data", "knowledge_units.jsonl")} {
		if _, err := os.Stat(filepath.Join(outputDir, rel)); err != nil {
			t.Fatalf("expected %s in output: %v", rel, err)
		}
	}

	compose, err := os.ReadFile(filepath.Join(outputDir, "docker-compose.yaml"))
	if err != nil {
		t.Fatalf("read compose: %v", err)
	}
	content := string(compose)
	if strings.Contains(content, "build: .") {
		t.Fatalf("compose must not use build context unless Dockerfile is generated:\n%s", content)
	}
	if !strings.Contains(content, "./manifest.yaml:/manifest/manifest.yaml:ro") {
		t.Fatalf("compose should mount copied manifest:\n%s", content)
	}
}

func TestGenerateDockerComposeRejectsEscapingKnowledgeSource(t *testing.T) {
	baseDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(baseDir, "manifest.yaml"), []byte("kind: Solution\n"), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	outside := filepath.Join(filepath.Dir(baseDir), "secret.txt")
	if err := os.WriteFile(outside, []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("write outside file: %v", err)
	}
	m := &manifest.SolutionManifest{
		Metadata: manifest.MetadataSpec{Name: "s", Version: "1.0.0"},
		BaseDir:  baseDir,
		Path:     filepath.Join(baseDir, "manifest.yaml"),
		Knowledge: manifest.KnowledgeSpec{
			Sources: []manifest.KnowledgeSourceSpec{{ID: "leak", Type: "jsonl", URI: "../secret.txt"}},
		},
	}

	err := GenerateDockerCompose(m, environment.ResolvedEnvironment{EnvironmentName: "poc"}, filepath.Join(t.TempDir(), "deploy"))
	if err == nil {
		t.Fatalf("expected path escape error")
	}
}

func TestGenerateDockerComposeDocumentsRuntimeImage(t *testing.T) {
	baseDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(baseDir, "manifest.yaml"), []byte("kind: Solution\n"), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	m := &manifest.SolutionManifest{
		Metadata: manifest.MetadataSpec{Name: "s", Version: "1.0.0"},
		BaseDir:  baseDir,
		Path:     filepath.Join(baseDir, "manifest.yaml"),
	}
	outputDir := filepath.Join(t.TempDir(), "deploy")
	if err := GenerateDockerCompose(m, environment.ResolvedEnvironment{EnvironmentName: "poc"}, outputDir); err != nil {
		t.Fatalf("GenerateDockerCompose() error = %v", err)
	}
	readme, err := os.ReadFile(filepath.Join(outputDir, "README.md"))
	if err != nil {
		t.Fatalf("read README: %v", err)
	}
	if !strings.Contains(string(readme), "solution-runtime:1.0.0") {
		t.Fatalf("README must document required runtime image:\n%s", string(readme))
	}
}

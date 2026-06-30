package registry

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

func TestPublishComponentPackagesComponentYAMLAndSource(t *testing.T) {
	componentDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(componentDir, "component.yaml"), []byte(`
ref: registry.team.echo@1.0.0
category: processor
factory: echo
inputSchema:
  message: string
outputSchema:
  answer: string
`), 0o644); err != nil {
		t.Fatalf("write component.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(componentDir, "component.go"), []byte("package echo\n"), 0o644); err != nil {
		t.Fatalf("write component.go: %v", err)
	}

	outputDir := filepath.Join(t.TempDir(), "published")
	artifact, err := PublishComponent(componentDir, outputDir)
	if err != nil {
		t.Fatalf("PublishComponent() error = %v", err)
	}
	if filepath.Base(artifact) != "registry.team.echo@1.0.0.tar.gz" {
		t.Fatalf("artifact = %q", artifact)
	}

	names := tarGzNames(t, artifact)
	if !names["component.yaml"] || !names["component.go"] {
		t.Fatalf("archive names = %#v, want component.yaml and component.go", names)
	}
}

func TestPublishComponentRejectsMissingRef(t *testing.T) {
	componentDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(componentDir, "component.yaml"), []byte("category: processor\n"), 0o644); err != nil {
		t.Fatalf("write component.yaml: %v", err)
	}
	_, err := PublishComponent(componentDir, t.TempDir())
	if err == nil {
		t.Fatalf("expected missing ref error")
	}
}

func tarGzNames(t *testing.T, path string) map[string]bool {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open artifact: %v", err)
	}
	defer file.Close()
	gz, err := gzip.NewReader(file)
	if err != nil {
		t.Fatalf("gzip reader: %v", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	out := map[string]bool{}
	for {
		header, err := tr.Next()
		if err != nil {
			break
		}
		out[header.Name] = true
	}
	return out
}

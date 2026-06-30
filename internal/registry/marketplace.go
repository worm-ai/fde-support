package registry

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// PublishComponent packages a component directory into a .tar.gz archive.
func PublishComponent(componentDir, outputDir string) (string, error) {
	// Read component.yaml to get name and version
	componentPath := filepath.Join(componentDir, "component.yaml")
	data, err := os.ReadFile(componentPath)
	if err != nil {
		return "", fmt.Errorf("read component.yaml: %w", err)
	}

	var info struct {
		Ref string `yaml:"ref"`
	}
	if err := yamlUnmarshal(data, &info); err != nil {
		return "", fmt.Errorf("parse component.yaml: %w", err)
	}
	if info.Ref == "" {
		return "", fmt.Errorf("component ref is required in component.yaml")
	}

	archiveName := sanitizeRef(info.Ref) + ".tar.gz"
	archivePath := filepath.Join(outputDir, archiveName)

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return "", fmt.Errorf("create output directory: %w", err)
	}

	file, err := os.Create(archivePath)
	if err != nil {
		return "", fmt.Errorf("create archive: %w", err)
	}
	defer file.Close()

	gw := gzip.NewWriter(file)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	defer tw.Close()

	return archivePath, filepath.Walk(componentDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(componentDir, path)
		if err != nil {
			return err
		}

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = relPath

		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		src, err := os.Open(path)
		if err != nil {
			return err
		}
		defer src.Close()

		if _, err := io.Copy(tw, src); err != nil {
			return err
		}
		return nil
	})
}

// ReuseStats computes component and template reuse ratios for a manifest.
type ReuseStats struct {
	TotalComponents  int     `json:"totalComponents"`
	ReusedComponents int     `json:"reusedComponents"`
	ReuseRatio       float64 `json:"reuseRatio"`
	CustomComponents int     `json:"customComponents"`
	TemplateUsed     bool    `json:"templateUsed"`
}

// ComputeReuseStats calculates reuse metrics for a solution manifest.
func ComputeReuseStats(m interface{}) ReuseStats {
	// Placeholder for reuse statistics computation
	return ReuseStats{
		TotalComponents:  0,
		ReusedComponents: 0,
		ReuseRatio:       0,
		CustomComponents: 0,
		TemplateUsed:     false,
	}
}

func sanitizeRef(ref string) string {
	result := ""
	for _, r := range ref {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '.' || r == '@' {
			result += string(r)
		} else {
			result += "-"
		}
	}
	return result
}

func yamlUnmarshal(data []byte, v interface{}) error {
	return yaml.Unmarshal(data, v)
}

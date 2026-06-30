package registry

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

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
	// Use interface{} to avoid circular import with manifest package.
	// Extract components via reflection-like approach using common fields.
	type componentLike struct {
		Ref string
	}
	type manifestLike struct {
		Components   []componentLike
		SolutionType string
	}

	// Attempt to unmarshal from map or concrete type
	var ml manifestLike
	if stringMap, ok := m.(map[string]any); ok {
		if comps, ok := stringMap["components"].([]any); ok {
			for _, c := range comps {
				if cm, ok := c.(map[string]any); ok {
					ref, _ := cm["ref"].(string)
					ml.Components = append(ml.Components, componentLike{Ref: ref})
				}
			}
		}
		ml.SolutionType, _ = stringMap["solutionType"].(string)
	} else {
		// If not a map, return empty stats
		return ReuseStats{}
	}

	stats := ReuseStats{
		TotalComponents: len(ml.Components),
	}

	for _, comp := range ml.Components {
		if strings.HasPrefix(comp.Ref, "registry.") {
			stats.ReusedComponents++
		} else {
			stats.CustomComponents++
		}
	}

	if stats.TotalComponents > 0 {
		stats.ReuseRatio = float64(stats.ReusedComponents) / float64(stats.TotalComponents)
	}

	stats.TemplateUsed = ml.SolutionType != "" && strings.Contains(strings.ToLower(ml.SolutionType), "support")

	return stats
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

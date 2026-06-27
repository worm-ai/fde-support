package manifest

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

func LoadFile(path string) (*SolutionManifest, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m SolutionManifest
	if err := yaml.Unmarshal(bytes, &m); err != nil {
		return nil, err
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	m.Path = abs
	m.BaseDir = filepath.Dir(abs)
	return &m, nil
}

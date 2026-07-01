package knowledge

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// PythonBridge invokes the Python knowledge worker as a subprocess.
type PythonBridge struct {
	PythonBin string
	ScriptDir string
}

// NewPythonBridge creates a bridge with defaults for python3 and the workers directory.
func NewPythonBridge(projectRoot string) *PythonBridge {
	if projectRoot == "" {
		projectRoot = "."
	}
	return &PythonBridge{
		PythonBin: "python3",
		ScriptDir: filepath.Join(projectRoot, "workers", "knowledge"),
	}
}

// Run invokes the Python parser script with the given input file and output path.
// Returns the number of parsed records or an error.
func (b *PythonBridge) Run(inputPath, outputPath string) (int, error) {
	script := filepath.Join(b.ScriptDir, "parser.py")
	if _, err := os.Stat(script); err != nil {
		return 0, fmt.Errorf("python parser script not found: %w", err)
	}
	if _, err := os.Stat(inputPath); err != nil {
		return 0, fmt.Errorf("input file not found: %w", err)
	}

	cmd := exec.Command(b.PythonBin, script, inputPath, outputPath)
	cmd.Stderr = os.Stderr
	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("python worker failed: %w (output: %s)", err, string(output))
	}

	// Parse output for record count
	var records int
	fmt.Sscanf(string(output), "Parsed %d records", &records)
	return records, nil
}

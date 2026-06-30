package delivery

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"fde-support/internal/environment"
	"fde-support/internal/manifest"
)

// GenerateDockerCompose creates a docker-compose.yaml and .env.example for production deployment.
func GenerateDockerCompose(m *manifest.SolutionManifest, env environment.ResolvedEnvironment, outputDir string) error {
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	if err := copyRuntimeInputs(m, outputDir); err != nil {
		return err
	}

	// Write docker-compose.yaml
	compose := generateComposeContent(m, env)
	composePath := filepath.Join(outputDir, "docker-compose.yaml")
	if err := os.WriteFile(composePath, []byte(compose), 0o644); err != nil {
		return fmt.Errorf("write docker-compose.yaml: %w", err)
	}

	// Write .env.example
	envExample := generateEnvExample(m, env)
	envPath := filepath.Join(outputDir, ".env.example")
	if err := os.WriteFile(envPath, []byte(envExample), 0o644); err != nil {
		return fmt.Errorf("write .env.example: %w", err)
	}

	// Write README
	readme := generateReadme(m, env)
	readmePath := filepath.Join(outputDir, "README.md")
	if err := os.WriteFile(readmePath, []byte(readme), 0o644); err != nil {
		return fmt.Errorf("write README.md: %w", err)
	}

	return nil
}

func generateComposeContent(m *manifest.SolutionManifest, env environment.ResolvedEnvironment) string {
	lines := []string{
		"version: \"3.8\"",
		"services:",
		"  solution-runtime:",
		fmt.Sprintf("    image: solution-runtime:%s", m.Metadata.Version),
		"    command: [\"solution\", \"run\", \"/manifest/manifest.yaml\", \"--env\", \"" + env.EnvironmentName + "\", \"--addr\", \"0.0.0.0:8080\"]",
		"    ports:",
		"      - \"8080:8080\"",
		"    volumes:",
		"      - ./manifest.yaml:/manifest/manifest.yaml:ro",
		"      - ./data:/manifest/data:ro",
		"      - solution-traces:/manifest/.solution/traces",
		"    environment:",
	}

	sensorTokens := collectSensorTokens(m)
	for _, token := range sensorTokens {
		lines = append(lines, fmt.Sprintf("      - %s=${%s}", token, strings.TrimPrefix(token, "env:")))
	}
	lines = append(lines, "      - OPENAI_API_KEY=${OPENAI_API_KEY}")
	lines = append(lines, "")
	lines = append(lines, "volumes:")
	lines = append(lines, "  solution-traces:")
	lines = append(lines, "")

	return strings.Join(lines, "\n")
}

func generateEnvExample(m *manifest.SolutionManifest, env environment.ResolvedEnvironment) string {
	lines := []string{
		"# Solution-as-Code FDE Platform - Environment Variables",
		fmt.Sprintf("# Solution: %s v%s", m.Metadata.Name, m.Metadata.Version),
		fmt.Sprintf("# Environment: %s", env.EnvironmentName),
		"",
		"# Model credentials (required)",
		"OPENAI_API_KEY=sk-...",
		"",
	}

	sensorTokens := collectSensorTokens(m)
	for _, token := range sensorTokens {
		lines = append(lines, fmt.Sprintf("# Sensor auth token for %s", token))
		lines = append(lines, fmt.Sprintf("%s=change-me", strings.TrimPrefix(token, "env:")))
		lines = append(lines, "")
	}

	lines = append(lines, "# Action credentials")
	for _, comp := range m.Components {
		if comp.Category != "action" {
			continue
		}
		for _, val := range comp.Config {
			if s, ok := val.(string); ok && strings.HasPrefix(s, "env:") {
				lines = append(lines, fmt.Sprintf("%s=change-me  # for action %s", strings.TrimPrefix(s, "env:"), comp.ID))
			}
		}
	}

	return strings.Join(lines, "\n")
}

func generateReadme(m *manifest.SolutionManifest, env environment.ResolvedEnvironment) string {
	lines := []string{
		fmt.Sprintf("# %s", m.Metadata.Name),
		"",
		fmt.Sprintf("Version: %s", m.Metadata.Version),
		fmt.Sprintf("Environment: %s", env.EnvironmentName),
		"",
		"## Deployment",
		"",
		"1. Copy `.env.example` to `.env` and fill in the required credentials.",
		"2. Run `docker-compose up -d` to start the solution runtime.",
		"3. Verify the service is running: `curl http://localhost:8080/health`",
		"",
		"## Runtime Image",
		"",
		fmt.Sprintf("This deployment expects the Docker image `solution-runtime:%s` to be available on the target host or registry before running `docker-compose up -d`.", m.Metadata.Version),
		"Build or publish that image from the platform runtime before using this deployment package.",
		"",
		"## Rebuilding",
		"",
		"After updating the manifest or data files:",
		"```bash",
		"docker-compose down",
		"docker-compose up -d",
		"```",
		"",
		"## Manifest",
		"",
		fmt.Sprintf("The solution is defined by `manifest.yaml` in the `%s` directory.", filepath.Dir(m.Path)),
		"All configuration (components, workflows, knowledge bindings, model policies) is declared in the manifest.",
		"Modify the manifest to change solution behavior without writing code.",
	}
	return strings.Join(lines, "\n")
}

func copyRuntimeInputs(m *manifest.SolutionManifest, outputDir string) error {
	if m.Path != "" {
		if err := copyFile(m.Path, filepath.Join(outputDir, "manifest.yaml")); err != nil {
			return fmt.Errorf("copy manifest.yaml: %w", err)
		}
	}
	for _, source := range m.Knowledge.Sources {
		if source.URI == "" || filepath.IsAbs(source.URI) {
			continue
		}
		cleanURI := filepath.Clean(source.URI)
		if cleanURI == "." || cleanURI == ".." || strings.HasPrefix(cleanURI, ".."+string(filepath.Separator)) {
			return fmt.Errorf("knowledge source %s escapes manifest directory", source.URI)
		}
		src := filepath.Join(m.BaseDir, cleanURI)
		if !containedPath(m.BaseDir, src) {
			return fmt.Errorf("knowledge source %s escapes manifest directory", source.URI)
		}
		dst := filepath.Join(outputDir, cleanURI)
		if !containedPath(outputDir, dst) {
			return fmt.Errorf("knowledge source %s escapes output directory", source.URI)
		}
		info, err := os.Stat(src)
		if err != nil {
			return fmt.Errorf("stat knowledge source %s: %w", source.URI, err)
		}
		if info.IsDir() {
			continue
		}
		if err := copyFile(src, dst); err != nil {
			return fmt.Errorf("copy knowledge source %s: %w", source.URI, err)
		}
	}
	return nil
}

func containedPath(base, target string) bool {
	baseAbs, err := filepath.Abs(base)
	if err != nil {
		return false
	}
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(baseAbs, targetAbs)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}

func collectSensorTokens(m *manifest.SolutionManifest) []string {
	var tokens []string
	for _, sensor := range m.Perception.Sensors {
		if ref, ok := sensor.Config["authTokenRef"].(string); ok && strings.HasPrefix(ref, "env:") {
			tokens = append(tokens, ref)
		}
	}
	return tokens
}

# Design Review Remediation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 修复 2026-06-30 设计审查发现的致命和高风险差距，使项目从 PoC 进入可验证、可发布、符合详细设计核心约束的 MVP 状态。

**Architecture:** 按垂直链路修复：先恢复组件发布和部署包这两个完全阻塞的 Phase 4 流程，再接入真实模型网关和 release 只读门禁，最后补齐 Manifest schema、Logger 脱敏、releaseChecks 和评测指标一致性。每个任务都必须先写失败测试，再做最小实现，并用本地 Go 完整验证。

**Tech Stack:** Go 1.21, Cobra CLI, YAML manifests via `gopkg.in/yaml.v3`, file-based Trace/quality reports, Docker Compose artifact generation, OpenAI-compatible chat completions provider.

---

## Context For Workers

先阅读：

- `docs/reviews/2026-06-30-design-implementation-review.md`
- `docs/solution-as-code-fde-platform-design.md`
- `docs/solution-as-code-fde-platform-technical-architecture.md`
- `docs/specs/attachment-3-trace-json-schema.md`
- `docs/specs/attachment-4-golden-case-jsonl.md`

只修改源码、测试和必要示例。不要重构无关模块。

本地 Go：

```powershell
& '.\.tools\go1.21.13\go\bin\go.exe' version
& '.\.tools\go1.21.13\go\bin\go.exe' test ./cmd/... ./internal/...
```

当前基线应为：

```text
go version go1.21.13 windows/amd64
all cmd/internal tests pass
```

## Target File Map

| Area | Files |
|---|---|
| Component publish | `internal/registry/marketplace.go`, new `internal/registry/marketplace_test.go`, `internal/app/app.go` if output target changes |
| Delivery artifacts | `internal/delivery/docker_compose.go`, new `internal/delivery/docker_compose_test.go`, `internal/app/app.go`, optional command test in `cmd/solution/main_test.go` |
| Model gateway wiring | `internal/app/app.go`, new helper file `internal/app/model_gateway.go` if cleaner, `internal/model/*`, app tests |
| Release quality gate semantics | `internal/knowledge/loader.go`, `internal/app/app.go`, `internal/release/checker.go`, `internal/release/checker_test.go` |
| Logger redaction | `internal/runtimecore/types.go`, `internal/trace/redactor.go`, `internal/runtimecore/*_test.go` or `internal/trace/*_test.go` |
| Manifest consistency | `internal/manifest/types.go`, `internal/manifest/validator.go`, `internal/manifest/validator_test.go`, `examples/*/manifest.yaml`, `templates/*.yaml` |
| Evaluation metrics | `internal/evaluation/types.go`, `internal/evaluation/metrics.go`, `internal/evaluation/metrics_test.go`, `internal/manifest/validator.go` |

---

## Task 1: Fix `solution component-publish`

**Files:**

- Modify: `internal/registry/marketplace.go`
- Create: `internal/registry/marketplace_test.go`

- [ ] **Step 1: Write failing tests for publish success and invalid component yaml**

Create `internal/registry/marketplace_test.go`:

```go
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
```

- [ ] **Step 2: Run the targeted failing test**

Run:

```powershell
& '.\.tools\go1.21.13\go\bin\go.exe' test ./internal/registry -run TestPublishComponent -count=1
```

Expected before implementation:

```text
FAIL
parse component.yaml: yaml parsing not available in marketplace
```

- [ ] **Step 3: Replace the YAML stub with real parsing**

In `internal/registry/marketplace.go`:

- Import `gopkg.in/yaml.v3`.
- Replace `yamlUnmarshalFunc` with direct `yaml.Unmarshal`.
- Remove the fixed-error stub.
- Keep `sanitizeRef` behavior unless tests require a safer filename.

Implementation shape:

```go
func yamlUnmarshal(data []byte, v interface{}) error {
	return yaml.Unmarshal(data, v)
}
```

- [ ] **Step 4: Verify registry tests**

Run:

```powershell
& '.\.tools\go1.21.13\go\bin\go.exe' test ./internal/registry -count=1
```

Expected:

```text
ok  	fde-support/internal/registry
```

---

## Task 2: Make Release Artifacts Self-Contained

**Files:**

- Modify: `internal/delivery/docker_compose.go`
- Create: `internal/delivery/docker_compose_test.go`
- Modify: `internal/app/app.go` only if passing source manifest path/base dir into delivery is needed

- [ ] **Step 1: Write failing test for artifact references**

Create `internal/delivery/docker_compose_test.go`:

```go
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
```

- [ ] **Step 2: Run targeted failing test**

Run:

```powershell
& '.\.tools\go1.21.13\go\bin\go.exe' test ./internal/delivery -run TestGenerateDockerComposeCopiesReferencedRuntimeInputs -count=1
```

Expected before implementation:

```text
FAIL
expected manifest.yaml in output
```

- [ ] **Step 3: Copy manifest and knowledge data into outputDir**

In `GenerateDockerCompose`:

- Copy `m.Path` to `outputDir/manifest.yaml` when `m.Path` is set.
- Copy every relative knowledge source file into the same relative path under `outputDir`.
- Create parent directories.
- Use read/write copy with regular file mode `0o644`.
- Keep behavior simple: copy files only, not directories recursively unless a knowledge source URI is a directory.

- [ ] **Step 4: Make compose runnable without missing build context**

In `generateComposeContent`:

- Remove `build: .`.
- Use an image name that can be overridden later, for now `image: solution-runtime:<version>`.
- Keep manifest/data volume mounts consistent with files copied into outputDir.

- [ ] **Step 5: Verify delivery tests and release smoke**

Run:

```powershell
& '.\.tools\go1.21.13\go\bin\go.exe' test ./internal/delivery -count=1
& '.\.tools\go1.21.13\go\bin\go.exe' test ./cmd/... ./internal/... -count=1
```

Expected:

```text
ok  	fde-support/internal/delivery
all cmd/internal tests pass
```

---

## Task 3: Wire Runtime to Real Model Gateway

**Files:**

- Modify: `internal/app/app.go`
- Optional create: `internal/app/model_gateway.go`
- Test: add focused tests in `internal/app` if test package exists, otherwise `internal/model` plus app-level dependency injection refactor

- [ ] **Step 1: Add a gateway builder**

Create `internal/app/model_gateway.go`:

```go
package app

import (
	"fmt"

	"fde-support/internal/environment"
	"fde-support/internal/model"
	"fde-support/internal/registry"
)

func buildModelGateway(env environment.ResolvedEnvironment, allowMock bool) (registry.ModelGateway, error) {
	if env.DefaultModel == "" {
		if allowMock {
			return model.NewMockGateway(), nil
		}
		return nil, fmt.Errorf("runtime.modelPolicy.defaultModel is required")
	}
	keyRef := env.ModelKeyRef
	if keyRef == "" {
		keyRef = "env:OPENAI_API_KEY"
	}
	apiKey, ok := env.ResolveSecret(keyRef)
	if !ok || apiKey == "" {
		if allowMock {
			return model.NewMockGateway(), nil
		}
		return nil, fmt.Errorf("model key not configured: %s", keyRef)
	}
	return model.NewGateway(
		model.NewOpenAIProvider(apiKey),
		nil,
		env.DefaultModel,
		env.FallbackModel,
		env.MaxLatencyMs,
	), nil
}
```

- [ ] **Step 2: Replace app-level `model.NewMockGateway()` calls**

In `internal/app/app.go`:

- In `BuildRuntime`, call `buildModelGateway(resolvedEnv, false)`.
- In `EvaluateManifestFile`, call `buildModelGateway(resolvedEnv, false)` unless tests require explicit mock mode.
- In `ReleaseManifestFile`, call `buildModelGateway(resolvedEnv, false)` after release credential check or before evaluator construction if required.
- Keep existing tests passing by setting env vars in tests or introducing a test-only helper that passes `allowMock=true`.

- [ ] **Step 3: Add test for missing model config**

Add or update tests so a manifest with model-dependent components and missing `defaultModel` or env key fails before runtime starts. The expected error should contain either:

```text
runtime.modelPolicy.defaultModel is required
```

or:

```text
model key not configured
```

- [ ] **Step 4: Verify app/model-related packages**

Run:

```powershell
& '.\.tools\go1.21.13\go\bin\go.exe' test ./internal/app ./internal/model ./internal/runtimecore ./cmd/solution -count=1
```

Expected:

```text
ok or ? for listed packages
```

---

## Task 4: Make Release Knowledge Gate Read-Only

**Files:**

- Modify: `internal/knowledge/loader.go`
- Modify: `internal/app/app.go`
- Modify: `internal/release/checker_test.go`

- [ ] **Step 1: Add failing release test that missing report stays missing**

Add to `internal/release/checker_test.go` or a new app-level release test if easier:

```go
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
```

- [ ] **Step 2: Introduce knowledge load options**

In `internal/knowledge/loader.go`, add:

```go
type LoadOptions struct {
	WriteReport bool
}

func Load(ctx context.Context, m *manifest.SolutionManifest, env environment.ResolvedEnvironment) (*Store, *QualityReport, error) {
	return LoadWithOptions(ctx, m, env, LoadOptions{WriteReport: true})
}

func LoadWithOptions(ctx context.Context, m *manifest.SolutionManifest, env environment.ResolvedEnvironment, opts LoadOptions) (*Store, *QualityReport, error) {
	// move existing Load body here
	// only call writeReport when opts.WriteReport is true
}
```

Existing `Load` behavior must remain unchanged for `run` and `ingest` compatibility.

- [ ] **Step 3: Use no-report loading in release**

In `internal/app/app.go` `ReleaseManifestFile`:

- Run checker before any loader that writes reports.
- If evaluator needs a knowledge store, use `knowledge.LoadWithOptions(..., knowledge.LoadOptions{WriteReport: false})`.
- Do not create `knowledge-quality.json` during release.

- [ ] **Step 4: Verify release tests**

Run:

```powershell
& '.\.tools\go1.21.13\go\bin\go.exe' test ./internal/knowledge ./internal/release ./internal/app -count=1
```

Expected:

```text
ok or ? for listed packages
```

---

## Task 5: Redact Runtime Logger Output

**Files:**

- Modify: `internal/runtimecore/types.go`
- Optional modify: `internal/trace/redactor.go`
- Test: new `internal/runtimecore/types_test.go`

- [ ] **Step 1: Write logger redaction test**

Create `internal/runtimecore/types_test.go`:

```go
package runtimecore

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func TestRuntimeLoggerRedactsSensitiveFields(t *testing.T) {
	stderr := captureStderr(t, func() {
		runtimeLogger{traceID: "trace-1"}.Info("", "call", map[string]any{
			"Authorization": "Bearer secret",
			"phone":         "13800138000",
			"email":         "user@example.com",
			"raw_payload":   map[string]any{"token": "secret"},
		})
	})
	if strings.Contains(stderr, "Bearer secret") || strings.Contains(stderr, "13800138000") || strings.Contains(stderr, "user@example.com") || strings.Contains(stderr, "raw_payload") {
		t.Fatalf("stderr not redacted: %s", stderr)
	}
	if !strings.Contains(stderr, "[trace-1]") {
		t.Fatalf("stderr missing trace id: %s", stderr)
	}
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stderr = w
	fn()
	_ = w.Close()
	os.Stderr = old
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	_ = r.Close()
	return buf.String()
}
```

- [ ] **Step 2: Implement logger redaction**

In `internal/runtimecore/types.go`:

- Import `fde-support/internal/trace`.
- Redact fields before `fmt.Fprintf`.
- Prefer `l.traceID` when non-empty; otherwise use method argument.

Implementation shape:

```go
func (l runtimeLogger) Info(traceID string, msg string, fields map[string]any) {
	fmt.Fprintf(os.Stderr, "[%s] INFO: %s %v\n", l.effectiveTraceID(traceID), msg, trace.RedactMap(fields))
}
```

- [ ] **Step 3: Verify runtimecore tests**

Run:

```powershell
& '.\.tools\go1.21.13\go\bin\go.exe' test ./internal/runtimecore -count=1
```

Expected:

```text
ok  	fde-support/internal/runtimecore
```

---

## Task 6: Tighten Release Credential and `releaseChecks` Semantics

**Files:**

- Modify: `internal/release/checker.go`
- Modify: `internal/release/checker_test.go`
- Modify: `internal/manifest/validator.go`
- Modify: `internal/manifest/validator_test.go`

- [ ] **Step 1: Add model credential failure test**

Add to `internal/release/checker_test.go`:

```go
func TestCheckModelCredentialsFailsWithoutDefaultModel(t *testing.T) {
	m := releaseQualityManifest()
	env := releaseQualityEnv(t)
	result := NewChecker(m, env).checkModelCredentials(context.Background())
	if result.Passed {
		t.Fatalf("expected model credentials check to fail without default model")
	}
}
```

- [ ] **Step 2: Change `checkModelCredentials`**

In `internal/release/checker.go`:

- If `c.env.DefaultModel` and `c.manifest.Runtime.ModelPolicy.DefaultModel` are both empty, return failed block.
- Resolve keyRef from env or default `env:OPENAI_API_KEY`.
- Fail when secret is missing.

- [ ] **Step 3: Validate unknown release checks**

In `internal/manifest/validator.go`:

- Add a supported release check set:

```go
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
```

- During validation, for each `m.Delivery.ReleaseChecks`, add `UNKNOWN_RELEASE_CHECK` when unsupported.

- [ ] **Step 4: Decide and implement empty `releaseChecks` behavior**

Use this project policy unless user says otherwise:

- `releaseChecks` omitted or empty means use default full check set.
- Non-empty means run exactly the declared known checks.

Update `Checker.Run` accordingly by filtering the `checks` list.

- [ ] **Step 5: Verify release and manifest tests**

Run:

```powershell
& '.\.tools\go1.21.13\go\bin\go.exe' test ./internal/release ./internal/manifest -count=1
```

Expected:

```text
ok  	fde-support/internal/release
ok  	fde-support/internal/manifest
```

---

## Task 7: Add Manifest `solutionType` and `apiVersion` Compatibility Checks

**Files:**

- Modify: `internal/manifest/types.go`
- Modify: `internal/manifest/validator.go`
- Modify: `internal/manifest/validator_test.go`
- Modify: `examples/after-sales-support/manifest.yaml`
- Modify: `examples/guoran-support/manifest.yaml`
- Modify: `templates/*.yaml`

- [ ] **Step 1: Add `SolutionType` field**

In `internal/manifest/types.go`:

```go
SolutionType string `yaml:"solutionType" json:"solutionType"`
```

Place it next to `Kind`/`Metadata`.

- [ ] **Step 2: Validate required `solutionType`**

In `internal/manifest/validator.go`:

- Add missing field error when `strings.TrimSpace(m.SolutionType) == ""`.
- Accept at least: `customer-support`, `data-inquiry`, `alert-escalation`, `approval-flow`.

- [ ] **Step 3: Validate `apiVersion`**

MVP policy:

- Accept `solution.codex/v1`.
- If examples still need legacy support, allow `solution.ai/v1alpha1` only with a compatibility warning is not currently supported by validator type. Prefer migrating examples/templates to `solution.codex/v1`.
- Reject any other non-empty `apiVersion` with `UNSUPPORTED_API_VERSION`.

- [ ] **Step 4: Update examples and templates**

Add near top:

```yaml
apiVersion: solution.codex/v1
kind: Solution
solutionType: customer-support
```

Use matching solution types:

- after-sales support: `customer-support`
- guoran support: `customer-support`
- `templates/customer-support.yaml`: `customer-support`
- `templates/data-inquiry.yaml`: `data-inquiry`
- `templates/alert-escalation.yaml`: `alert-escalation`

- [ ] **Step 5: Verify manifest and command tests**

Run:

```powershell
& '.\.tools\go1.21.13\go\bin\go.exe' test ./internal/manifest ./cmd/solution -count=1
```

Expected:

```text
ok  	fde-support/internal/manifest
ok  	fde-support/cmd/solution
```

---

## Task 8: Add Missing Built-In Evaluation Metrics

**Files:**

- Modify: `internal/evaluation/types.go`
- Modify: `internal/evaluation/metrics.go`
- Modify: `internal/evaluation/metrics_test.go`

- [ ] **Step 1: Register metrics**

In `NewMetricRegistry`, add:

```go
"result_accuracy":       evalResultAccuracy,
"escalation_precision":  evalEscalationPrecision,
```

- [ ] **Step 2: Implement `result_accuracy`**

Minimal MVP behavior:

- If expected has `AnswerContains`, pass when all expected fragments appear in `ActualAnswer`.
- If there are no expected fragments, metric is not applicable and returns `(0, false)`.

- [ ] **Step 3: Implement `escalation_precision`**

Minimal MVP behavior:

- For cases with expected intent `human_handoff` or `complaint`, pass when actual intent is `human_handoff` or response actions include a handoff-like node/status.
- Otherwise not applicable.

- [ ] **Step 4: Add tests**

Add table tests proving:

- `result_accuracy` returns 1 when all expected fragments appear.
- `result_accuracy` returns 0 when a fragment is missing.
- `escalation_precision` returns 1 for expected handoff and actual handoff.
- `escalation_precision` returns 0 for expected handoff and no handoff.

- [ ] **Step 5: Verify evaluation tests**

Run:

```powershell
& '.\.tools\go1.21.13\go\bin\go.exe' test ./internal/evaluation -count=1
```

Expected:

```text
ok  	fde-support/internal/evaluation
```

---

## Final Verification

After all tasks:

- [ ] **Run full tests**

```powershell
& '.\.tools\go1.21.13\go\bin\go.exe' test ./cmd/... ./internal/... -count=1
```

Expected:

```text
all cmd/internal tests pass
```

- [ ] **Run component publish smoke**

```powershell
$tmp = Join-Path $env:TEMP ('fde-component-' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Force -Path $tmp | Out-Null
Set-Content -Path (Join-Path $tmp 'component.yaml') -Value "ref: registry.test.echo@1.0.0`ncategory: processor`nfactory: echo`n"
& '.\.tools\go1.21.13\go\bin\go.exe' run ./cmd/solution component-publish $tmp
Remove-Item -LiteralPath $tmp -Recurse -Force
```

Expected:

```text
component artifact generated at ...
```

- [ ] **Run release artifact smoke**

Use a copied example directory and required env vars. Confirm generated `deploy/<env>` contains:

```text
docker-compose.yaml
.env.example
README.md
manifest.yaml
data/
```

---

## Suggested Prompt For The Next AI

```text
你是接手修复的资深 Go 工程师。请在 D:\work\ai\fde-support 仓库中，严格按照 docs/superpowers/plans/2026-06-30-design-review-remediation.md 执行修复。

要求：
1. 先阅读 docs/reviews/2026-06-30-design-implementation-review.md 和详细设计文档 docs/solution-as-code-fde-platform-design.md。
2. 必须按计划中的 Task 顺序推进，使用 TDD：先写失败测试，再做最小实现，再运行对应测试。
3. 每完成一个 Task，更新计划文档中的 checkbox，并运行该 Task 指定的验证命令。
4. 不要重构无关代码，不要修改第三方库或生成文件，不要覆盖用户未提交改动。
5. 使用项目本地 Go：.\.tools\go1.21.13\go\bin\go.exe。
6. 最终必须运行：
   & '.\.tools\go1.21.13\go\bin\go.exe' test ./cmd/... ./internal/... -count=1
7. 最终回复需用中文说明：完成了哪些 Task、修改了哪些文件、验证命令和结果、仍遗留哪些设计差距。

优先修复 P0：component-publish、release 自包含部署包、真实 model gateway、release 只读知识质量门禁、Logger 脱敏、模型凭据检查。
```

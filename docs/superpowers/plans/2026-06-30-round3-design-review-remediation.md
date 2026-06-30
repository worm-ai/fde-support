# Round 3 Design Review Remediation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 修复第 3 轮设计审查发现的可交付性、安全门禁和 release 可靠性问题，使受版本控制源码可从干净 checkout 构建，并关闭路径逃逸与发布门禁绕过。

**Architecture:** 先修工程交付闭环，再修安全边界，然后修 release/eval 门禁语义。所有修复采用 TDD：先写能复现问题的失败测试，再做最小实现，最后运行 targeted test 和全量 `go test`。

**Tech Stack:** Go 1.21, Cobra CLI, YAML manifests, file-based release reports, Docker Compose artifacts, OpenAI-compatible model gateway.

---

## Context For Workers

先阅读：

- `docs/reviews/2026-06-30-round3-design-implementation-review.md`
- `docs/solution-as-code-fde-platform-design.md`
- `docs/solution-as-code-fde-platform-technical-architecture.md`

本轮重点不是重新实现大功能，而是修复第 3 轮发现的交付和安全缺陷。

本地 Go：

```powershell
& '.\.tools\go1.21.13\go\bin\go.exe' version
& '.\.tools\go1.21.13\go\bin\go.exe' test ./cmd/... ./internal/... -count=1
```

要求：

- 不覆盖已有 review 文档，新增文档要带 round 或时间戳。
- 不修改第三方库和生成文件。
- 不丢弃用户未提交改动。
- 修复完成时，`git ls-files --others --exclude-standard` 不能包含被生产代码依赖的 Go 文件。

## Target File Map

| Area | Files |
|---|---|
| tracked-only 构建 | `internal/app/model_gateway.go`, `internal/app/model_gateway_test.go`, `internal/app/app.go` |
| 路径安全 | `internal/manifest/validator.go`, `internal/manifest/validator_test.go`, `internal/delivery/docker_compose.go`, `internal/delivery/docker_compose_test.go` |
| releaseChecks 强制门禁 | `internal/release/checker.go`, `internal/release/checker_test.go`, `internal/manifest/validator.go` |
| eval gate 缺失指标 | `internal/evaluation/runner.go`, `internal/evaluation/runner_test.go`, `internal/release/checker_test.go` |
| release image 策略 | `internal/delivery/docker_compose.go`, `internal/delivery/docker_compose_test.go` |

---

## Task 1: Make Tracked Source Buildable

**Files:**

- Add to git: `internal/app/model_gateway.go`
- Add to git: `internal/app/model_gateway_test.go`
- Verify: tracked-only build command

- [x] **Step 1: Confirm untracked dependency**

Run:

```powershell
git ls-files --others --exclude-standard internal/app/model_gateway.go internal/app/model_gateway_test.go
```

Expected before fix:

```text
internal/app/model_gateway.go
internal/app/model_gateway_test.go
```

- [x] **Step 2: Keep model gateway helper as production source**

Ensure `internal/app/model_gateway.go` contains:

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

- [x] **Step 3: Ensure model gateway tests are tracked**

`internal/app/model_gateway_test.go` should cover:

```go
package app

import (
	"strings"
	"testing"

	"fde-support/internal/environment"
)

func TestBuildModelGatewayRequiresDefaultModel(t *testing.T) {
	_, err := buildModelGateway(environment.ResolvedEnvironment{}, false)
	if err == nil || !strings.Contains(err.Error(), "defaultModel") {
		t.Fatalf("buildModelGateway() error = %v, want default model error", err)
	}
}

func TestBuildModelGatewayRequiresModelKey(t *testing.T) {
	_, err := buildModelGateway(environment.ResolvedEnvironment{DefaultModel: "gpt-4.1"}, false)
	if err == nil || !strings.Contains(err.Error(), "model key") {
		t.Fatalf("buildModelGateway() error = %v, want model key error", err)
	}
}

func TestBuildModelGatewayAllowsMockForTests(t *testing.T) {
	gateway, err := buildModelGateway(environment.ResolvedEnvironment{}, true)
	if err != nil {
		t.Fatalf("buildModelGateway() error = %v", err)
	}
	if gateway == nil {
		t.Fatalf("buildModelGateway() gateway = nil")
	}
}
```

- [x] **Step 4: Verify normal build**

Run:

```powershell
& '.\.tools\go1.21.13\go\bin\go.exe' test ./internal/app ./cmd/solution -count=1
```

Expected:

```text
ok  	fde-support/internal/app
ok  	fde-support/cmd/solution
```

- [x] **Step 5: Verify tracked-only build**

Run this PowerShell check from repo root:

```powershell
$tmp = Join-Path $env:TEMP ('fde-tracked-only-' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Force -Path $tmp | Out-Null
$files = git ls-files
foreach ($f in $files) {
  $src = Join-Path (Get-Location) $f
  $dst = Join-Path $tmp $f
  New-Item -ItemType Directory -Force -Path (Split-Path $dst -Parent) | Out-Null
  Copy-Item -LiteralPath $src -Destination $dst -Force
}
Push-Location $tmp
& 'D:\work\ai\fde-support\.tools\go1.21.13\go\bin\go.exe' test ./cmd/... ./internal/... -count=1
$code = $LASTEXITCODE
Pop-Location
Remove-Item -LiteralPath $tmp -Recurse -Force
exit $code
```

Expected:

```text
all cmd/internal tests pass
```

---

## Task 2: Block Knowledge Source Path Escape

**Files:**

- Modify: `internal/manifest/validator.go`
- Modify: `internal/manifest/validator_test.go`
- Modify: `internal/delivery/docker_compose.go`
- Modify: `internal/delivery/docker_compose_test.go`

- [x] **Step 1: Add validator test for escaping knowledge URI**

Add to `internal/manifest/validator_test.go`:

```go
func TestValidatorRejectsKnowledgeSourcePathEscape(t *testing.T) {
	m := validManifest()
	m.Knowledge.Sources[0].URI = "../secret.txt"
	errs := NewValidator(registry.NewBuiltinComponentRegistry(), w2a.NewBuiltinSensorRegistry()).Validate(m)
	if !hasCode(errs, "INVALID_KNOWLEDGE_SOURCE_URI") {
		t.Fatalf("expected INVALID_KNOWLEDGE_SOURCE_URI, got %#v", errs)
	}
}

func TestValidatorRejectsAbsoluteKnowledgeSourcePath(t *testing.T) {
	m := validManifest()
	m.Knowledge.Sources[0].URI = `C:\secret.txt`
	errs := NewValidator(registry.NewBuiltinComponentRegistry(), w2a.NewBuiltinSensorRegistry()).Validate(m)
	if !hasCode(errs, "INVALID_KNOWLEDGE_SOURCE_URI") {
		t.Fatalf("expected INVALID_KNOWLEDGE_SOURCE_URI, got %#v", errs)
	}
}
```

If helper names differ, adapt to existing `validator_test.go` helpers.

- [x] **Step 2: Implement path validation**

In `internal/manifest/validator.go`, add helper:

```go
func validateRelativeManifestPath(uri string, path string, add func(string, string, string)) {
	if strings.TrimSpace(uri) == "" {
		return
	}
	if filepath.IsAbs(uri) {
		add("INVALID_KNOWLEDGE_SOURCE_URI", path, "knowledge source uri must be relative to the manifest directory")
		return
	}
	clean := filepath.Clean(uri)
	if clean == "." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || clean == ".." {
		add("INVALID_KNOWLEDGE_SOURCE_URI", path, "knowledge source uri must not escape the manifest directory")
	}
}
```

Call it for every `knowledge.sources[i].uri`.

Update imports to include `path/filepath`.

- [x] **Step 3: Add delivery defense test**

Add to `internal/delivery/docker_compose_test.go`:

```go
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
```

- [x] **Step 4: Implement delivery source/destination containment**

In `internal/delivery/docker_compose.go`:

- Before copying, compute absolute clean source and base paths.
- Reject if source is outside `m.BaseDir`.
- Compute destination under `outputDir` using only safe relative paths.
- Reject if destination is outside `outputDir`.

Implementation shape:

```go
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
	return rel == "." || (!strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != "..")
}
```

- [x] **Step 5: Verify targeted tests**

Run:

```powershell
& '.\.tools\go1.21.13\go\bin\go.exe' test ./internal/manifest ./internal/delivery -count=1
```

Expected:

```text
ok  	fde-support/internal/manifest
ok  	fde-support/internal/delivery
```

---

## Task 3: Prevent `releaseChecks` From Skipping Mandatory Gates

**Files:**

- Modify: `internal/release/checker.go`
- Modify: `internal/release/checker_test.go`
- Optional modify: `internal/manifest/validator.go`

- [x] **Step 1: Add release checker test**

Add to `internal/release/checker_test.go`:

```go
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
}
```

Add helper if missing:

```go
func hasCheck(report *ReleaseReport, name string) bool {
	for _, check := range report.Checks {
		if check.Name == name {
			return true
		}
	}
	return false
}
```

- [x] **Step 2: Define mandatory checks**

In `internal/release/checker.go`:

```go
var mandatoryReleaseChecks = map[string]bool{
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

For MVP, all eight are mandatory. A non-empty `delivery.releaseChecks` may add ordering preference later, but must not skip mandatory checks.

- [x] **Step 3: Update `Checker.Run` filtering**

Replace skip logic:

```go
if len(declared) > 0 && !declared[check.name] {
	continue
}
```

with logic that always runs mandatory checks:

```go
if len(declared) > 0 && !declared[check.name] && !mandatoryReleaseChecks[check.name] {
	continue
}
```

Because all current checks are mandatory, current checker should run all checks.

- [x] **Step 4: Verify release tests**

Run:

```powershell
& '.\.tools\go1.21.13\go\bin\go.exe' test ./internal/release -count=1
```

Expected:

```text
ok  	fde-support/internal/release
```

---

## Task 4: Fail On Missing OnRelease Block Metric

**Files:**

- Modify: `internal/evaluation/runner.go`
- Modify: `internal/evaluation/runner_test.go`
- Modify: `internal/release/checker_test.go`

- [x] **Step 1: Add evaluation runner test**

Add to `internal/evaluation/runner_test.go`:

```go
func TestRunnerCreatesFailedGateResultWhenBlockMetricMissing(t *testing.T) {
	dataset := writeGoldenCases(t, []string{`{"id":"c1","trigger":{"type":"chat"},"request":{"message":"hi"},"expected":{"intent":"support"}}`})
	exec := fakeExecutor{response: map[string]any{"intent": "support", "answer": "ok"}}
	m := &manifest.SolutionManifest{
		Metadata: manifest.MetadataSpec{Name: "s", Version: "1"},
		Evaluation: manifest.EvaluationSpec{
			Metrics: []string{"result_accuracy"},
			Gates: []manifest.EvaluationGateSpec{{
				Metric: "result_accuracy", Min: 0.9, Severity: "block", Schedule: "onRelease",
			}},
		},
	}
	report, err := NewRunner(exec, m, NewMetricRegistry()).Run(context.Background(), dataset, m.Evaluation.Gates)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(report.GateResults) != 1 {
		t.Fatalf("GateResults len = %d, want 1", len(report.GateResults))
	}
	if report.GateResults[0].Passed {
		t.Fatalf("missing block metric must fail gate")
	}
}
```

Adapt fake executor/helper names to existing `runner_test.go`.

- [x] **Step 2: Update gate loop**

In `internal/evaluation/runner.go`, replace:

```go
actual, ok := report.Metrics[gate.Metric]
if !ok {
	continue
}
```

with:

```go
actual, ok := report.Metrics[gate.Metric]
if !ok {
	report.GateResults = append(report.GateResults, GateResult{
		Metric: gate.Metric, Min: gate.Min, Actual: 0,
		Severity: gate.Severity, Schedule: gate.Schedule, Passed: false,
	})
	if gate.Severity == "block" {
		report.Warnings = append(report.Warnings, fmt.Sprintf("gate %s failed: metric not found", gate.Metric))
	}
	continue
}
```

- [x] **Step 3: Verify evaluation and release tests**

Run:

```powershell
& '.\.tools\go1.21.13\go\bin\go.exe' test ./internal/evaluation ./internal/release -count=1
```

Expected:

```text
ok  	fde-support/internal/evaluation
ok  	fde-support/internal/release
```

---

## Task 5: Make Release Runtime Image Strategy Explicit

**Files:**

- Modify: `internal/delivery/docker_compose.go`
- Modify: `internal/delivery/docker_compose_test.go`

- [x] **Step 1: Add compose/readme consistency test**

Add to `internal/delivery/docker_compose_test.go`:

```go
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
```

- [x] **Step 2: Update README generation**

In `generateReadme`, add a Runtime Image section:

```go
"## Runtime Image",
"",
fmt.Sprintf("This deployment expects the Docker image `solution-runtime:%s` to be available on the target host or registry before running `docker-compose up -d`.", m.Metadata.Version),
"Build or publish that image from the platform runtime before using this deployment package.",
"",
```

- [x] **Step 3: Verify delivery tests**

Run:

```powershell
& '.\.tools\go1.21.13\go\bin\go.exe' test ./internal/delivery -count=1
```

Expected:

```text
ok  	fde-support/internal/delivery
```

---

## Final Verification

- [x] **Run normal full test**

```powershell
& '.\.tools\go1.21.13\go\bin\go.exe' test ./cmd/... ./internal/... -count=1
```

Expected:

```text
all cmd/internal tests pass
```

- [x] **Run tracked-only full test**

Use the tracked-only script from Task 1 Step 5.

Expected:

```text
all cmd/internal tests pass
```

- [x] **Run path escape smoke**

Create a temporary manifest with `knowledge.sources[].uri: ../secret.txt`.

Expected:

```text
solution validate fails with INVALID_KNOWLEDGE_SOURCE_URI
```

- [x] **Run release smoke**

```powershell
$tmp = Join-Path $env:TEMP ('fde-release-final-' + [guid]::NewGuid().ToString('N'))
Copy-Item -Path 'examples\after-sales-support' -Destination $tmp -Recurse
$env:OPENAI_API_KEY='test-key'
$env:TICKET_WEBHOOK_TOKEN='test-token'
$env:TICKET_API_KEY='test-ticket-key'
& '.\.tools\go1.21.13\go\bin\go.exe' run ./cmd/solution ingest "$tmp\manifest.yaml"
& '.\.tools\go1.21.13\go\bin\go.exe' run ./cmd/solution release "$tmp\manifest.yaml" --env poc
Remove-Item Env:OPENAI_API_KEY -ErrorAction SilentlyContinue
Remove-Item Env:TICKET_WEBHOOK_TOKEN -ErrorAction SilentlyContinue
Remove-Item Env:TICKET_API_KEY -ErrorAction SilentlyContinue
Remove-Item -LiteralPath $tmp -Recurse -Force
```

Expected:

```text
release poc: passed=true
deploy/poc contains manifest.yaml, data/, docker-compose.yaml, .env.example, README.md
```

---

## Suggested Prompt For The Next AI

```text
你是接手第3轮修复的资深 Go 工程师。请在 D:\work\ai\fde-support 仓库中，严格按照 docs/superpowers/plans/2026-06-30-round3-design-review-remediation.md 执行修复。

必须先阅读：
1. docs/reviews/2026-06-30-round3-design-implementation-review.md
2. docs/solution-as-code-fde-platform-design.md
3. docs/solution-as-code-fde-platform-technical-architecture.md

执行要求：
1. 按计划 Task 1 到 Task 5 顺序执行，不跳步。
2. 使用 TDD：先写失败测试，再做最小实现，再运行该 Task 指定验证命令。
3. 不覆盖已有过程文档；如需新增过程文档，文件名带 round 或时间戳，并复制到 docs/reviews/archive/。
4. 不回滚用户已有改动，不修改第三方库或生成文件。
5. 使用本地 Go：.\.tools\go1.21.13\go\bin\go.exe。
6. 修复完成后必须同时通过：
   - & '.\.tools\go1.21.13\go\bin\go.exe' test ./cmd/... ./internal/... -count=1
   - 计划 Task 1 中的 tracked-only 测试脚本
7. 最终中文回复说明：完成的 Task、修改文件、验证命令与结果、是否仍有遗留设计差距。

本轮 P0 是：
1. 让 git tracked 源码从干净 checkout 可构建。
2. 阻止 knowledge source 路径逃逸导致发布包复制方案目录外文件。
3. 防止 releaseChecks 配置跳过知识质量和评测强制门禁。
```

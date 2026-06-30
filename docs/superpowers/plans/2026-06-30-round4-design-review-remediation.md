# Round 4 Design Implementation Review — 开发修复计划

基于 [2026-06-30 第 4 轮设计实现审查](/Users/cc/ai/fde-support/docs/reviews/2026-06-30-round4-design-implementation-review.md) 报告。

---

## 修复清单

### P0-1：修复 Phase 2 processor 组件输出中的 `status` 字段（BUG-R4-002）

**目标**：从所有 processor 组件输出中移除 `status` 字段，使其符合设计契约：processor 只返回纯业务数据，失败通过 `(nil, error)` 传递。

**文件**：`internal/registry/components.go`

**具体修改**：

1. **`llmExtractor.Run()`**（约第 270-295 行）：
   - 成功时：移除 `"status"` 键，只返回 `{"extracted": resp.Content}`
   - model gateway 不可用时：改为返回 `(nil, error)` 而非 `{"status": "failed", "error": {...}}`

2. **`dataQuery.Run()`**（约第 300-330 行）：
   - 移除 `"status"` 键，只返回 `{"rows": result.Raw, "count": len(result.Raw), "citations": citations}`

3. **`ruleEvaluator.Run()`**（约第 395-425 行）：
   - 成功匹配时移除 `"status"` 键，返回 `{"matched": true, "rule": ..., "result": ..., "matches": ...}`
   - 无匹配时移除 `"status"` 键，返回 `{"matched": false}`

4. **`executeNodeWithRetry` 增加 post-condition 检查**（`internal/runtimecore/executor.go`）：
   - 在 `component.Run()` 返回 `(output, nil)` 后，若组件为 processor 且 output 包含 `"status"` 键，写入 DEBUG 日志（不作为 hard failure）

**验收标准**：
- `llmExtractor`、`dataQuery`、`ruleEvaluator` 返回的 output 中无 `"status"` 键
- `llmExtractor` 在 model 不可用时返回 `(nil, error)`
- 现有测试通过（Phase 1 组件不变）

---

### P0-2：完善知识源路径校验 cross-platform + delivery 二级防御（BUG-R4-001）

**目标**：确保知识源路径在任何平台上都不会逃逸 Manifest 目录或输出目录。

**文件**：
- `internal/manifest/validator.go` → `validateRelativeManifestPath`
- `internal/delivery/docker_compose.go` → `copyRuntimeInputs`

**具体修改**：

1. **`validateRelativeManifestPath` 改为平台无关校验**：
   ```go
   func validateRelativeManifestPath(uri string, path string, add func(string, string, string)) {
       if strings.TrimSpace(uri) == "" {
           return
       }
       // 平台无关的绝对路径检测
       if strings.HasPrefix(uri, "/") || filepath.IsAbs(uri) {
           add("INVALID_KNOWLEDGE_SOURCE_URI", path, "absolute paths are not allowed")
           return
       }
       // 检测 Windows drive letter: C:\
       if len(uri) >= 2 && uri[1] == ':' && (uri[0] >= 'A' && uri[0] <= 'Z' || uri[0] >= 'a' && uri[0] <= 'z') {
           add("INVALID_KNOWLEDGE_SOURCE_URI", path, "absolute paths are not allowed")
           return
       }
       // 检测 .. 路径逃逸
       clean := filepath.Clean(uri)
       if strings.HasPrefix(clean, "..") || clean == ".." {
           add("INVALID_KNOWLEDGE_SOURCE_URI", path, "path must not escape manifest directory")
           return
       }
   }
   ```

2. **`copyRuntimeInputs` 增加防御性检查**：
   - 复制前检查 `filepath.HasPrefix(resolvedSrc, m.BaseDir)`
   - 复制前检查 `filepath.HasPrefix(resolvedDst, outputDir)`
   - 不满足任一项时返回错误

3. **`TestValidatorRejectsAbsoluteKnowledgeSourcePath` 改为跨平台测试**：
   - Unix 路径：`/etc/passwd`
   - Windows 路径：`C:\secret.txt`
   - 逃逸路径：`../secret.txt`（已有测试）

**验收标准**：
- `go test ./internal/manifest/... -count=1` 全部通过（包括 macOS/Linux）
- 新增测试覆盖 `/etc/passwd`、`C:\secret.txt`、`..\secret.txt`
- release 测试覆盖路径逃逸场景

---

### P1-1：修复 `knowledgeBindings` 运行时过滤（HIGH-R4-001）

**目标**：使 `runtime.knowledgeBindings` 在运行时生效，不同 retriever 组件只能看到绑定的知识源子集。

**文件**：`internal/runtimecore/executor.go`

**具体修改**：

1. 在 `executeNodeWithRetry` 或 `NewExecutor` 中，根据 `e.manifest.Runtime.KnowledgeBindings` 为每个组件构造 scoped `KnowledgeReader`：
   ```go
   func (e *Executor) scopedKnowledge(componentID string) registry.KnowledgeReader {
       for _, binding := range e.manifest.Runtime.KnowledgeBindings {
           if binding.Component == componentID {
               // 从 e.knowledge 中筛选出绑定源的知识单元
               return e.knowledge.FilterBySources(binding.Sources)
           }
       }
       return e.knowledge // 未声明绑定的组件看到全量
   }
   ```

2. 在 `knowledge.Store` 中增加 `FilterBySources(sourceIDs []string) *Store` 方法，返回仅包含指定 source 的子集 store

**验收标准**：
- 两个 retriever 分别绑定 `source_a` 和 `source_b` 时，检索结果不交叉
- 未声明 binding 的 retriever 仍看到全量
- Validator 的校验逻辑不变

---

### P1-2：evaluate 命令支持 `--env` 参数（HIGH-R4-002）

**目标**：`solution evaluate` 可对非 `poc` 环境执行评测。

**文件**：
- `cmd/solution/main.go`：添加 `--env` flag
- `internal/app/app.go` → `EvaluateManifestFile`：接受 envName 参数

**具体修改**：
1. CLI 注册：`cmd.Flags().String("env", "poc", "target environment for evaluation")`
2. `EvaluateManifestFile` 签名改为 `EvaluateManifestFile(path, envName string)`
3. 评测前解析对应环境配置

**验收标准**：
- `solution evaluate manifest.yaml --env=poc` 与原行为一致
- `solution evaluate manifest.yaml --env=production` 使用 production 环境配置执行评测

---

### P1-3：防止 releaseChecks 跳过强制门禁（HIGH-R4-003）

**目标**：确保关键质量和安全门禁不可被 Manifest 配置绕过。

**文件**：`internal/release/checker.go` → `Checker.Run`、`internal/manifest/validator.go`

**具体修改**：

方案 A（推荐）：定义强制检查最小集合，始终执行：
```go
var mandatoryReleaseChecks = []string{
    "knowledge_quality_passed",
    "eval_gates_passed",
    "observability_enabled",
    "security_baseline_passed",
}
```

方案 B：Validator 拒绝生产环境排除 P0 检查的 Manifest。

**验收标准**：
- 即使 `delivery.releaseChecks` 只声明一个检查项，强制门禁仍执行
- `solution validate` 在生产环境下拒绝缺少关键检查的配置

---

### P1-4：Release runtime image 策略（HIGH-R4-004）

**目标**：release 产出物可被用户直接启动。

**文件**：`internal/delivery/docker_compose.go`

**具体修改**（三选一，建议方案 A）：

方案 A（推荐）：生成 Dockerfile 并包含构建上下文
```yaml
services:
  solution-runtime:
    build:
      context: .
      dockerfile: Dockerfile
    command: ...
```

方案 B：compose 使用 `image:` 但在 README 中声明构建命令（`go build -o solution ./cmd/solution`）。

方案 C：release CLI 增加 `--image` 参数指定镜像名。

**验收标准**：
- README 或 Dockerfile 中有明确的可跟踪步骤说明如何获得 runtime
- 若使用 `image:`，提供构建命令或拉取指令

---

### P1-5：Eval gate 缺失 metric 处理（HIGH-R4-005）

**目标**：`severity: block` 且 `schedule: onRelease` 的 gate 在 metric 缺失时阻断，而非静默跳过。

**文件**：`internal/evaluation/runner.go` → `Runner.Run`

**具体修改**：
```go
if !ok {
    if gate.Severity == "block" && gate.Schedule == "onRelease" {
        report.GateResults = append(report.GateResults, GateResult{
            Metric: gate.Metric, Min: gate.Min, Actual: 0,
            Severity: gate.Severity, Schedule: gate.Schedule, Passed: false,
        })
        report.Warnings = append(report.Warnings, "gate failed: metric not found: "+gate.Metric)
    }
    continue
}
```

**验收标准**：
- block onRelease gate 的 metric 缺失时 release 必须失败
- warn 级别的 metric 缺失时输出告警但不阻断

---

### P1-6：evaluate/release 模型网关 mock fallback（HIGH-R4-006）

**目标**：当 workflow 不使用 `model.generate` 时，evaluate 和 release 不因模型密钥缺失而失败。

**文件**：`internal/app/app.go` → `EvaluateManifestFile`、`ReleaseManifestFile`

**具体修改**：

1. 增加判断：扫描 manifest 中是否有组件声明 `requires: [model.generate]`
2. 若无组件需要模型调用，`allowMock=true`
3. 或改为始终 `allowMock=true`，模型调用时再用真实 gateway

**验收标准**：
- 使用纯关键词检索方案的 Manifest 可通过 evaluate 和 release（不需 OPENAI_API_KEY）
- 使用 llm-classifier 或 llm-generator 的方案仍需模型密钥

---

## P2 任务（Phase 2 完善）

| 任务 | 描述 | 文件 |
|---|---|---|
| P2-1 | 实现 eval cache：execution/dataset/knowledge fingerprint + TTL 复用 | `internal/evaluation/runner.go` |
| P2-2 | 实现自定义组件运行时执行（Go plugin 或 subprocess） | `internal/registry/` |
| P2-3 | Python Worker 接入：Go 调用 Python 子进程处理 Markdown/PDF/Word | `internal/knowledge/python_bridge.go` |
| P2-4 | PostgreSQL schema 实现 + knowledge unit 持久化 | `internal/knowledge/` |
| P2-5 | W2A 幂等迁移到 Redis/PostgreSQL（共享持久化存储） | `internal/w2a/idempotency.go` |
| P2-6 | 实现 `solution destroy` 命令 | `cmd/solution/` |
| P2-7 | 组件复用统计（`ComputeReuseStats`） | `internal/registry/marketplace.go` |

---

## 执行顺序建议

建议先执行 P0 任务（核心契约违规和路径安全），再执行 P1 任务（功能完善），最后执行 P2 任务（Phase 2 能力扩展）。

```
P0-1 (processor status) ──→ P0-2 (path security)
                                    ↓
P1-1 (knowledge binding) ←── P1-2 (eval --env)
P1-3 (releaseChecks gate) ←── P1-5 (eval missing metric)
P1-4 (runtime image)      ←── P1-6 (mock fallback)
                                    ↓
                              P2 tasks...
```

## 测试要求

每个 P0/P1 任务完成后必须：
1. 新增或修改对应单元测试
2. `go test ./cmd/... ./internal/... -count=1` 全部通过
3. 涉及的集成测试（validate、release smoke）通过

## 提示词

将本文档（修复计划）交给 AI 时，使用以下提示词：

```
你是资深 Go 工程师。请严格按照以下修复计划逐任务执行修复，每个任务修复完成后运行对应测试确认通过，再继续下一个任务。修复计划文件：[docs/superpowers/plans/2026-06-30-round4-design-review-remediation.md](/Users/cc/ai/fde-support/docs/superpowers/plans/2026-06-30-round4-design-review-remediation.md)

执行规则：
1. 先执行 P0 任务（P0-1 → P0-2），再执行 P1 任务（P1-1 → P1-6）
2. 每个任务修复完成后必须运行 `go test ./... -count=1` 确保无回归
3. 不要在同一次修复中修改超过 2 个不相关的文件
4. 遇到不确定的点先查看设计文档 [docs/solution-as-code-fde-platform-design.md](/Users/cc/ai/fde-support/docs/solution-as-code-fde-platform-design.md)
5. 修复完成后向用户报告修复结果和测试状态
```

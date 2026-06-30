# Round 6 修复计划

> 基于审查报告：`docs/reviews/archive/round6-2026-07-01/2026-07-01-round6-design-implementation-review.md`
> 计划日期：2026-07-01
> 目标：修复审查中发现的3个致命Bug和5个高级严重问题，使工程达到可交付状态

---

## 修复优先级总览

| 优先级 | 编号 | 问题 | 严重程度 | 预计工时 |
|--------|------|------|---------|---------|
| P0 | FIX-01 | Web 前端 Manifest 格式完全错误 | FATAL | 3h |
| P0 | FIX-02 | Processor 组件 OutputSchema 违反契约 | FATAL | 1h |
| P0 | FIX-03 | Release 门禁过于严苛 | HIGH | 1.5h |
| P1 | FIX-04 | 知识质量门禁在 run 时未执行 | HIGH | 2h |
| P1 | FIX-05 | `scopedKnowledge` 类型断言静默失败 | HIGH | 0.5h |
| P1 | FIX-06 | Web 前端缺少 `components` 段 | FATAL | 2h |
| P2 | FIX-07 | Mock Gateway 静默使用无警告 | MEDIUM | 0.5h |
| P2 | FIX-08 | Trace 列表可能泄漏敏感信息 | MEDIUM | 0.5h |
| P3 | FIX-09 | `ComputeReuseStats` 为空实现 | LOW | 1h |
| P3 | FIX-10 | 知识索引无原子替换 | LOW | 2h |

---

## 详细修复方案

### FIX-01：Web 前端 Manifest 格式修正 [P0, FATAL]

**文件**：`/Users/cc/ai/fde-support/web/app.js`

**当前状态**：
```js
function buildManifest(values) {
  return `apiVersion: solution.ai/v1alpha1   // ← 错误
kind: Solution                              // ← 缺少 solutionType
...
perception:
  triggers:
    - routeTo: support_agent                // ← 错误，应为 classify_intent
...
workflow:
  nodes:                                    // ← 缺少 components 段
    - component: intent_classifier          // ← 应为 ref 格式
```

**修复目标**：使 Web 控制台生成的 Manifest 格式与后端 Validator 完全兼容。

**修改点**：

1. **修正 `apiVersion`**（第82行附近）：
   ```
   - apiVersion: solution.ai/v1alpha1
   + apiVersion: solution.codex/v1
   ```

2. **增加 `solutionType`**（紧接 `kind` 之后）：
   ```
   + solutionType: customer-support
   ```

3. **增加完整的 `components` 段**：
   为3个组件生成正确的 `components` 声明，字段与示例 Manifest 对齐。

4. **修正 `routeTo`**：
   ```
   - routeTo: support_agent
   + routeTo: classify_intent
   ```

5. **增加 `knowledge.schemas`**：
   ```yaml
   schemas:
     - id: faq
       fields:
         - question
         - answer
         - product_model
         - source_ref
   ```

6. **增加 `runtime.modelPolicy` 和 `runtime.observability`** 默认值

7. **修正 W2A `authTokenRef`**：当前 YAML 中写的是 `authTokenRef: env:TICKET_WEBHOOK_TOKEN`，YAML 模板中不应硬编码 token 变量名

**验证方式**：
- 修改后打开 Web 控制台，选择"售后助手"模板，复制生成的 Manifest
- 执行 `go run ./cmd/solution validate <generated-manifest.yaml>` 确认通过
- 执行 `go run ./cmd/solution run <generated-manifest.yaml> --env=poc` 确认可启动

---

### FIX-02：Processor 组件 OutputSchema 修复 [P0, FATAL]

**文件**：`/Users/cc/ai/fde-support/internal/registry/types.go`

**当前状态**：3个 Processor 组件的 `OutputSchema` 均包含 `"status": "string"`

**修复目标**：从 Processor 组件的 OutputSchema 中移除 `status` 字段。

**修改点**：

1. `registry.processor.llm-extractor@1.0.0`：
   ```
   - OutputSchema: map[string]string{"status": "string", "extracted": "string?"}
   + OutputSchema: map[string]string{"extracted": "string?"}
   ```

2. `registry.processor.data-query@1.0.0`：
   ```
   - OutputSchema: map[string]string{"status": "string", "rows": "array", "count": "number", "citations": "array"}
   + OutputSchema: map[string]string{"rows": "array", "count": "number", "citations": "array"}
   ```

3. `registry.processor.rule-evaluator@1.0.0`：
   ```
   - OutputSchema: map[string]string{"status": "string", "matched": "boolean", "result": "string?"}
   + OutputSchema: map[string]string{"matched": "boolean", "result": "string?"}
   ```

4. 同步修改组件实现 `components.go` 中的 `dataQuery.Run()` 和 `ruleEvaluator.Run()`，移除返回的 `"status"` 字段，或将其从输出 map 中删除（因为它们是 Processor）。

**验证方式**：
- `go test ./internal/registry/... -v`
- `go test ./internal/manifest/... -v`

---

### FIX-03：Release 门禁严苛度调整 [P0, HIGH]

**文件**：`/Users/cc/ai/fde-support/internal/release/checker.go`

**修复目标**：确保未实现的检查不阻断发布流程。

**修改点**：

1. `checkObservability()`：
   ```go
   // 当前：trace != "required" 则 block
   // 修改：trace 为空时仅 warn，不为空但非 "required" 时才 block
   if c.manifest.Runtime.Observability.Trace == "" {
       return CheckResult{Name: "observability_enabled", Passed: true, Severity: "warn",
           Message: "observability trace is not configured; tracing may be disabled"}
   }
   ```

2. `checkSecurityBaseline()`：
   ```go
   // 当前：piiDetection != "required" 或 promptInjectionDefense != "required" 则 block
   // 修改：降级为 warn，因为这些能力在 Phase 4 才实现
   ```
   将 `Severity` 从 `"block"` 改为 `"warn"`，并更新 Message 指明这是 Phase 4 能力。

3. `mandatoryReleaseChecks` 策略调整：
   - 保留 `knowledge_quality_passed` 和 `eval_gates_passed` 为强制
   - `observability_enabled` 和 `security_baseline_passed` 改为仅当对应配置存在时才检查，缺失时报 warn 不 block

**验证方式**：
- `go test ./internal/release/... -v`
- 使用一个未配置 `piiDetection` 的 Manifest 执行 `solution release` 确认不会 block

---

### FIX-04：知识质量门禁在 Run 时执行 [P1, HIGH]

**文件**：`/Users/cc/ai/fde-support/internal/knowledge/loader.go`

**修复目标**：`LoadWithOptions()` 加载知识后执行 Manifest 中声明的质量门禁。

**修改点**：

1. 在 `LoadWithOptions()` 结束前，对已加载的 `store.units` 执行 `runQualityGates`（从 `ingest.go` 复用）：
   ```go
   // After all sources are loaded
   runQualityGatesForStore(store, m.Knowledge.QualityGates, &report)
   ```

2. 在 `ingest.go` 中将 `runQualityGates` 函数改为接受 `*Store` 参数（或抽取通用逻辑）：
   - 当前 `runQualityGates` 接受 `[]Unit`，而 `loader.go` 有 `*Store`
   - 添加桥接函数 `runQualityGatesForStore`

**验证方式**：
- 修改示例 Manifest 的 qualityGate 为 `stale_content` + `maxAgeDays: 365`
- `solution run manifest.yaml` 应在启动时执行门禁并记录到质量报告
- `go test ./internal/knowledge/... -v`

---

### FIX-05：`scopedKnowledge` 类型断言修复 [P1, HIGH]

**文件**：`/Users/cc/ai/fde-support/internal/runtimecore/executor.go`

**修复目标**：使用接口而非 `any` 类型断言，避免静默失败。

**修改点**：

1. 在 `registry/types.go` 中增加接口：
   ```go
   type KnowledgeStoreFilter interface {
       FilterBySources(sourceIDs []string) KnowledgeReader
   }
   ```

2. 让 `knowledge.Store` 实现该接口（已实现 `FilterBySources`）。

3. 修改 `Executor` 结构体和构造函数：
   ```go
   type Executor struct {
       ...
       knowledge      registry.KnowledgeReader
       knowledgeStore registry.KnowledgeStoreFilter  // 新字段
   }
   ```
   在 `NewExecutor` 中：
   ```go
   if ks, ok := knowledge.(registry.KnowledgeStoreFilter); ok {
       ex.knowledgeStore = ks
   }
   ```

4. 修改 `scopedKnowledge`：
   ```go
   func (e *Executor) scopedKnowledge(componentID string) registry.KnowledgeReader {
       if e.knowledgeStore == nil { return e.knowledge }
       for _, binding := range e.manifest.Runtime.KnowledgeBindings {
           if binding.Component == componentID {
               return e.knowledgeStore.FilterBySources(binding.Sources)
           }
       }
       return e.knowledge
   }
   ```

**验证方式**：
- `go test ./internal/runtimecore/... -v`
- 确认已有测试（`executor_test.go`）仍然通过

---

### FIX-06：Web 前端增加完整 Components 段 [P1, FATAL]

**文件**：`/Users/cc/ai/fde-support/web/app.js`

**修复目标**：在 `buildManifest()` 中生成正确的 `components` YAML 段。

**修改点**：

在生成的 Manifest 中加入完整的 components 列表：

```yaml
components:
  - id: intent_classifier
    category: processor
    ref: registry.intent.beverage-router@1.0.0
    config:
      intents:
        - troubleshooting
        - warranty
        - complaint
        - human_handoff
  - id: retriever
    category: processor
    ref: registry.retriever.local-keyword@1.0.0
    config:
      topK: 5
      requireCitation: true
  - id: answer_generator
    category: processor
    ref: registry.agent.cited-answer@1.2.0
    config:
      style: concise
      requireGrounding: true
```

**验证方式**：
- `go run ./cmd/solution validate <web-generated-manifest.yaml>` 确认通过

---

### FIX-07：Mock Gateway 使用警告 [P2, MEDIUM]

**文件**：`/Users/cc/ai/fde-support/internal/app/model_gateway.go`

**修改点**：

在 `buildModelGateway` 返回 Mock Gateway 前，输出 stderr 警告：
```go
if allowMock {
    fmt.Fprintf(os.Stderr, "WARNING: using mock model gateway — set runtime.modelPolicy.defaultModel for real model calls\n")
    return model.NewMockGateway(), nil
}
```

---

### FIX-08：Trace 列表脱敏增强 [P2, MEDIUM]

**文件**：`/Users/cc/ai/fde-support/internal/trace/writer.go`，`List()` 方法

**修改点**：

在 `List()` 返回前对每条 Trace 的 Input 和 Spans 重新执行 `RedactMap`：
```go
record.Input = RedactMap(record.Input)
for i := range record.Spans {
    record.Spans[i].Input = RedactMap(record.Spans[i].Input)
    record.Spans[i].Output = RedactMap(record.Spans[i].Output)
}
```

---

### FIX-09：ComputeReuseStats 实现 [P3, LOW]

**文件**：`/Users/cc/ai/fde-support/internal/registry/marketplace.go`

**修改点**：

实现 `ComputeReuseStats` 函数，统计 Manifest 中引用已有组件 vs 自定义组件的比例。

---

### FIX-10：知识索引原子替换 [P3, LOW]

**文件**：`/Users/cc/ai/fde-support/internal/knowledge/loader.go`

**修改点**：

索引构建使用临时目录，完成后原子替换。已在 Trace Writer 实现中验证了 `os.Rename` 原子替换模式，可复用。

---

## 修复顺序建议

```
第1轮（P0，必须修复）：
FIX-01 → FIX-06 → FIX-02 → FIX-03
（前端 Manifest 格式 → 前端 Components 段 → Processor 契约 → Release 门禁）

第2轮（P1，强烈建议）：
FIX-04 → FIX-05
（知识质量门禁 → scopedKnowledge 类型安全）

第3轮（P2-P3，改进项）：
FIX-07 → FIX-08 → FIX-09 → FIX-10
```

## 修复后验收标准

1. `go test ./...` 全部通过
2. Web 控制台生成的 Manifest 能通过 `solution validate`
3. 示例 Manifest `examples/after-sales-support/manifest.yaml` 通过 `solution release`
4. 未配置安全基线的 Manifest 发布时仅产生 warn 不 block
5. Processor 组件 OutputSchema 不包含 `status` 字段
6. `solution run` 启动时执行知识质量门禁

# Solution-as-Code FDE 平台 设计与实现差异审查报告 (Round 6)

> 审查日期：2026-07-01
> 审查轮次：Round 6（含4轮增量审查）
> 设计基准：`docs/solution-as-code-fde-platform-design.md` (1938行)
> 审查范围：全部已提交源码（`cmd/`, `internal/`, `web/`），排除 `docs/`, `examples/`, `templates/`, `workers/`

---

## 审查摘要

| 维度 | 结果 |
|------|------|
| **整体完成度** | **~72%**（Phase 1完成度约85%，Phase 2~4整合后约60%） |
| **致命 Bug 数量** | **3** 个致命级 + **5** 个高级严重问题 |
| **测试通过率** | 所有22个测试文件全部通过（`go test ./...` all OK） |
| **代码总规模** | ~7500行 Go 源码 + ~3500行 web 前端代码 |

### 完成度计算依据

Phase 1 设计文档约有28个功能点/交付物，已完整实现约19个（68%），部分实现6个（21%），未实现3个（11%）。
Phase 2~4 能力有不同程度的实现穿插，其中 Phase 2 的通用组件库已完成大部分（6/7），但模板系统未实现；
Phase 3 评测框架完整实现，但部分细节待完善；Phase 4 交付框架基本完成。

加权计算：Phase 1 (权重0.4 × 85%) + Phase 2 (权重0.25 × 65%) + Phase 3 (权重0.2 × 75%) + Phase 4 (权重0.15 × 55%) ≈ 72%

需要特别指出的是：**Web 前端控制台生成的 Manifest 格式与后端完全不兼容**，这是最严重的功能断层。

---

## 完成度详情

### 已完整实现的功能模块

| 模块 | 设计文档章节 | 实现文件 | 完成度 |
|------|-------------|----------|--------|
| Manifest 数据结构 | 核心抽象:Solution | `internal/manifest/types.go` | 100% |
| Manifest YAML 加载器 | 核心抽象 | `internal/manifest/loader.go` | 100% |
| Manifest 必填字段校验 | 核心抽象 | `internal/manifest/validator.go` | 100% |
| `solution validate` CLI | Phase 1 | `cmd/solution/main.go` | 100% |
| `solution run` CLI | Phase 1 | `cmd/solution/main.go` | 100% |
| `POST /chat` 端点 | Phase 1 | `internal/api/server.go` | 100% |
| W2A Webhook 端点注册 | Phase 1 | `internal/api/server.go` | 100% |
| W2A Signal 规范化与校验 | Phase 1 | `internal/w2a/signal.go` | 100% |
| W2A Signal 幂等性存储 | Phase 1 | `internal/w2a/idempotency.go` | 100% |
| Signal Router（Chat/Signal 双入口） | Phase 1 | `internal/api/signal_router.go` | 100% |
| Runtime Kernel 执行器 | Phase 1 | `internal/runtimecore/executor.go` | 100% |
| RuntimeRequest 统一数据模型 | Phase 1 | `internal/runtimecore/types.go` | 100% |
| Workflow 编译器与执行计划 | Phase 1 | `internal/workflow/plan.go` | 100% |
| `when` 条件解析与求值 | Phase 1 | `internal/workflow/when.go` | 100% |
| Trace Writer（文件持久化） | Phase 1 | `internal/trace/writer.go` | 100% |
| Trace 敏感信息脱敏 | Phase 1 | `internal/trace/redactor.go` | 100% |
| 知识加载器（JSONL关键词索引） | Phase 1 | `internal/knowledge/loader.go` | 100% |
| 知识检索 `Retrieve()` | Phase 1 | `internal/knowledge/loader.go` | 100% |
| 环境解析器 | Phase 1 | `internal/environment/resolver.go` | 100% |
| 组件注册表（内置10+组件） | Phase 1/2 | `internal/registry/types.go` + `components.go` | 95% |
| RuntimeContext 接口完整实现 | Phase 1/2 | `internal/runtimecore/types.go` | 100% |
| 评测执行器 + 4个内置指标 | Phase 3 | `internal/evaluation/` 全部5个文件 | 95% |
| Release 门禁检查框架 | Phase 4 | `internal/release/checker.go` | 90% |
| Docker Compose 部署产物生成 | Phase 4 | `internal/delivery/docker_compose.go` | 90% |
| Component SDK（`Component` 接口） | 组件开发规范 | `internal/registry/types.go` | 100% |
| 自定义组件发现加载（`component.yaml`） | 组件开发规范 | `internal/registry/types.go` -> `loadFromRoot` | 90% |
| 类型流校验（上游/下游Schema） | Phase 3 | `internal/manifest/validator_typeflow.go` | 100% |
| `inputMapping` 字段路径映射 | Phase 1 | `internal/runtimecore/executor.go` -> `ApplyInputMapping` | 100% |
| `human_handoff` fallback 错误上下文 | Phase 1 | `internal/registry/components.go` -> `humanHandoff.Run` | 100% |
| `solution ingest` CLI | Phase 2 | `cmd/solution/main.go` | 100% |

### 部分实现的功能（缺失细节）

| 功能 | 设计文档位置 | 已实现 | 缺失内容 | 优先级 |
|------|------------|--------|---------|--------|
| Web 前端控制台 | 未在设计文档详述，但属于交互入口 | `web/` 完整HTML/CSS/JS | **生成的 Manifest apiVersion 为 `solution.ai/v1alpha1`，后端仅支持 `solution.codex/v1`**；缺少 `solutionType` 字段；组件引用格式错误 | P0 |
| 前端模板系统 | 未在设计文档详述 | 3个内置模板（support/qa/ticket） | 生成的 Manifest 不包含 `components` 段，缺少 `category` 和 `ref` | P0 |
| 知识质量门禁执行 | Phase 1 MVP 知识流水线 | `loader.go` 执行基础文件级校验 | `loader.go` 未执行 `m.Knowledge.QualityGates` 中的 `stale_content` 和 `conflicting_answers` 门禁；门禁仅在 `solution ingest` 中执行 | P1 |
| 方案模板系统（Phase 2） | Phase 2 交付物 | 3个YAML模板在 `templates/` | 模板仅为静态YAML文件，无 `solutionType` 驱动的模板匹配逻辑；FDE 选择模板后仍需手动编写 Manifest | P1 |
| 组件 `requires` 能力校验 | Phase 2 | Validator 中已校验 | 校验仅标记为 UNKNOWN/UNAVAILABLE，但 Runtime 启动时未阻断缺少能力的组件（如 LLM extractor 需要 `model.generate` 但运行时不报错） | P2 |
| `solution ingest` 命令 | Phase 2 | 完整实现 | 未实现 `markdown`/`md` 类型的 Python Worker 调用（标记为 warning） | P2 |
| 知识索引原子替换 | MVP 非功能约束 | Trace 写入有原子替换 | 知识索引构建（`loader.go`）直接构建内存索引，无临时目录原子替换机制 | P2 |
| Release 门禁 `signal_ingress_reachable` | Phase 4 | 检查 `endpointPath` 配置存在 | 未实际探测端点可达性，不符合设计意图 | P2 |
| 评测门禁 `schedule: weekly` | Phase 3 | 校验通过但不执行 | 设计文档要求 weekly 门禁声明仅作校验通过；当前无内置调度器——此为符合设计的行为 | N/A |
| ReuseStats 复用率统计 | Phase 4 | `marketplace.go` 中 `ComputeReuseStats` | 函数体为空（返回零值），Placeholder 未实现 | P3 |

### 完全未实现的功能或模块

| 功能/模块 | 设计文档位置 | 优先级推测 | 影响 |
|-----------|------------|-----------|------|
| 子工作流引擎 | Phase 3 明确排除但 P2 提及 | P2（非MVP） | 复杂分支场景无法表达 |
| Kubernetes 部署生成 | Phase 4 明确排除 | P3（非MVP） | 仅支持 Docker Compose |
| 组件市场（团队共享） | Phase 4 | P2 | `solution component publish` CLI 存在但市场功能未实现 |
| PDF/Word→JSONL Python Worker | Phase 2 | P2 | Python Bridge 代码存在但未被调用流程集成 |
| 向量检索（pgvector） | Phase 2 | P3（Phase 2后） | 当前仅关键词检索 |
| 多 Sensor 编排与 Signal 增强 | W2A 感知层（未来职责） | P3 | 单 Sensor 模式 |
| `solution destroy` 命令 | 待决策项 | P3 | 清理部署资源 |
| `embedding` 配置的实际使用 | RuntimeSpec | P3 | 声明存在但无对应功能 |
| 知识回归测试框架 | Phase 2 未来职责 | P3 | 评测框架存在可复用但专用知识测试未实现 |

---

## 致命 Bug 报告

### 致命 Bug #1：Web 前端生成的 Manifest 与后端完全不兼容

- **文件**：`/Users/cc/ai/fde-support/web/app.js`，函数 `buildManifest()`
- **严重等级**：致命（FATAL）
- **触发条件**：用户通过 Web 控制台生成 Manifest 后执行 `solution validate` 或 `solution run`

**具体问题**：

1. **`apiVersion` 值错误**（第~82行）：
   ```js
   return `apiVersion: solution.ai/v1alpha1
   ```
   后端 Validator 仅接受 `solution.codex/v1`（[validator.go:69](/Users/cc/ai/fde-support/internal/manifest/validator.go:69)）。导致所有 Web 生成的 Manifest 立即被拒绝。

2. **缺少 `solutionType` 字段**：后端 Validator 要求 `solutionType` 为必填字段（[validator.go:74](/Users/cc/ai/fde-support/internal/manifest/validator.go:74)），前端生成的 Manifest 未包含此字段。

3. **缺少 `components` 段**：前端生成的 Manifest 没有 `components` 列表，只有 `workflow.nodes[].component` 引用。后端通过 `componentByID[node.Component]` 查找组件实例，找不到会导致运行时崩溃。

4. **`perception.triggers[].routeTo` 值错误**：前端生成 `routeTo: support_agent`，但 nodes 中第一个节点的 id 是 `classify_intent`，Validator 会报 `UNKNOWN_WORKFLOW_ROUTE`。

5. **缺少 `knowledge.schemas`**：知识源引用了 `schema: faq` 但未声明对应的 schema。

**影响范围**：Web 控制台作为面向 FDE 的交互入口完全不可用——任何通过 Web 生成的 Manifest 都无法通过验证，用户必须手动编写 YAML。

**修复建议**：
- 修正 `apiVersion` 为 `solution.codex/v1`
- 增加 `solutionType` 字段（根据模板映射，如 support→`customer-support`）
- 在生成的 Manifest 中加入完整的 `components` 列表（含 `category`、`ref` 和 `config`）
- 修正 `routeTo` 值为 `classify_intent`
- 增加 `knowledge.schemas` 声明

---

### 致命 Bug #2：Processor 组件输出 Schema 违反处理器契约

- **文件**：`/Users/cc/ai/fde-support/internal/registry/types.go`，`builtinComponentDescriptors()`
- **严重等级**：致命（FATAL）
- **触发条件**：任何引用以下 Phase 2 组件的 Manifest 运行时

**具体问题**：

以下三个 Processor 组件的 `OutputSchema` 均包含 `"status": "string"`，违反了设计文档的处理器契约：

| 组件 Ref | 问题字段 | 行号参考 |
|----------|---------|---------|
| `registry.processor.llm-extractor@1.0.0` | `OutputSchema: {"status": "string", "extracted": "string?"}` | types.go |
| `registry.processor.data-query@1.0.0` | `OutputSchema: {"status": "string", "rows": "array", ...}` | types.go |
| `registry.processor.rule-evaluator@1.0.0` | `OutputSchema: {"status": "string", "matched": "boolean", ...}` | types.go |

设计文档明确规定：
> 组件错误模型规范：不可恢复错误通过 `(nil, error)` 抛出；可恢复业务状态通过 `output` 中的 `status` 字段表达，但适用于 action 组件
> 禁止：组件不得通过 `output["status"] = "error"` 来模拟系统异常
> Post-condition: processor components must not emit "status" in output.

矛盾点：Bult-in 的 descriptor 声明了 `status` 字段，而 executor 会打印 DEBUG 警告（[executor.go:139](/Users/cc/ai/fde-support/internal/runtimecore/executor.go:139)）指出处理器组件不应输出 `status`。这导致平台的契约规范与自身的组件定义自相矛盾。

**影响范围**：所有 Phase 2 通用处理器组件（llm-extractor、data-query、rule-evaluator）的契约定义不一致；下游节点如果依赖 `status` 字段会因类型流校验通过而期望该字段存在，但 executor 认为这是契约违规。

**修复建议**：
- 从三个 Processor 组件的 `OutputSchema` 中移除 `status` 字段
- 处理器组件只通过返回值 `error` 报错
- 如需表达业务状态，应在 Action 组件中通过 `status` 输出

---

### 致命 Bug #3：`release` 命令安全基线检查过于严苛导致所有解决方案无法发布

- **文件**：`/Users/cc/ai/fde-support/internal/release/checker.go`
- **函数**：`checkObservability()` (行 ~218)，`checkSecurityBaseline()` (行 ~225)
- **严重等级**：高（HIGH）

**具体问题**：

```go
func (c *Checker) checkObservability(ctx context.Context) CheckResult {
    if c.manifest.Runtime.Observability.Trace != "required" {
        return CheckResult{Name: "observability_enabled", Passed: false, Severity: "block", ...}
    }
```

```go
func (c *Checker) checkSecurityBaseline(ctx context.Context) CheckResult {
    sec := c.manifest.Delivery.Security
    if sec.PIIDetection != "required" { ... }      // block
    if sec.PromptInjectionDefense != "required" { ... }  // block
```

- `observability_enabled` 要求 Manifest 显式设置 `observability.trace: "required"`，但设计文档未将此作为 Phase 1 的硬性要求。
- `security_baseline_passed` 要求 `piiDetection: "required"` 和 `promptInjectionDefense: "required"`，但这些能力在设计文档中列为 Phase 4 交付物，未在 Phase 1 实现。
- `knowledge_quality_passed` 和 `eval_gates_passed` 被标记为 `mandatoryReleaseChecks`，**即使 Manifest 未在 `releaseChecks` 中声明也会执行**。这意味着任何解决方案都必须通过所有4个强制门禁才能发布，即使 FDE 只配置了其中一个。

**触发条件**：执行 `solution release manifest.yaml --env=production` 时，若 Manifest 未设置：
- `runtime.observability.trace: "required"`
- `delivery.security.piiDetection: "required"`
- `delivery.security.promptInjectionDefense: "required"`

则必然失败。

**影响范围**：所有解决方案的发布流程被阻断。当前示例 Manifest（`examples/after-sales-support/manifest.yaml`）恰好设置了这些字段所以能通过，但这是一个容易遗漏的陷阱。

**修复建议**：
- 将未实现的检查降级为 `severity: "warn"` 而非 `severity: "block"`
- 或者，mandatoryReleaseChecks 仅当对应字段在 Manifest 中显式声明时才执行强制检查
- 对于 Phase 4 才实现的能力（PII检测、Prompt注入防御），不应在 Phase 1 作为阻断条件

---

### 高级严重问题（非致命但影响显著）

#### H1：知识质量门禁在 `solution run` 时未执行

- **文件**：`internal/knowledge/loader.go` vs `internal/knowledge/ingest.go`
- **问题**：`LoadWithOptions()` 函数加载知识源时，只做文件格式校验（JSON 合法性、`source_ref` 存在、内容非空），但不执行 `m.Knowledge.QualityGates` 中的门禁检查（`stale_content`、`conflicting_answers`）。这些检查仅在 `solution ingest` 命令（`ingest.go`）中执行。
- **设计文档依据**：Phase 1 MVP 知识流水线要求"执行基础质量门禁"；Runtime 启动时应确保加载的知识已通过质量门禁。
- **修复**：在 `LoadWithOptions` 中调用 `runQualityGates` 或等效逻辑。

#### H2：`scopedKnowledge` 方法类型断言可能静默失败

- **文件**：`internal/runtimecore/executor.go:126-138`
- **问题**：
  ```go
  func (e *Executor) scopedKnowledge(componentID string) registry.KnowledgeReader {
      if e.knowledgeStore == nil { return e.knowledge }
      for _, binding := range e.manifest.Runtime.KnowledgeBindings {
          if binding.Component == componentID {
              if ks, ok := e.knowledgeStore.(*knowledge.Store); ok {
                  return ks.FilterBySources(binding.Sources)
              }
              return e.knowledge
          }
      }
      return e.knowledge
  }
  ```
  `knowledgeStore` 被赋值为 `any` 类型（[executor.go:55](/Users/cc/ai/fde-support/internal/runtimecore/executor.go:55)），类型断言 `e.knowledgeStore.(*knowledge.Store)` 可能对未来的知识存储实现静默失败，返回未过滤的完整知识库而不是过滤后的子集。这会导致 `knowledgeBindings` 的隐私隔离失效。
- **修复**：将 `knowledgeStore` 改为接口类型 `KnowledgeStoreFilter`，或使用更安全的类型断言模式。

#### H3：`buildManifest()` 中的组件引用使用裸名而非注册表引用

- **文件**：`web/app.js`
- **问题**：前端生成的 Manifest 使用 `component: intent_classifier` 等裸名，而设计文档要求使用 `ref: registry.intent.beverage-router@1.0.0` 格式。虽然后端可以通过 `componentByID` 映射，但前端 Manifest 的 `components` 段格式完全错误（使用 `component` 字段而非 `ref` 字段）。这在当前实现中导致前端生成的 Manifest 无法通过验证。

#### H4：模型 Gateway 默认使用 mock 策略可能掩盖真实故障

- **文件**：`internal/app/model_gateway.go`
- **问题**：
  ```go
  func buildModelGateway(env environment.ResolvedEnvironment, allowMock bool) (registry.ModelGateway, error) {
      if env.DefaultModel == "" {
          if allowMock { return model.NewMockGateway(), nil }
          return nil, fmt.Errorf("...")
      }
      ...
  }
  ```
  当 `DefaultModel` 为空且 `allowMock=true` 时返回 Mock Gateway，但不会打印任何警告。这意味着如果用户忘记配置模型，系统会静默使用 Mock Gateway 返回固定响应，而不是报错。对于首次使用的 FDE，这可能误导为"系统正常运行"。
- **修复**：使用 Mock Gateway 时通过 stderr 输出警告。

#### H5：Trace 列表 API 返回敏感字段可能泄漏

- **文件**：`internal/trace/writer.go`，`List()` 方法
- **问题**：`List()` 方法反序列化完整的 Trace 文件内容，未经过 `RedactMap` 处理。虽然在写入时已脱敏，但如果由于任何原因（如直接文件写入、手动编辑）导致 Trace 文件包含未脱敏数据，`GET /api/traces` 将直接暴露敏感信息。

---

## 与设计文档的一致性检查

### 结构一致性

| 设计文档 | 代码实现 | 一致性 |
|----------|---------|--------|
| `manifest.SolutionManifest` 顶层结构 | 9个字段全部对应（含 `BaseDir`/`Path` 内部字段） | 一致 |
| `apiVersion: solution.codex/v1` | Validator 强制要求 | 一致 |
| `solutionType` 支持4种类型 | Validator 支持 `customer-support`, `data-inquiry`, `alert-escalation`, `approval-flow` | 一致 |
| `perception.sensors[].config.authTokenRef` | Validator 校验 `env:` 格式 | 一致 |
| `workflow.nodes[].when` 仅支持单条件 | `ParseWhen` 拒绝 `&&`, `||`, `[`, `(`, `)` | 一致 |
| `workflow.onError.retry` | Executor 按次数重试 | 一致 |
| W2A Signal 幂等键 `(environment, sensor_id, signal_id)` | `SignalIdempotencyKey` 结构匹配 | 一致 |
| 组件错误模型（硬失败 vs 软失败） | Executor 区分 `status: "error"/"failed"` 和非错误 status | 一致 |
| MVPRuntime 响应格式 `{answer, intent, confidence, citations, traceId}` | `mapResponse()` 实现完整 | 一致 |

### 接口签名一致性

| 接口 | 设计文档 | 代码实现 | 一致性 |
|------|---------|----------|--------|
| `Component.Run(ctx, input, runtime) → (output, error)` | Component 接口 | 完全匹配 | 一致 |
| `RuntimeContext.Knowledge()` | `KnowledgeReader` 接口 | 完全匹配 | 一致 |
| `RuntimeContext.Model()` | `ModelGateway` 接口 | 完全匹配 | 一致 |
| `RuntimeContext.HTTP()` | `HTTPCaller` 接口 | 完全匹配 | 一致 |
| `KnowledgeReader.Retrieve(ctx, query, topK)` | 3参数接口 | 完全匹配 | 一致 |

### 设计文档本身的模糊或矛盾之处

1. **知识质量门禁执行时机不明确**：设计文档说"执行基础质量门禁"和"持久化质量报告"，但未明确说明是在 `solution run` 时自动执行还是必须通过 `solution ingest` 手动触发。当前实现选择了后者，但 `solution run` 仍会加载知识，这造成两个入口的质量保障不一致。

2. **模板系统定位模糊**：文档在"设计哲学"中详细描述了模板系统，但在 MVP 实施计划中标记为 Phase 2。Phase 1 的 `--template` flag 已实现，但仅支持静态 YAML 文件，不是设计哲学中描述的"选模板 → 改配置 → 拉起方案"流程。

3. **Processor 组件 `status` 字段矛盾**：文档规定处理器组件不应输出 `status`，但三个内置 Phase 2 处理器组件的 `OutputSchema` 声明了 `status`。

4. **Web 控制台的设计契约缺失**：设计文档未对 Web 控制台的功能边界和 Manifest 生成格式做出严格约束，导致前端实现与后端协议完全脱节。

---

## 附录：测试覆盖情况

| 包 | 测试文件 | 行数 | 覆盖范围 |
|----|---------|------|---------|
| `cmd/solution` | main_test.go | 56 | CLI 命令基础测试 |
| `internal/api` | server_test.go, signal_router_test.go, trace_endpoints_test.go, static_test.go | ~582 | HTTP API 全面测试 |
| `internal/evaluation` | metrics_test.go, runner_test.go | ~173 | 评测指标与执行 |
| `internal/knowledge` | loader_test.go | 133 | 知识加载与检索 |
| `internal/manifest` | validator_test.go | 291 | Manifest 校验 |
| `internal/registry` | catalog_test.go, marketplace_test.go | ~234 | 组件注册与发布 |
| `internal/release` | checker_test.go | 286 | 发布门禁 |
| `internal/runtimecore` | executor_test.go, types_test.go | ~144 | 执行器基础 |
| `internal/shared` | types_test.go | 75 | 类型系统工具 |
| `internal/trace` | writer_test.go | 176 | Trace 写入 |
| `internal/w2a` | adapter_test.go | 56 | W2A 适配 |
| `internal/workflow` | when_test.go, plan_test.go | ~218 | Workflow 编译执行 |
| `internal/model` | 无测试文件 | 0 | **未覆盖** |
| **总计** | **22个文件** | **~2424** | ~70% 行覆盖（估算） |

### 测试缺口

1. `internal/model/` 包无任何测试文件——`openai.go`、`gateway.go`、`mock.go` 均未测试
2. `internal/knowledge/ingest.go` 无专门测试（质量门禁逻辑未测试）
3. `internal/app/app.go` 的 `RunHTTP`、`BuildRuntime` 等核心编排函数无集成测试
4. Web 前端 `app.js` 无自动化测试

---

*审查工具: 人工代码审查 + `go test ./...` + 设计文档逐点比对*
*审查人: AI Code Review Agent*
*审查轮次: 第6轮（含第1-4轮增量审查）*

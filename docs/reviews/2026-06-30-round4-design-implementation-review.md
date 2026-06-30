# 2026-06-30 Round 4 Design Implementation Review

## 审查摘要

**审查基准**：
- `docs/solution-as-code-fde-platform-design.md`（详细设计文档）
- `docs/solution-as-code-fde-platform-technical-architecture.md`（技术架构）
- `docs/development-plan.md`（开发计划）
- `docs/specs/*`（实现规格附件）

**审查范围**：
- 仅分析受版本控制的项目源码、示例、模板和项目文档
- 忽略第三方库、自动生成代码、运行期产物
- 审查深度：逐模块比对设计文档与代码实现，覆盖编译验证、测试运行、数据流追踪

**验证命令与结果**：

| 验证项 | 命令 | 结果 |
|---|---|---|
| 全工作区测试 | `go test ./cmd/... ./internal/... -count=1` | 1 个测试失败（platform-specific） |
| tracked-only 构建 | `git ls-files` 范围内 `go build ./...` | **通过**（较 R3 修复） |
| 示例 validate | `solution validate examples/{after-sales-support,guoran-support}/manifest.yaml` | 通过 |
| release smoke | `solution release --env poc` with after-sales-support | 通过 |
| 路径逃逸防御 | `uri: ../secret.txt` → validate | **通过**（较 R3 修复） |
| 绝对路径防御 | `uri: C:\secret.txt` → validate（macOS） | **失败**（cross-platform issue） |

**整体结论**：

- 第 4 轮相较第 3 轮有 **1 个关键进展**：`internal/app/model_gateway.go` 已纳入版本控制，tracked-only 构建闭环恢复。R3 的两个致命 Bug 中已修复 1 个。
- **新增了 3 个独立发现的致命/高风险 Bug**：涉及 processor 组件违反设计契约、knowledgeBindings 运行时未生效、cross-platform 路径校验失效。
- 工程整体完成度估计为 **70%**（当前工作区），较 R3 下降 2%，因为新增发现暴露了更深层的违规。
- 致命 Bug **2 个**（1 个从 R3 遗留 + 1 个新发现），高风险问题 **6 个**（4 个从 R3 遗留 + 2 个新发现）。

**完成度计算依据**：

| 领域 | 权重 | R4 得分 | 说明 |
|---|---:|---:|---|
| Manifest 与校验 | 15 | 13 | 路径逃逸防御已起步；cross-platform 和 delivery 二级防御缺失 |
| Runtime/W2A/Trace | 20 | 17 | 构建闭环恢复；knowledgeBindings 未运行时过滤；Phase 2 processor 返回 status 违反契约 |
| Knowledge | 15 | 10 | loader/report/fingerprint 基础存在；Python Worker/Markdown/PostgreSQL 未闭环 |
| Component/SDK | 15 | 9 | component publish 可打包；自定义组件执行 SDK 不完整；Phase 2 组件违反 processor 契约 |
| Evaluation/Release Gate | 15 | 9 | onRelease gate 可执行；缺 cache/fingerprint；eval 环境硬编码为 poc；缺少 metric 静默跳过 |
| Delivery/Marketplace | 15 | 8 | release 产物更完整；路径和 image 问题仍存在；releaseChecks 可跳过强制门禁 |
| Templates/Examples/Docs | 5 | 4 | 2 个示例 + 3 个模板可 validate；文档齐全 |
| **合计** | **100** | **70** | 总体完成度约 70% |

---

## 完成度详情

### 已完整实现的功能模块

| 模块 | 状态 | 证据 |
|---|---|---|
| Manifest Loader（YAML → Go struct） | 完整 | `internal/manifest/loader.go`，保留 Path/BaseDir |
| Manifest Validator 基本框架 | 完整 | 分阶段校验：结构→唯一ID→交叉引用→密钥引用→工作流语法→数据流→组件契约→知识Schema→类型流 |
| W2A Signal 校验与归一化 | 完整 | `internal/w2a/signal.go`：schema_version、字段完整性、sensor_id 校验 |
| W2A Webhook 入口（Auth + 幂等） | 完整 | `internal/api/signal_router.go`：Bearer token、幂等缓存、拒绝 Trace |
| 工作流执行器（线性序列 + when + retry + fallback） | 完整 | `internal/runtimecore/executor.go` |
| When 条件解析器（MVP 子集：6 操作符 + 一层字段访问） | 完整 | `internal/workflow/when.go` |
| Trace Writer（JSON 文件 + 原子 rename） | 完整 | `internal/trace/writer.go`：Start/AppendSpan/Finish |
| 内置组件（7个 Phase 1 + 4 个 Phase 2） | 完整 | `registry/components.go`：descriptor + factory |
| 组件注册表框架（内置 map + 目录扫描 + component.yaml） | 完整 | `internal/registry/` |
| 知识加载器（JSONL + CSV + 关键词索引 + 质量报告 + fingerprint） | 完整 | `internal/knowledge/loader.go` |
| CLI 命令集（validate/run/ingest/evaluate/release/component-publish） | 完整 | `cmd/solution/main.go` |
| Release 基础门禁（8 项检查） | 完整 | model/sensor/action/signal/knowledge/eval/observability/security |
| Release 基础产物（Docker Compose + .env.example + README） | 完整 | `internal/delivery/docker_compose.go` |
| 方案模板（3 个） | 完整 | `templates/customer-support.yaml`、`data-inquiry.yaml`、`alert-escalation.yaml` |
| 示例 Manifest（2 个） | 完整 | `examples/after-sales-support/`、`examples/guoran-support/` |
| Logger 脱敏 | 完整 | `runtimeLogger` 调用 `trace.RedactMap` |
| TraceWriter 脱敏 | 完整 | `Start`/`AppendSpan` 调用 `RedactMap` |
| 类型流校验（Type Flow） | 完整 | `validator_typeflow.go`：5×5 兼容矩阵 |
| Component publish 打包 | 完整 | `registry/marketplace.go` 生成 tar.gz |

### 部分实现的功能

| 功能 | 缺失细节 | 设计位置 | 优先级 |
|---|---|---|---|
| **知识源路径安全（二级防御）** | Validator 已拒绝 `../` 路径，但 `copyRuntimeInputs` 仍未增加防御性检查；`TestValidatorRejectsAbsoluteKnowledgeSourcePath` 在 macOS 上失败（cross-platform issue） | Manifest 路径解析、交付自包含 | P0 |
| **处理器组件违反输出契约** | `llmExtractor`、`dataQuery`、`ruleEvaluator` 返回 `{"status": "ok"}`，违反设计规定 processor 禁止包含 `status` 字段 | Component SDK、组件契约 | P0 |
| **knowledgeBindings 运行时未过滤** | Validator 校验了绑定关系，但 Executor 传入所有组件的 knowledge 是全量 store，未按 binding 过滤子集 | `runtime.knowledgeBindings` | P1 |
| Release 知识检查时序 | Release 在 checker 前调用 `LoadWithOptions`，不符合"先只读检查已有报告"的流程 | 发布检查最小标准 | P1 |
| Eval cache/fingerprint 复用 | `ComputeFingerprint` 存在但未用于 release cache 读写；缺 1 小时 TTL 复用 | `eval_gates_passed` | P1 |
| 模型网关配置弹性 | 缺少 OpenAI-compatible base URL/env 字段；evaluate 和 release 不可用 mock fallback | RuntimeContext/Model Gateway | P1 |
| Delivery runtime image | compose 使用 `image: solution-runtime:<version>`，无 Dockerfile 或构建说明 | Phase 4 交付 | P1 |
| 自定义组件运行 SDK | 只能加载 descriptor + 实例化内置 factory；不能执行方案级自定义 Go `Run()` 实现 | Component SDK | P1 |
| Knowledge ingest Python Worker | Go ingest 对 Markdown 只 warning，不调用 worker；PDF/Word/PostgreSQL 未实现 | Phase 2 知识流水线 | P2 |
| 组件复用统计 | `ComputeReuseStats` 仍固定零值 | Phase 4 复用率 | P2 |

### 完全未实现的功能

| 功能 | 缺失表现 | 设计位置 | 优先级 |
|---|---|---|---|
| 生产级 eval cache | 无缓存读写，无 execution/dataset/knowledge fingerprint 失效策略 | `eval_gates_passed` | P1 |
| 自定义组件运行 | 方案级 `components/<name>/component.yaml + Run()` 不能自动执行 | Component SDK | P1 |
| PostgreSQL/Redis 持久化 | W2A 幂等仍为进程内 TTL；知识 store 为内存 | Phase 2 | P2 |
| PDF/Word 解析流水线 | Python worker 未从 Go CLI 接入 | Phase 2 Python Worker | P2 |
| `weekly` 调度执行 | 无内置调度器触发 weekly gate 执行 | Phase 3 评测 | P2 |
| solution destroy 命令 | 未实现 | Phase 4 | P2 |
| 复用率统计面板 | `ComputeReuseStats` 固定为 0 | Phase 4 | P2 |

---

## 致命 Bug 报告

### BUG-R4-001：知识源路径逃逸在 delivery 层仍可被利用（R3-BUG-R3-002 遗留）

严重等级：**致命**。

**所在文件与函数**：
- `internal/delivery/docker_compose.go` → `copyRuntimeInputs`
- `internal/manifest/validator.go` → `validateRelativeManifestPath`

**触发条件与复现路径**：
1. 在 Manifest 中配置：
   ```yaml
   knowledge:
     sources:
       - id: leaked
         type: jsonl
         uri: /etc/passwd   # Unix 绝对路径
         schema: faq
   ```
2. 若 validator 因任何原因未拒绝（如未来新增 source type、validator bypass），delivery 层的 `filepath.Join(m.BaseDir, "/etc/passwd")` 会生成 `/etc/passwd`（Join 丢弃前缀），导致外部文件被复制到发布目录。
3. 更隐蔽的路径：`filepath.Join(outputDir, "../../sensitive")` 也会逃逸。

**具体原因**：
- Validator 已防御 `../` 路径逃逸（R3 修复）
- 但在 macOS/Linux 上，`filepath.IsAbs("C:\\secret.txt")` 返回 false，validator 对 Windows 风格绝对路径在非 Windows 平台放行
- delivery 层 `copyRuntimeInputs` 未做独立性防御检查（源路径必须在 `BaseDir` 内，目标路径必须在 `outputDir` 内）

**影响范围**：
- 可在非 Windows 平台绕过 Windows 风格绝对路径的校验
- delivery 层缺少防御性路径 containment 检查
- 潜在的安全漏洞和数据泄露风险

**修复建议**：
1. 将 `validateRelativeManifestPath` 的检测逻辑改为平台无关正则：拒绝以 `/`、`\`、`[A-Z]:` 开头和包含 `..` 的路径
2. 在 `copyRuntimeInputs` 中增加防御性检查：`filepath.HasPrefix` 或使用 `filepath.Rel` 验证路径 containment
3. 增加跨平台单元测试：覆盖 `/etc/passwd`、`C:\secret.txt`、`..\secret.txt` 等
4. validator 和 delivery 层均做防御

---

### BUG-R4-002：Phase 2 processor 组件返回 `status` 字段违反设计契约（新发现）

严重等级：**致命**。

**所在文件与函数**：
- `internal/registry/components.go`:
  - `llmExtractor.Run()` → 返回 `{"extracted": ..., "status": "ok"}`
  - `dataQuery.Run()` → 返回 `{"rows": ..., "status": "ok"}`
  - `ruleEvaluator.Run()` → 返回 `{"matched": ..., "status": "ok"}`
- `internal/runtimecore/executor.go` → `executeNodeWithRetry`（第 170-178 行）

**触发条件与复现路径**：
1. 在 Manifest 中使用任一 Phase 2 processor 组件
2. 组件执行成功，返回 `{"status": "ok", ...}`
3. `llmExtractor` 在 model gateway 不可用时返回 `{"status": "failed", "error": {...}}`（而非 `(nil, error)`）
4. 平台检查 `status == "failed"` 且 `CategoryProcessor`，判定为 hard failure → 进入重试和 fallback → 即使这是可恢复业务状态

**具体原因**：
- 设计规范明确：`processor` 组件输出应为纯业务数据，**禁止包含 `status` 字段**；失败通过 `(nil, error)` 传递
- Phase 2 组件（`llmExtractor`、`dataQuery`、`ruleEvaluator`）在成功时输出 `status: "ok"`，在可恢复失败时输出 `status: "failed"`
- `llmExtractor` 把 model 不可用当作业务状态（返回 output），而非系统异常（error），破坏了错误模型

**影响范围**：
- Phase 2 组件与 Phase 1 组件的错误模型不一致
- `llmExtractor` 的 model 不可用场景触发不必要的重试和 fallback
- 下游组件可能依赖 `status` 字段做判断，产生隐式耦合

**修复建议**：
1. 从所有 `processor` 组件输出中移除 `status` 字段
2. `llmExtractor` 在 model 不可用时返回 `(nil, error)`
3. `dataQuery` 和 `ruleEvaluator` 移除 `status` key，纯业务字段命名
4. 在 executeNodeWithRetry 中对 processor 组件输出做 post-condition 检查
5. component.yaml 规范文档中增加 processor 禁止 status 字段的约束

---

## 高风险问题

### HIGH-R4-001：`runtime.knowledgeBindings` 校验通过但运行时未过滤（新发现）

严重等级：**高**。

**文件**：`internal/runtimecore/executor.go:158`

**问题**：Validator 正确校验了 `runtime.knowledgeBindings` 中的绑定关系，但 Executor 传入所有组件的 `runtimeContext.knowledge` 都是全量 store（`e.knowledge`），未按 binding 过滤。同一 Solution 中多个 retriever 无法绑定不同知识源子集。

**修复**：在 `executeNodeWithRetry` 中根据 `runtime.knowledgeBindings` 创建每个组件的 scoped KnowledgeReader。

---

### HIGH-R4-002：`evaluate` 命令硬编码 `poc` 环境（新发现）

严重等级：**高**。

**文件**：`internal/app/app.go` → `EvaluateManifestFile`（第 210 行）

**问题**：`resolvedEnv, err := environment.Resolve(m, "poc")` 固定使用 `poc`。无法对 production 环境的模型策略、知识绑定执行评测。

**修复**：`solution evaluate` 接受 `--env` 参数，与 `run`/`release` 保持一致。

---

### HIGH-R4-003：releaseChecks 非空时可绕过强制门禁（R3-BUG-R3-003 遗留）

严重等级：**高**。

**文件**：`internal/release/checker.go` → `Checker.Run`

**问题**：非空 `delivery.releaseChecks` 被解释为精确过滤集合，可排除 `knowledge_quality_passed` 和 `eval_gates_passed`。

**修复**：生产环境强制包含 knowledge 和 eval checks。validate 拒绝生产环境排除 P0 检查的配置。

---

### HIGH-R4-004：Release 产物缺 runtime image 来源（R3-BUG-R3-004 遗留）

严重等级：**高**。

**文件**：`internal/delivery/docker_compose.go`

**问题**：compose 使用 `image: solution-runtime:<version>`，无 Dockerfile 或镜像获取说明。

**修复**：选择明确策略（生成 Dockerfile / 传入 image 参数 / 使用 build context）。

---

### HIGH-R4-005：Eval gate 缺失指标时静默跳过（R3-BUG-R3-005 遗留）

严重等级：**高**。

**文件**：`internal/evaluation/runner.go` → `Runner.Run`

**问题**：gate loop 中对缺失 metric 用 `continue` 跳过。若 `severity: block` 的 onRelease gate 的 metric 在所有 case 中 not applicable，release 可能误通过。

**修复**：缺失 metric 时生成 failed GateResult 或返回错误。

---

### HIGH-R4-006：模型网关在 evaluate/release 中拒绝 mock fallback（新发现）

严重等级：**高**。

**文件**：`internal/app/app.go` → `EvaluateManifestFile`（第 218 行）、`ReleaseManifestFile`（第 271 行）

**问题**：两个函数均调用 `buildModelGateway(resolvedEnv, false)`（`allowMock=false`）。若 workflow 不使用 `model.generate` 能力，评测和发布仍因密钥缺失失败。

**修复**：当无组件需要 `model.generate` 时允许 mock gateway；或 evaluate/release 使用 `allowMock=true`。

---

## 中低风险问题

1. **MED-R4-001**：`executeNodeWithRetry` 对 `input_mapping_error` 和 `input_type_mismatch` 执行无意义重试。这些错误是确定性的，重试无收益。应直接返回。文件：`internal/runtimecore/executor.go:148-157`。

2. **MED-R4-002**：`human_handoff` action 组件缺少 `target` 字段。设计期望 `{"status":"handed_off","target":"support-l2"}`，实际返回 `{"status":"created","queue":...}`。文件：`internal/registry/components.go:213-225`。

3. **MED-R4-003**：`internal/model` 包无测试文件。OpenAI provider、Gateway、MockProvider 缺少单元测试。

4. **MED-R4-004**：`recordContent` 函数对非字符串字段静默跳过。number 类型的 `answer` 被排除在索引之外。文件：`internal/knowledge/loader.go`。

5. **LOW-R4-001**：`apiVersion` 校验硬编码，缺少版本兼容性框架。
6. **LOW-R4-002**：`synthesizeAnswer` 直接拼接 passages 而非调用 model gateway。
7. **LOW-R4-003**：`internal/model` 无 Go test 文件。

---

## 设计一致性检查

### 代码结构与模块划分
- 目录结构与技术架构保持一致，核心模块划分清晰
- `internal/` 内包遵循依赖方向，无明显跨层直接依赖违规

### 接口签名和数据模型
- `SolutionManifest` 含 `SolutionType`，与设计一致
- `RuntimeContext` 接口与设计能力清单一致
- `Component` 接口：`Run(ctx, input, runtime) → (output, error)` 符合设计

### 关键流程偏差
- **knowledgeBindings 未过滤**（见 HIGH-R4-001）
- **Phase 2 processor 返回 status**（见 BUG-R4-002）
- **evaluate 环境硬编码**（见 HIGH-R4-002）
- **release 检查流程顺序**：Release 仍在 checker 前加载知识 store

### 设计文档自身的模糊或矛盾
- `delivery.releaseChecks: []` 语义需统一
- 自定义 Go 组件运行机制（plugin/sidecar/subprocess）需明确
- 模型 provider 的 base URL/provider type/fallback 字段边界待补充
- 交付包中 runtime image 来源需设计明确

---

## R3 遗留 Bug 状态追踪

| Bug ID | 描述 | R4 状态 |
|---|---|---|
| BUG-R3-001 | 只提交已提交源码构建失败 | ✅ **已修复** — `model_gateway.go` 已纳入版本控制 |
| BUG-R3-002 | 知识源路径逃逸 | ⚠️ **部分修复** — `../` 已防御；cross-platform 和 delivery 层未完善 |
| BUG-R3-003 | releaseChecks 可绕过强制门禁 | ❌ **未修复** |
| BUG-R3-004 | Release runtime image 缺失 | ❌ **未修复** |
| BUG-R3-005 | Eval gate 缺失 metric 静默跳过 | ❌ **未修复** |

---

## 后续修复入口

Round 4 修复计划文件：
- `docs/superpowers/plans/2026-06-30-round4-design-review-remediation.md`

建议修复优先级：
1. **P0**：修复 Phase 2 processor 组件 `status` 字段（BUG-R4-002）
2. **P0**：完善知识源路径校验（cross-platform + delivery 二级防御）（BUG-R4-001）
3. **P1**：修复 `knowledgeBindings` 运行时过滤（HIGH-R4-001）
4. **P1**：evaluate 命令支持 `--env` 参数（HIGH-R4-002）
5. **P1**：防止 releaseChecks 跳过强制门禁（HIGH-R4-003）
6. **P1**：Release runtime image 策略（HIGH-R4-004）
7. **P1**：Eval gate 缺失 metric 处理（HIGH-R4-005）
8. **P1**：evaluate/release 模型网关 mock fallback（HIGH-R4-006）
9. **P2**：eval cache、Python Worker、复用统计、自定义组件 SDK、solution destroy

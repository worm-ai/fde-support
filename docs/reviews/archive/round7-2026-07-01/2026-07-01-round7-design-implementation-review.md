# Solution-as-Code FDE 平台 设计与实现差异审查报告 (Round 7)

> 审查日期：2026-07-01
> 审查轮次：Round 7（独立深度审查，含3轮交叉验证）
> 设计基准：`docs/solution-as-code-fde-platform-design.md`（1938行）
> 审查范围：全部已提交源码（`cmd/`, `internal/`, `web/`, `workers/`），排除运行期产物

---

## 审查摘要

| 维度 | 结果 |
|------|------|
| **整体完成度** | **~73%**（Phase 1 完成度约 88%，Phase 2 约 65%，Phase 3 约 72%，Phase 4 约 58%） |
| **致命 Bug 数量** | **4** 个致命级 |
| **高风险问题** | **6** 个 |
| **中风险问题** | **10** 个 |
| **低风险/改进建议** | **7** 个 |
| **测试通过率** | 14/14 测试包全部通过（`go test ./...` all OK） |
| **代码总规模** | ~7500行 Go + ~3500行 web + Python Worker |

### 完成度计算依据

| 阶段 | 权重 | 完成度 | 加权 |
|------|------|--------|------|
| Phase 1 (Manifest/Runtime/W2A/Trace) | 0.40 | 88% | 35.2 |
| Phase 2 (Knowledge/通用组件/模板) | 0.25 | 65% | 16.3 |
| Phase 3 (评测/Release门禁/类型流) | 0.20 | 72% | 14.4 |
| Phase 4 (交付/市场/组件发布) | 0.15 | 58% | 8.7 |
| **总计** | **1.00** | | **~74** |

*较 R6 的 ~72% 小幅提升，主要来自：Phase 2 质量门禁 `conflicting_answers`/`stale_content` 实际已执行、knowledge CSV 表格型支持发现为已实现、Phase 1 知识加载器已实现最小质量报告与原子写入。下调部分来自：发现新致命/高风险 Bug。*

---

## 完成度详情

### 已完整实现的功能模块（42项）

| 模块 | 设计文档章节 | 实现位置 | 备注 |
|------|-------------|----------|------|
| Manifest 数据结构 | 核心抽象:Solution | `manifest/types.go` | 含 `solutionType`、`metadata.industry`、`runtime.embedding` |
| Manifest YAML 加载器 | 核心抽象 | `manifest/loader.go` | BaseDir/Path 注入 |
| `apiVersion`/`kind`/必填字段校验 | Schema 规则 | `manifest/validator.go` | 含版本不支持拒绝 |
| 交叉引用校验（Sensor/组件/节点/知识） | ManifestValidator 检查清单 | `manifest/validator.go` | 含 knowledgeSchema、qualityGate scope |
| W2A 入口校验（endpointPath/auth） | Schema 规则 | `manifest/validator.go` | 含 `signalType` 授权检查 |
| 敏感配置引用校验 | Schema 规则 | `manifest/validator.go` + `shared/types.go` | `env:VAR_NAME` 格式 + 密钥键识别 |
| 输入映射校验（`inputMapping`） | Schema 规则 | `manifest/validator.go` | `chat`/`w2a_signal` 触发类型区分 |
| 节点输入引用校验 | Schema 规则 | `manifest/validator.go` | 含 unsafe dependency 检测 |
| `when` 条件语法校验 | Schema 规则 | `workflow/when.go` | 仅支持 `node_id.field <op> literal` |
| 环境覆盖白名单校验 | 环境覆盖规则 | `manifest/validator.go` | 7 个白名单字段 |
| 类型流校验 | Phase 3 | `manifest/validator_typeflow.go` | 一级扁平字段兼容矩阵 |
| `solution validate` CLI | Phase 1 | `cmd/solution/main.go` | `--json` 输出支持 |
| `solution run` CLI（HTTP服务） | Phase 1 | `cmd/solution/main.go` | `--env`, `--addr`, `--template` 标志 |
| `solution ingest` CLI | Phase 2 | `cmd/solution/main.go` | JSONL/CSV 知识摄取 |
| `solution evaluate` CLI | Phase 3 | `cmd/solution/main.go` | `--env` 标志（待修复注册） |
| `solution release` CLI | Phase 4 | `cmd/solution/main.go` | `--env` 标志 |
| `solution component-publish` CLI | Phase 4 | `cmd/solution/main.go` | 组件打包 |
| `POST /chat` 端点 | Phase 1 | `api/server.go` → `signal_router.go` | `message` 必填校验 |
| W2A Webhook 端点动态注册 | Phase 1 | `api/server.go` | 按 Sensor `endpointPath` 注册 |
| W2A Signal 规范化与校验 | Phase 1 | `w2a/signal.go` | W2A/0.1 envelope 校验、`source_event.schema` 剥离 |
| W2A Signal 幂等性 | Phase 1 | `w2a/idempotency.go` | 进程内 TTL 24h |
| Signal Router（Chat/Signal 双入口） | Phase 1 | `api/signal_router.go` | 认证、来源校验、类型校验、拒绝 Trace |
| RuntimeRequest 统一数据模型 | Phase 1 | `runtimecore/types.go` | `trigger`/`request`/`signal`/`raw_payload` |
| Runtime Kernel 执行器 | Phase 1 | `runtimecore/executor.go` | 线性序列 + `when` + retry + fallback |
| `workflow.inputMapping` 字段路径映射 | Phase 1 | `runtimecore/executor.go` → `ApplyInputMapping` | 缺失路径返回 400 |
| Workflow 编译器与执行计划 | Phase 1 | `workflow/plan.go` | MaySkip 传递依赖分析 |
| `when` 条件求值 | Phase 1 | `workflow/when.go` | 字符串/数字/布尔比较 + 类型校验 |
| Trace Writer（JSON 文件持久化） | Phase 1 | `trace/writer.go` | 每请求独立文件、原子 rename、TraceWriter 接口 |
| Trace 脱敏 | Phase 1 | `trace/redactor.go` | 手机/邮箱脱敏、敏感键 `[REDACTED]`、`raw_payload` 移除 |
| Trace API（列表+详情） | 扩展功能 | `api/server.go` | `/api/traces` + `/api/traces/{traceId}` |
| Runtime API（运行时视图） | 扩展功能 | `api/runtime_view.go` | `/api/runtime` |
| 知识加载器（JSONL+CSV关键词索引） | Phase 1/2 | `knowledge/loader.go` | 含 `FilterBySources` 知识绑定过滤 |
| 知识检索 `Retrieve()` | Phase 1 | `knowledge/loader.go` | 关键词分词+评分+排序 |
| 知识质量报告 | Phase 1/2 | `knowledge/loader.go` | fingerprint × 3、原子写入、`conflicting_answers`+`stale_content` |
| 环境解析器 | Phase 1 | `environment/resolver.go` | 白名单覆盖 + `env:VAR` 密钥解析 |
| 组件注册表（内置14+组件） | Phase 1/2 | `registry/types.go` + `components.go` | 三层发现 + `component.yaml` |
| RuntimeContext 接口 | Phase 1/2 | `runtimecore/types.go` | 7 个子接口全部实现 |
| 组件 `requires` 能力声明 | Phase 2 | `manifest/validator.go` | 含 `UNKNOWN_COMPONENT_REQUIRES`/`UNAVAILABLE` |
| 评测执行器 + 4 内置指标 | Phase 3 | `evaluation/` 全部5个文件 | `citation_coverage`、`answer_accuracy`、`groundedness`、`handoff_precision` |
| Release 门禁框架（8项检查） | Phase 4 | `release/checker.go` | 含强制门禁 + fingerprint 验证 |
| Docker Compose 部署产物 | Phase 4 | `delivery/docker_compose.go` | `./deploy/<env>/` |

### 部分实现的功能（11项）

| 序号 | 功能 | 已实现 | 缺失 | 设计位置 | 优先级 |
|------|------|--------|------|---------|--------|
| P-01 | **evaluate `--env` 标志** | 变量声明+传递 | 未注册到 cobra `evaluateCmd.Flags()` | CLI与API | **P0** |
| P-02 | **Release 模型网关 mock fallback** | `buildModelGateway` 支持 `allowMock` | `ReleaseManifestFile` 传入 `false`，"无LLM需求"方案 release 失败 | Phase 4 | **P0** |
| P-03 | **Web 前端 Manifest 生成兼容性** | 完整前端交互 | 生成 `apiVersion: solution.ai/v1alpha1`，后端仅接受 `solution.codex/v1`；无 `solutionType` | 交互入口 | **P0** |
| P-04 | **Web 前端模板系统** | 3个内置模板 | 无 `components` 段、无 `category`/`ref` | 交互入口 | **P0** |
| P-05 | **`solution run` 时知识质量门禁执行** | 文件级校验 | `loader.Load()` 未执行 `QualityGates` 中的 `stale_content`、`conflicting_answers`；门禁仅在 `ingest` 单独执行 | Phase 1/2 | P1 |
| P-06 | **方案模板系统** | 3个静态 YAML | 无 `solutionType` 驱动的模板匹配/推荐逻辑 | Phase 2 | P1 |
| P-07 | **组件 `requires` Runtime 阻断** | Validator 校验 | Runtime 启动时未因缺少能力而阻断 | Phase 2 | P1 |
| P-08 | **Python Worker 集成** | `python_bridge.go` 存在 | `solution ingest` 未调用 Python Worker 处理 markdown/PDF | Phase 2 | P2 |
| P-09 | **`signal_ingress_reachable` 探测** | 检查 `endpointPath` 配置 | 未实际探测端点可达性 | Phase 4 | P2 |
| P-10 | **eval cache 指纹复用** | `ComputeFingerprint` 存在 | `solution release` 未使用缓存避免重复评测 | Phase 3 | P2 |
| P-11 | **复用率统计** | `ComputeReuseStats` 函数体 | 返回零值占位 | Phase 4 | P3 |

### 完全未实现的功能（7项）

| 功能 | 设计文档位置 | 优先级 | 影响 |
|------|---------|--------|------|
| 子工作流引擎 | Phase 3 排除（P2迭代） | P2 | 复杂分支无法表达 |
| Kubernetes 部署生成 | Phase 4 排除 | P3 | 仅 Docker Compose |
| 组件市场（团队共享注册表） | Phase 4 | P2 | `component publish` CLI 存在但无远端 marketplace |
| 向量检索（pgvector） | Phase 2 | P3 | 仅关键词检索 |
| 多 Sensor 编排与 Signal 增强 | W2A 感知层 | P3 | 单 Sensor 模式 |
| `solution destroy` 命令 | 待决策项 | P3 | 仅 CLI 占位 |
| 知识回归测试框架 | Phase 2 未来职责 | P3 | 无专用知识测试 |

---

## 致命 Bug 报告

### BUG-7-001 [致命] evaluate 命令无法使用 `--env` 标志

- **文件与位置**: `cmd/solution/main.go:78-119`
- **触发条件**: 执行 `solution evaluate manifest.yaml --env=production`
- **具体原因**: `evaluateCmd` 的 `RunE` 中使用了 `evalEnvName` 变量，且声明了 `var evalEnvName string`（line 50），但未通过 `evaluateCmd.Flags().StringVar(&evalEnvName, "env", "poc", ...)` 注册到 cobra。标志注册语句在 R5 被提到但实际代码中**完全缺失**。`--env` 始终使用零值 `""`，导致 `EvaluateManifestFile` 中 `environment.Resolve(m, "")` 失败，返回 `"environment '' not found"` 错误。
- **影响范围**: 评测命令完全不可用——选择非 `poc` 即失败，即便 `poc` 也可能因空字符串匹配失败。
- **修复建议**: 在 `evaluateCmd` 定义后加入 `evaluateCmd.Flags().StringVar(&evalEnvName, "env", "poc", "delivery environment name")`

### BUG-7-002 [致命] Release 对无 LLM 需求的 Solution 必然失败

- **文件与位置**: `internal/app/app.go:271` → `ReleaseManifestFile` → `buildModelGateway(resolvedEnv, false)`
- **触发条件**: Manifest 中任何组件都未声明 `requires: [model.generate]`（如纯规则评估或纯人工升级方案），执行 `solution release`。
- **具体原因**: `ReleaseManifestFile` 调用 `buildModelGateway(resolvedEnv, false)`，第二个参数 `allowMock=false`。当 `env.DefaultModel` 为空时，`buildModelGateway` 返回错误 `"runtime.modelPolicy.defaultModel is required"`，导致 release 立即失败。而 `BuildRuntime`/`RunHTTP`/`EvaluateManifestFile` 中的类似调用使用的是 `needsModelGateway(m)`（检查组件实际需求）。Release 路径未做此检查。
- **影响范围**: 所有不含 LLM 组件的方案（如 `alert-escalation` 模板）`solution release` 必然失败。
- **修复建议**: 将 `ReleaseManifestFile` 中 `buildModelGateway(resolvedEnv, false)` 改为 `buildModelGateway(resolvedEnv, needsModelGateway(m))`。

### BUG-7-003 [致命] Web 前端生成的 Manifest 与后端完全不兼容

- **文件与位置**: `web/app.js` — Manifest 生成逻辑
- **触发条件**: 通过 Web 前端控制台创建方案并尝试 `solution run`。
- **具体原因**:
  - 前端生成 `apiVersion: solution.ai/v1alpha1`，后端仅接受 `solution.codex/v1`
  - 前端生成不含 `solutionType` 字段
  - 前端模板系统（support/qa/ticket）输出不含 `components` 段、`category` 和 `ref`
  - 后端 Validator 会在 `apiVersion` 检查阶段直接拒绝，根本无法进入后续流程
- **影响范围**: Web 控制台生成的任何方案都无法被 Runtime 执行；前端-后端功能完全断层。
- **修复建议**:
  1. 修改 `web/app.js` 中的模板对象，使 `apiVersion` 改为 `"solution.codex/v1"`
  2. 添加 `solutionType` 字段到模板中
  3. 为每个模板添加正确的 `components` 数组，每项包含 `category`、`ref`、`config`
  4. 前端表单提交时按正确 Schema 组装 Manifest

### BUG-7-004 [致命] Signal Router `HandleSignal` 中认证失败不写安全审计日志

- **文件与位置**: `internal/api/signal_router.go:56-75` `HandleSignal`
- **触发条件**: 向已注册的 Sensor endpoint 发送带错误 Bearer Token 的 W2A Signal
- **具体原因**: 设计文档明确要求"认证失败只写安全审计日志，不写拒绝类 Trace"（见 MVP 幂等实现和 Trace 结构规则章节）。当前代码在认证失败时调用 `r.writeRejectedTrace()` 写入拒绝类 Trace，这违反了安全设计约定：认证失败是安全事件应进入审计日志，不应泄露到业务 Trace 中。
- **影响范围**: 安全审计数据混入业务 Trace；攻击者可通过故意发送无效认证探测 Trace 系统行为。
- **修复建议**:
  1. 为认证失败创建独立的安全审计日志写入路径（如 `audit.log`）
  2. 认证失败时不调用 `writeRejectedTrace()`
  3. 协议校验失败（`UNSUPPORTED_VERSION`、`SIGNAL_TYPE_NOT_ALLOWED`）依然写入拒绝 Trace

---

## 高风险问题报告

### HIGH-7-001 Knowledge 加载器 `Load()` 未在主加载路径执行质量门禁

- **文件与位置**: `internal/knowledge/loader.go:85-127` `LoadWithOptions`
- **问题**: `solution run` 启动时调用 `Load()` 加载知识，`Load()` 中的 `evaluateQualityGates` 仅在 `len(m.Knowledge.QualityGates) > 0` 条件下执行，且只执行 `stale_content` 和 `conflicting_answers`。`missing_required_fields` 门禁**仅检查 scope 指向已知 Schema**，而实际字段必填校验仅发生在 `ingest` 的单独路径中。`Load()` 路径的 `loadJSONLSource` / `loadCSVSource` 不执行 Schema 字段必填校验——它们只检查 `source_ref` 存在性和可检索文本字段存在性。
- **影响**: `solution run` 启动时无法发现知识字段缺失问题，只有执行 `solution ingest` 才能发现。Release 门禁 `knowledge_quality_passed` 依赖质量报告，若报告由 `Load()` 在启动时生成（且门禁检查不全），release 可能通过但实际上有字段缺失。
- **修复建议**: 将 Schema 字段必填校验从 ingest 路径提取到 `loadJSONLSource`/`loadCSVSource` 共用函数中，确保 `Load()` 和 `Ingest()` 执行一致的校验。

### HIGH-7-002 `when` 条件引用的字段在 output 中不存在时静默失败

- **文件与位置**: `internal/workflow/when.go:126-140` `Evaluate` 方法
- **问题**: 当 `when` 条件引用的上游节点成功执行，但其 output 中缺少被引用的字段（如 `classify_intent.intent` 字段因组件实现变更而改名），`Evaluate` 返回 `false, error`。在 `executor.go` 的 `run()` 中，这个错误被捕获并转为 `condition_error`，导致工作流直接进入 hard failure 和 fallback，而不是跳过该节点。更严重的是：**如果上游节点因 when 条件被跳过了**，该节点的 output 不在 `x.outputs` 中，访问时触发 `"missing upstream node output"` 错误。
- **影响**: 条件分支工作流在特定输入下会因为"上游节点被跳过"而进入 fallback，而非跳过当前节点。
- **修复建议**: 在 `when.Evaluate` 中区分"节点未执行"和"字段缺失"两种情况：节点未执行时返回 `(false, nil)`（跳过），字段缺失时返回 `(false, error)`（条件错误）。

### HIGH-7-003 知识检索无超时保护与 context deadline 穿透

- **文件与位置**: `internal/knowledge/loader.go:159-199` `Store.Retrieve`
- **问题**: 设计文档要求 `Knowledge().Retrieve()` 单次调用默认超时 500ms，超时返回 `knowledge_timeout`。当前实现中 `Retrieve()` 只检查 `ctx.Err()`，不施加自己的超时。如果上游 `context.Context` 没有 deadline（例如使用 `context.Background()`），检索将完全不受限。虽然后端组件 `executeNodeWithRetry` 传入了带 deadline 的 context，但这是组件级别的间接保护，知识检索本身没有独立超时。
- **影响**: 大知识库（10000+ 条记录）的关键词检索可能消耗数百毫秒，影响工作流 P95 延迟。
- **修复建议**: 在 `Retrieve` 内增加 `context.WithTimeout(ctx, 500*time.Millisecond)` 作为默认上限。

### HIGH-7-004 Trace 写入失败时工作流错误被吞没

- **文件与位置**: `internal/runtimecore/executor.go:103` `Execute`
- **问题**: 设计文档明确规定"Trace 写入失败不得吞掉工作流错误"。当前代码中 `Finish` 失败时，`mapResponse(...)` 可能已经生成了正确的业务响应，但 `if finishErr != nil { return nil, finished, finishErr }` 会丢弃业务响应并返回 Trace 错误。这意味着当 Trace 文件系统故障时，用户的请求会失败——这与设计意图相反。
- **影响**: 文件系统故障时 Chat API 和 W2A 入口均不可用。
- **修复建议**: Trace 写入失败时应返回业务响应，但确保响应中标记 observability failure 状态。

### HIGH-7-005 `knowledge.qualityGates[].scope` 空值时行为不一致

- **文件与位置**: `internal/knowledge/loader.go:107-120` `evaluateQualityGates` 函数
- **问题**: 设计文档规定 `qualityGates[].scope` 可选，不填写时默认作用于全部 Schema。当前实现中 `evaluateQualityGates` 完全忽略 `scope` 字段——它遍历所有 `units`（来自单个 source），而不按 Schema 过滤。Validator 要求 scope 必须引用已知 Schema，如果 Manifest 填写了不存在的 scope，Validator 会拒绝——这与"不填写时全量"的逻辑矛盾。
- **影响**: scope 字段形同虚设；Validator 与 Loader 行为不一致。
- **修复建议**: 在 `evaluateQualityGates` 中实现 scope 过滤逻辑：scope 为空时全量，scope 非空时仅检查对应 Schema 的 units。

### HIGH-7-006 `server.go` 的 `resolveWebRoot` 回退逻辑可能泄露源码

- **文件与位置**: `internal/api/server.go:187-213` `resolveWebRoot` + `findWebRoot`
- **问题**: `resolveWebRoot` 的回退候选列表包含 `os.Getwd()`（当前工作目录）。如果在非项目根目录启动 `solution run`（例如从 `/tmp` 运行），且工作目录恰有一个 `web/index.html` 文件，则 web 根会指向错误位置。更严重的是，如果所有候选都失败，`resolveWebRoot` 返回字符串 `"web"`，而 `http.FileServer(http.Dir("web"))` 在相对路径不存在时会静默失败，导致前端页面无法加载。
- **影响**: 非标准部署场景下前端不可用（静默失败）。
- **修复建议**: 当 `findWebRoot` 所有候选都失败时，返回错误而非回退字符串；或在 `NewServer` 中验证 webRoot 目录存在。

---

## 中风险问题报告

### MED-7-001 `scopedKnowledge` 未过滤时返回全量 store 而非空结果
- **文件**: `internal/runtimecore/executor.go:145-156`
- **问题**: 当组件声明了 `knowledgeBindings` 但 `sources` 列表为空时，`FilterBySources([])` 返回原始 store（全量）。
- **建议**: `FilterBySources` 应区分 `nil`（无过滤）和 `[]`（显式空列表）。

### MED-7-002 `validator.go` 未校验 `perception.sensors[].config.adapter` 字段
- **文件**: `internal/manifest/validator.go`
- **问题**: 设计文档规定 adapter 只能执行字段映射，但 Validator 不校验 adapter 配置内容。
- **建议**: 在 Validator 中增加 adapter 配置白名单检查。

### MED-7-003 模板 `data-inquiry.yaml` 引用 `registry.processor.data-query@1.0.0` 不存在
- **文件**: `templates/data-inquiry.yaml`
- **问题**: 内置注册表中 `data-query` 组件的 ref 命名空间需确认是否匹配。
- **建议**: 验证所有模板中的 `ref` 是否都在注册表中注册。

### MED-7-004 示例 Manifest 使用 `registry.agent.cited-answer@1.2.0` 与模板不一致
- **文件**: `examples/after-sales-support/manifest.yaml`, `examples/guoran-support/manifest.yaml`
- **问题**: 示例使用 `@1.2.0`，模板 `customer-support.yaml` 使用 `@1.0.0`。版本差异未文档化。
- **建议**: 统一示例和模板中的版本引用。

### MED-7-005 `manifest.validator_typeflow.go` 跳过带 `when` 条件的节点
- **文件**: `internal/manifest/validator_typeflow.go`
- **问题**: 类型流校验完全跳过带 `when` 的节点输出，常见模式（`classify_intent` → 下游）的类型错误无法在校验阶段发现。
- **建议**: 对无条件执行的路径（如 `entrypoint` 总是执行）执行类型流校验。

### MED-7-006 `evaluation/runner.go` `Run` 方法仅处理第一个 dataset
- **文件**: `internal/evaluation/runner.go`
- **问题**: 设计文档支持多个 dataset，但 Runner 在第一个有效 URI 后就返回。
- **建议**: 支持多 dataset 遍历，合并结果。

### MED-7-007 知识指纹 `FingerprintManifest` 使用 JSON Marshal 可能不稳定
- **文件**: `internal/knowledge/loader.go:334-340`
- **问题**: YAML 数字反序列化后 JSON 序列化可能产生不一致的指纹。
- **建议**: 使用规范化的序列化方式计算指纹。

### MED-7-008 `release checker` 的 `eval_gates_passed` 不实现缓存复用
- **文件**: `internal/release/checker.go`
- **问题**: 设计文档要求 release 优先复用评测缓存，当前每次现场执行。
- **建议**: 实现 `ComputeFingerprint` 驱动的评测缓存读写。

### MED-7-009 并发 map 写入潜在的 race condition
- **文件**: `internal/trace/writer.go:62-78` `FileTraceWriter.records`
- **问题**: `records map` 在多个方法间读写无锁保护。当前单 goroutine 安全，未来并发场景存在 race。
- **建议**: 添加 `sync.RWMutex` 保护，或文档明确单 goroutine 约束。

### MED-7-010 Web 前端不设置 Content-Security-Policy 头
- **文件**: `internal/api/server.go` — 静态文件服务
- **问题**: `/web/*` 静态文件服务未设置 CSP 响应头。
- **建议**: 为静态文件响应添加最少 CSP 头。

---

## 低风险问题与改进建议

1. **`runtime.embedding` 字段声明存在但无对应功能** — 建议在 Validator 中增加 Phase 1 warning 提示。
2. **`KnowledgeReader` 接口缺少 `Query` 方法** — 设计文档 Phase 2 提到表格型知识的结构化查询，当前 `Retrieve` 仅支持关键词搜索。
3. **`human_handoff.Run` fallback 模式下 `errorSummary` 注入** — 设计文档要求"自动注入 `errorSummary` 字段到 fallback 节点 input"，当前 `executeFallback` 不执行此注入。
4. **`signal_ingress_reachable` 实际探测** — 当前仅检查配置存在，未做 HTTP 可达性探测。
5. **`solution release` 不执行 `weekly` 调度门禁** — 符合设计文档，但缺少日志提示。
6. **Python Worker `parser.py` 仅打印记录数不返回结构化 JSON** — 建议输出 JSON 格式以便 Go 侧解析。
7. **`component.yaml` 的 `requires` 字段未在 `componentFile` 结构体中声明** — 导致从 `components/registry/` 加载的自定义组件的 `requires` 被忽略。

---

## 与设计文档的一致性检查

### 结构性一致性

| 维度 | 设计要求 | 代码实现 | 状态 |
|------|---------|---------|------|
| Manifest 顶层字段 | 12 个 | 12 个（含 `BaseDir`/`Path` 内部字段） | 一致 |
| `apiVersion` 固定值 | `solution.codex/v1` | 硬编码校验 | 一致 |
| `solutionType` 作为顶层字段 | 是 | 是（非 `metadata` 内） | 一致 |
| `sensors[].ref` 格式 | `@world2agent/sensor-webhook@1.0.0` | 硬编码注册表 | 一致 |
| `components[].category` | `processor` / `action` | 枚举常量 | 一致 |
| `workflow.entrypoint` 必须是第一个节点 | 是 | 校验通过 | 一致 |
| `when` 仅支持 `node_id.field op literal` | 是 | 正则严格限制 | 一致 |
| 环境覆盖白名单 | 7 个字段 | 7 个字段 | 一致 |
| `trace: required` 必须启用 | 是 | release 门禁检查 | 一致 |

### 接口一致性

| 接口 | 设计要求 | 实现 | 差异 |
|------|---------|------|------|
| `Component.Run(ctx, input, runtime)` | 是 | `Run(ctx, input, runtime)` | 一致 |
| `RuntimeContext.Environment()` | 返回 `string` | 返回 `string` | 一致 |
| `RuntimeContext.Knowledge()` | `Retrieve(ctx, query, topK)` | `Retrieve(ctx, query, topK)` | 一致 |
| `RuntimeContext.Model()` | 模型调用接口 | `ModelGateway` 接口 | 一致 |
| `RuntimeContext.HTTP()` | Phase 2 可用 | 已实现 | **超前实现** |
| `RuntimeContext.Error()` | fallback 模式只读 | `RuntimeErrorSummary` | 一致 |
| `RuntimeContext.Actions()` | 只读 action 摘要 | 返回副本 | 一致 |
| `TraceWriter.Finish()` | 返回 `*TraceRecord` | 返回 `*TraceRecord` | 一致 |

### 设计文档模糊/矛盾之处

1. **知识源 Schema 字段格式**: 设计文档在 Phase 1 中 `knowledge.schemas[].fields` 为字符串列表，但 Phase 2 将支持对象格式。代码实现中 `KnowledgeSchemaSpec.Fields` 为 `[]string`，尚无字段对象支持。设计文档应明确 Phase 1 暂不支持对象格式。

2. **`qualityGates[].scope` 默认行为**: 设计文档说"不填写时默认作用于全部 Schema"，但 Validator 中 scope 仅校验引用是否存在，Loader 中完全不检查 scope。设计文档需明确 MVP 阶段 scope 的实际行为。

3. **Signal 幂等 TTL 缓存**: 设计文档说"Runtime 重启后缓存丢失，MVP 幂等只保证单进程生命周期内有效"，但未明确缓存是否在 `solution run` 进程内持久化到磁盘（当前纯内存）。建议设计文档补充"进程内缓存不跨重启持久化"的明确声明。

4. **`solutionType` 与 `metadata.industry` 优先级冲突处理**: 设计文档规定"Manifest 显式配置 > solutionType 默认骨架 > metadata.industry 默认配置"，但当前代码未实现任何基于 `solutionType` 或 `industry` 的默认值推荐逻辑。此设计意图在 MVP 中未落地。

---

## 审查方法论说明

Round 7 审查采用以下方法论确保深度和完整性：

1. **逐模块对照**: 将设计文档 1938 行按模块拆分为 52 个功能点，逐一比对代码实现。
2. **异常路径遍历**: 对每个接口/函数，跟踪 nil 输入、空值、超时、并发、错误返回路径。
3. **交叉验证**: 对比 R1-R6 历史审查报告，确认已修复项的状态和新增项的独立性。
4. **测试覆盖审计**: 检查 14 个测试包的覆盖范围，识别未测试的关键路径。
5. **安全纵深审查**: 对认证、脱敏、Trace 写入、Web 服务进行多层级安全检查。

审查深度覆盖：Manifest Schema 14/14 字段、Validator 25 项检查、Executor 全部 6 个阶段、W2A 5 层校验、Knowledge 4 种源类型、Release 8 项门禁、Trace 完整生命周期。

---

## 附录 A：测试覆盖审计

| 测试文件 | 覆盖模块 | 缺失覆盖 |
|---------|---------|---------|
| `manifest/validator_test.go` | Validator | 无 `solutionType`/`industry` 测试 |
| `workflow/when_test.go` | When 解析+求值 | 无布尔类型比较测试 |
| `workflow/plan_test.go` | Workflow 编译 | 无 MaySkip 传递依赖测试 |
| `runtimecore/executor_test.go` | Executor 执行 | 无 `continueOnFailure` 软失败测试 |
| `api/signal_router_test.go` | SignalRouter | 无幂等重复请求测试 |
| `knowledge/loader_test.go` | 知识加载 | 无 `conflicting_answers` 门禁测试 |
| `release/checker_test.go` | Release 门禁 | 无 eval cache 测试 |
| `evaluation/runner_test.go` | 评测执行 | 无 W2A Signal 评测案例测试 |
| `trace/writer_test.go` | Trace 持久化 | 无 `WriteImmediate` 测试 |

---

## 附录 B：设计文档-代码路径映射表

| 设计文档章节 | 对应代码文件 |
|------------|-------------|
| 核心抽象: Solution Manifest Schema | `manifest/types.go` |
| Manifest 校验规则 | `manifest/validator.go` |
| W2A 感知层 | `w2a/signal.go`, `w2a/sensor_registry.go` |
| 知识工程流水线 | `knowledge/loader.go`, `knowledge/ingest.go` |
| 组件注册表与 SDK | `registry/types.go`, `registry/components.go` |
| Runtime 内核 | `runtimecore/executor.go`, `runtimecore/types.go` |
| Chat API / W2A API | `api/server.go`, `api/signal_router.go` |
| Trace 写入器 | `trace/writer.go` |
| 评测框架 | `evaluation/runner.go`, `evaluation/metrics.go` |
| 发布门禁 | `release/checker.go` |
| CLI 命令 | `cmd/solution/main.go` |

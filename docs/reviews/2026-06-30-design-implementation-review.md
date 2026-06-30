# 2026-06-30 Design Implementation Review

## 审查摘要

审查基准：

- `docs/solution-as-code-fde-platform-design.md`
- `docs/solution-as-code-fde-platform-technical-architecture.md`
- `docs/development-plan.md`
- `docs/specs/*`

审查范围：

- 只分析 `git ls-files` 中已提交源码、示例、模板和项目文档。
- 忽略第三方依赖、自动生成代码、运行期生成文件。
- 未跟踪的 `docs/reviews/` 和 `docs/superpowers/plans/2026-06-30-design-review-remediation.md` 不计入项目实现完成度。

验证边界：

- 本地 Go 使用 `.tools/go1.21.13/go/bin/go.exe`。
- 已执行：`& '.\.tools\go1.21.13\go\bin\go.exe' version; & '.\.tools\go1.21.13\go\bin\go.exe' test ./cmd/... ./internal/...`
- 结果：`go version go1.21.13 windows/amd64`，`cmd` 和 `internal` 包测试通过。
- 已复现 `solution component-publish` 最小合法组件目录失败，错误为 `parse component.yaml: yaml parsing not available in marketplace`。
- 已复现 `solution release <example manifest> --env poc` 在凭据齐全时可通过检查，但生成目录仅包含 `.env.example`、`docker-compose.yaml`、`README.md`，不包含 compose 所需 `manifest.yaml`、`data`、`Dockerfile` 或可运行镜像上下文。

整体结论：

- 当前工程已具备可运行 PoC 和一批自动化测试，但仍未达到详细设计文档定义的 MVP/Phase 4 生产交付目标。
- 主要差距集中在：组件发布、交付包可启动性、真实模型网关接入、release 只读门禁语义、Manifest schema 一致性、Logger 脱敏。
- 工程整体完成度估算为 **56%**。
- 已发现致命 Bug **3 个**，高风险问题 **3 个**。

完成度计算依据：

| 领域 | 权重 | 当前得分 | 说明 |
|---|---:|---:|---|
| Manifest 与校验 | 15 | 9 | 基础结构和多数引用校验存在，缺 `solutionType`、`apiVersion` 兼容校验、`releaseChecks` 白名单校验 |
| Runtime/W2A/Trace | 20 | 15 | 基础运行、W2A、TraceWriter 可用，Logger 未脱敏，真实模型未接入 |
| Knowledge | 15 | 10 | JSONL/CSV/table/rules 可加载，Python Worker/Markdown/PostgreSQL 未闭环 |
| Component/SDK | 15 | 7 | 内置组件较完整，自定义组件发布和执行链路不完整 |
| Evaluation/Release Gate | 15 | 8 | 评测和 release gate 有基础实现，指标和 cache/fingerprint 语义不足，release 会生成质量报告 |
| Delivery/Marketplace | 15 | 3 | release CLI 存在，但部署包不可启动；component publish 直接失败；复用统计为占位 |
| Templates/Examples/Docs | 5 | 4 | 示例和模板存在，仍缺 `solutionType` 等设计字段 |
| 合计 | 100 | 56 | 整体完成度约 56% |

## 完成度详情

### 已完整实现的功能模块

| 模块 | 完成情况 | 主要文件 |
|---|---|---|
| CLI 基础入口 | `validate`、`ingest`、`evaluate`、`release`、`run --template`、`component-publish` 命令均已注册 | `cmd/solution/main.go` |
| W2A Webhook 基础流 | 鉴权、协议归一化、sensor 匹配、signal type 白名单、幂等、拒绝 trace 已实现 | `internal/api/signal_router.go`、`internal/w2a/*` |
| TraceWriter | trace 文件写入、span 追加、原子 rename、PII/secret 基础脱敏、`raw_payload` 省略已实现 | `internal/trace/writer.go`、`internal/trace/redactor.go` |
| Knowledge Loader | JSONL/CSV/table/rules 读取、source hash、质量报告、内存 keyword retrieve 已实现 | `internal/knowledge/loader.go` |
| Workflow Runtime 基础 | 线性节点执行、简单路径映射、retry、fallback、continueOnFailure、response mapping 已实现 | `internal/runtimecore/executor.go`、`internal/workflow/*` |
| Release Checker 骨架 | model/sensor/action/signal/knowledge/eval/observability/security 检查项存在 | `internal/release/checker.go` |

### 部分实现的功能

| 功能 | 缺失细节 | 设计位置 | 优先级推测 |
|---|---|---|---|
| Manifest schema | 缺顶层 `solutionType`；`apiVersion` 只校验非空；`delivery.releaseChecks` 未校验未知项；示例缺 `solutionType` | 详细设计 `MVP Manifest Schema`、`Manifest 校验规则` | P0 |
| Model Gateway | `internal/model/openai.go` 已实现 provider，但 app/runtime/evaluate/release 固定使用 `model.NewMockGateway()`，不读取 env 凭据创建真实 gateway | 详细设计 `RuntimeContext 能力清单`、`model.generate` | P0 |
| Evaluation 指标 | 只有 `citation_coverage`、`answer_accuracy`，缺按 `solutionType` 区分的 `result_accuracy`、`escalation_precision`；release eval cache/fingerprint 未实现 | 详细设计 Phase 3、`评测指标按 solutionType 区分` | P1 |
| Release Gate | checker 会读质量报告并执行 onRelease gate，但 `ReleaseManifestFile` 先调用 `knowledge.Load` 生成报告，违反 release 只检查不修复；checker 固定执行所有内置检查，未按 `releaseChecks` 声明过滤 | 详细设计 `发布检查最小标准` | P0 |
| Component Registry | 可扫描 `components/registry/component.yaml` 描述符，但只支持已内置 factory，不能加载任意自定义 Go 组件实现 | 详细设计 `Component SDK`、`组件发现层级` | P1 |
| Trace/Logger | TraceWriter 已脱敏，但 `RuntimeContext.Logger()` 仍原样输出 fields；未严格按 `logInputs`/`logOutputs` 控制写入 | 详细设计 `TraceWriter`、`Logger()` | P0/P1 |
| Knowledge ingest | Python Worker 文件存在，但 Go ingest 对 Markdown 只返回 warning；PostgreSQL 知识单元持久化未实现 | 详细设计 Phase 2 | P2 |
| 组件复用统计 | `ComputeReuseStats` 固定返回零值 | 详细设计 Phase 4 | P2 |

### 完全未实现或当前不可用的功能

| 功能 | 缺失表现 | 设计位置 | 优先级推测 |
|---|---|---|---|
| 团队组件发布 | `solution component-publish` 暴露了 CLI，但 YAML parser 是固定错误 stub，命令必定失败 | Phase 4 `solution component publish` | P0 |
| 可启动部署包 | release 成功后目录只有 `.env.example`、`docker-compose.yaml`、`README.md`；compose 引用的 `manifest.yaml`、`data` 和 build 上下文不存在 | Phase 4 `solution release` | P0 |
| 真实模型生产运行 | 运行时仍使用 mock provider，AI 输出不会来自配置模型 | `model.generate`、RuntimeContext | P0 |
| PostgreSQL/Redis 生产级幂等 | 当前 W2A 幂等为进程内 TTL，重启或多副本丢失 | 详细设计单实例约束说明 | P2 |
| PDF/Word/Markdown 到 JSONL 正式流水线 | Python worker 有原型，未从 Go CLI 接入 | Phase 2 Python Worker | P2 |

## 设计一致性检查

### 代码结构

- 目录结构与技术架构大体一致，核心模块已拆分到 `cmd/solution`、`internal/manifest`、`internal/runtimecore`、`internal/api`、`internal/knowledge`、`internal/evaluation`、`internal/release`、`internal/trace`、`internal/registry`。
- 实现状态明显偏 PoC：多处 M4 API 已暴露，但底层仍为占位或半成品，如 component publish、delivery artifacts、reuse stats。

### 接口签名和数据模型

- `SolutionManifest` 缺 `SolutionType string`，与详细设计中“强类型 Manifest 结构，含 `solutionType` 字段”不一致。
- `RuntimeContext` 暴露 `Model()`、`HTTP()`、`Logger()`，但 `Logger()` 未执行脱敏，`Model()` 当前生产入口仍接 mock。
- `TraceRecord` 和 `TraceSpan` 支持 input/output/error，但写入策略未绑定 Manifest 中 `runtime.observability.logInputs/logOutputs`。

### 关键算法和流程

- W2A 拒绝路径已避免写入 raw payload，符合安全方向。
- `knowledge_quality_passed` checker 校验报告 TTL、fingerprint、block findings，但 release 入口会先调用 loader 写新报告，破坏“release 只检查不修复”流程。
- `eval_gates_passed` 会现场执行 onRelease block gate，但没有设计要求的 eval cache/fingerprint 复用机制。
- Delivery 生成逻辑只写 compose/env/readme，不形成可执行包。

### 设计文档自身模糊或矛盾

- 示例 manifest 中 `releaseChecks: []` 与 Phase 4 “发布检查必须阻断生产发布”目标存在语义冲突。需要明确空数组是“跳过全部”还是“使用默认检查”。
- 自定义组件发布后的执行方式不明确：当前 Go 运行时不能安全动态加载任意 Go 源码，设计需说明是只发布契约、通过内置 factory 执行，还是引入 plugin/sidecar/进程隔离。
- `solutionType` 明确是顶层字段，但部分已提交 examples/templates 缺失该字段，容易造成实现偏差。
- 模型供应商 endpoint、fallback provider、OpenAI-compatible base URL 在 Manifest/env 覆盖中的字段边界不够清晰。

## 致命 Bug 报告

### BUG-001 `solution component-publish` 永远失败

严重等级：致命。

所在文件与函数：

- `internal/registry/marketplace.go`
- `PublishComponent`
- `yamlUnmarshalFunc`

触发条件与复现路径：

1. 创建任意包含 `component.yaml` 的组件目录。
2. `component.yaml` 内容只要包含合法 `ref`，例如：

   ```yaml
   ref: registry.test.echo@1.0.0
   ```

3. 执行：

   ```powershell
   & '.\.tools\go1.21.13\go\bin\go.exe' run ./cmd/solution component-publish <component-dir>
   ```

4. 实际结果：

   ```text
   Error: parse component.yaml: yaml parsing not available in marketplace
   ```

具体原因：

- `PublishComponent` 读取 `component.yaml` 后调用 `yamlUnmarshal`。
- `yamlUnmarshalFunc` 是固定返回错误的 stub，没有使用 `gopkg.in/yaml.v3` 或任何可用解析器。
- 这与设计中 Phase 4 `solution component publish` 用于团队共享组件的要求直接冲突。

影响范围：

- 团队组件共享、组件市场、后续方案复用链路完全不可用。
- 属于核心业务流程完全阻塞。

修复建议：

- 直接引入 `gopkg.in/yaml.v3` 解析 `component.yaml`。
- 校验 `ref`、`category`、`factory`、schema 字段。
- 按设计生成 `<name>-<version>.tar.gz`，并复制/发布到 `$SOLUTION_HOME/components/registry/<namespace>/<name>/<version>/`。
- 增加 `internal/registry/marketplace_test.go`，覆盖成功发布、缺 ref、非法 yaml、归档内容。

### BUG-002 release 生成的部署目录不可启动

严重等级：致命。

所在文件与函数：

- `internal/delivery/docker_compose.go`
- `GenerateDockerCompose`
- `generateComposeContent`
- `internal/app/app.go`
- `ReleaseManifestFile`

触发条件与复现路径：

1. 配置示例所需环境变量。
2. 执行：

   ```powershell
   & '.\.tools\go1.21.13\go\bin\go.exe' run ./cmd/solution release examples/after-sales-support/manifest.yaml --env poc
   ```

3. release 可通过，生成 `deploy/poc`。
4. 生成目录仅包含：

   ```text
   .env.example
   docker-compose.yaml
   README.md
   ```

5. `docker-compose.yaml` 却引用：

   ```yaml
   build: .
   volumes:
     - ./manifest.yaml:/manifest/manifest.yaml:ro
     - ./data:/manifest/data:ro
   ```

具体原因：

- `GenerateDockerCompose` 只写 compose、env example、README。
- 未复制 `manifest.yaml`、`data/`、评测/知识源快照、Dockerfile、runtime binary，也未改为使用已存在镜像。
- `build: .` 在 deploy 目录下没有有效 Docker build context。

影响范围：

- Phase 4 生产交付核心路径阻塞。
- FDE 即使看到 release passed，也无法按 README 启动服务。

修复建议：

- release 成功后生成自包含目录，至少包含 `docker-compose.yaml`、`.env.example`、`README.md`、`manifest.yaml`、`data/`，以及可构建的 Dockerfile 或明确使用预构建镜像。
- 如果使用预构建镜像，移除 `build: .` 并让 compose image 可配置。
- 增加 delivery 单元测试，检查 compose 引用的路径在 outputDir 下实际存在。
- 增加 app/release 集成测试，验证 release 后 artifact 文件集合。

### BUG-003 生产运行仍使用 mock model，AI 结果严重错误

严重等级：致命。

所在文件与函数：

- `internal/app/app.go`
- `BuildRuntime`
- `EvaluateManifestFile`
- `ReleaseManifestFile`

触发条件与复现路径：

1. Manifest 配置 `runtime.modelPolicy.defaultModel` 和环境 `modelKeyRef: env:OPENAI_API_KEY`。
2. 运行依赖 `model.generate` 的组件，例如 `registry.extractor.llm@1.0.0`。
3. 即使设置 `OPENAI_API_KEY`，app 仍注入 `model.NewMockGateway()`。
4. 实际模型输出来自 `internal/model/mock.go` 的固定 mock response。

具体原因：

- `internal/model/openai.go` 已实现 OpenAI-compatible provider，但 app 层没有根据 `ResolvedEnvironment` 构造真实 gateway。
- release checker 可以检查模型凭据，但运行时不会使用该凭据，形成“检查通过但生产不生效”的严重偏差。

影响范围：

- 所有依赖 AI 生成、结构化抽取、问答增强的业务结果可能严重错误。
- release/evaluate 与真实生产行为不一致。

修复建议：

- 新增 `internal/app` 层 gateway builder，根据 `env.ModelKeyRef` 读取 env secret。
- 默认使用 `model.NewGateway(model.NewOpenAIProvider(apiKey), fallback, env.DefaultModel, env.FallbackModel, env.MaxLatencyMs)`。
- Mock 只允许在测试或显式 dev/mock 模式使用。
- 增加 app 测试，证明设置 env 后注入非 mock provider；缺 key 时 runtime/release 按预期失败。

### BUG-004 release 入口自动生成知识质量报告，绕过只读门禁

严重等级：高。

所在文件与函数：

- `internal/app/app.go`
- `ReleaseManifestFile`
- `internal/knowledge/loader.go`
- `Load`

触发条件与复现路径：

1. 删除或缺失既有 `knowledge-quality.json`。
2. 执行 `solution release`。
3. `ReleaseManifestFile` 先调用 `knowledge.Load`，而 `Load` 会写入新的质量报告。
4. checker 随后读取刚生成的报告。

具体原因：

- 详细设计明确要求 `solution release` 只检查不修复：报告不存在、fingerprint 不匹配、过期、block finding 都必须失败。
- 当前 release 为了构造 evaluator/executor，复用了带写报告副作用的 knowledge loader。

影响范围：

- 发布门禁可能掩盖“未先执行 ingest/run 生成质量报告”的流程错误。
- 发布结果与设计的审计语义不一致。

修复建议：

- 为运行时加载新增不写报告选项，例如 `knowledge.LoadWithOptions(..., WriteReport: false)`。
- release 在 checker 前不得写质量报告。
- 如果 release 需要知识 store 做 eval，必须先通过只读质量报告检查，再用 no-report loader 构建 store。
- 增加测试：无质量报告时 release fail，且不会创建新报告。

### BUG-005 `RuntimeContext.Logger()` 可能泄露敏感字段

严重等级：高。

所在文件与函数：

- `internal/runtimecore/types.go`
- `runtimeLogger.Info`
- `runtimeLogger.Error`

触发条件与复现路径：

1. 自定义组件或未来内置组件调用：

   ```go
   runtime.Logger().Info(traceID, "call", map[string]any{
       "Authorization": "Bearer secret",
       "phone": "13800138000",
   })
   ```

2. stderr 输出原始 fields。

具体原因：

- `runtimeLogger` 直接 `fmt.Fprintf(os.Stderr, "... %v\n", fields)`。
- 未复用 `trace.RedactMap` / `trace.RedactValue`。
- 设计要求 Logger 输出必须带 `traceId`，并执行与 TraceWriter 一致的脱敏规则。

影响范围：

- 敏感 token、password、Authorization、手机号、邮箱可能进入控制台日志和采集系统。
- 属于安全基线不达标。

修复建议：

- Logger 输出前调用统一 redactor。
- 将 logger 的 traceID 参数语义理顺：优先使用 runtimeLogger 自身 traceID，避免调用者传错。
- 增加 runtimecore 或 trace 测试，覆盖 token、password、Authorization、phone、email、raw_payload。

### BUG-006 缺模型配置时 release 仍通过模型凭据检查

严重等级：高。

所在文件与函数：

- `internal/release/checker.go`
- `checkModelCredentials`

触发条件与复现路径：

1. Manifest 缺 `runtime.modelPolicy.defaultModel`。
2. 执行 release checker。
3. `checkModelCredentials` 返回 `Passed: true`，message 为 `no model configured`。

具体原因：

- 设计中模型凭据配置是生产发布检查项。
- 若没有默认模型，系统不能证明 `model.generate` 能力可用。

影响范围：

- 无模型配置的方案可能通过发布门禁。
- 与 BUG-003 叠加会使 release 结果更不可信。

修复建议：

- 缺 `DefaultModel` 时返回 `Passed: false`。
- 同时校验 `ModelKeyRef` 必须解析到非空 env secret。
- 增加 release checker 单测。

## 中低风险问题概览

| 问题 | 风险 | 建议 |
|---|---|---|
| `delivery.releaseChecks` 当前不控制 checker 执行集合 | Manifest 语义与实现不一致 | 明确空数组语义；校验未知检查项；按声明过滤或使用默认集合 |
| `solutionType` 缺失 | 模板、默认组件、评测指标无法按设计分派 | 添加字段、校验和示例迁移 |
| `apiVersion` 未校验兼容性 | 未来 schema 演进难以保护 | MVP 只接受 `solution.codex/v1` 或提供兼容迁移策略 |
| eval cache/fingerprint 未实现 | release 重复执行或误复用无保障 | 按设计补 execution/dataset/knowledge fingerprint |
| Python Worker 未接入 Go ingest | Markdown/PDF/Word 解析不可用 | 作为 Phase 2 后续任务接入 |

## 后续修复入口

后续 AI 应优先执行：

- `docs/superpowers/plans/2026-06-30-design-review-remediation.md`

推荐修复顺序：

1. P0：component publish 可用。
2. P0：release artifact 自包含且可启动。
3. P0：真实 model gateway 接入，mock 仅用于测试。
4. P0：release 只读质量门禁。
5. P0/P1：Logger 脱敏和模型配置检查。
6. P1：Manifest `solutionType`、`apiVersion`、`releaseChecks` 一致性。
7. P1/P2：评测指标、eval cache、自定义组件执行、Python Worker、复用统计。

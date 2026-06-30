# 2026-06-30 Round 3 Design Implementation Review

## 审查摘要

审查基准：

- `docs/solution-as-code-fde-platform-design.md`
- `docs/solution-as-code-fde-platform-technical-architecture.md`
- `docs/development-plan.md`
- `docs/specs/*`

审查范围：

- 按用户要求，只分析受版本控制的项目源码、示例、模板和项目文档。
- 第 3 轮发现当前工作区同时存在 tracked 修改和 untracked Go 文件。结论区分两类事实：
  - `git ls-files` 范围内的源码：计入完成度与致命交付风险。
  - untracked 文件：不计入已提交源码完成度，但若 tracked 代码依赖 untracked 文件，会作为交付风险指出。
- 忽略第三方库、自动生成代码、运行期产物。

验证命令与结果：

| 验证项 | 命令 | 结果 |
|---|---|---|
| 全工作区测试 | `& '.\.tools\go1.21.13\go\bin\go.exe' test ./cmd/... ./internal/... -count=1` | 通过 |
| 全工作区复跑 | 同上 | 复跑时本地 Go/vet 在 Windows 上触发 `0xc0000005` 崩溃；随后 `-vet=off` 复跑通过，判断为工具链/环境边界，不作为业务测试失败 |
| 全工作区测试（关闭 vet） | `& '.\.tools\go1.21.13\go\bin\go.exe' test -vet=off ./cmd/... ./internal/... -count=1` | 通过 |
| tracked-only 测试 | 临时目录只复制 `git ls-files` 文件后运行 `go test ./cmd/... ./internal/...` | 失败：`internal/app/app.go` 引用未跟踪的 `buildModelGateway` |
| 示例 validate | `solution validate examples/after-sales-support/manifest.yaml`、`solution validate examples/guoran-support/manifest.yaml` | 通过 |
| component publish smoke | 临时 `component.yaml` + `component.go` 执行 `solution component-publish` | 通过，生成 tar.gz |
| release smoke | 复制 `examples/after-sales-support`，设置 env，执行 `solution release --env poc` | 通过，生成 `manifest.yaml`、`data/`、compose、env、README |
| 路径逃逸复现 | `knowledge.sources[].uri: ../secret.txt`，先 ingest 再 release | validate 通过，release 将方案目录外文件复制到 `deploy/secret.txt` |

整体结论：

- 第 3 轮相较第 2 轮有明显进展：`component-publish`、release 自包含基础文件、`solutionType`、`apiVersion`、Logger 脱敏、部分评测指标、模型网关接入均已有实现迹象。
- 但当前“已提交源码范围”仍不可构建：tracked 的 `internal/app/app.go` 依赖 untracked 的 `internal/app/model_gateway.go`。
- 新增严重安全问题：知识源相对路径未限制在 Manifest 目录内，`../` 可导致 validate 放行并在 release 中复制方案目录外文件。
- 工程整体完成度按当前工作区能力估算为 **72%**；按“已提交源码可交付性”折减为 **60%**，因为 tracked-only 构建失败。
- 致命 Bug **2 个**，高风险问题 **5 个**。

完成度计算依据：

| 领域 | 权重 | 当前工作区得分 | tracked-only 折减后 | 说明 |
|---|---:|---:|---:|---|
| Manifest 与校验 | 15 | 12 | 12 | `solutionType`、`apiVersion`、`releaseChecks` 校验已补；缺路径安全校验 |
| Runtime/W2A/Trace | 20 | 16 | 10 | Logger 脱敏已补；tracked-only 因 model gateway helper 未跟踪导致 app 构建失败 |
| Knowledge | 15 | 10 | 10 | loader/report/fingerprint 基础存在；Python Worker/Markdown/PostgreSQL 未闭环 |
| Component/SDK | 15 | 9 | 9 | component publish 可打包；自定义组件执行 SDK 仍不完整 |
| Evaluation/Release Gate | 15 | 11 | 10 | onRelease gate 可执行；缺 cache/fingerprint，release 仍先加载知识 |
| Delivery/Marketplace | 15 | 10 | 7 | release 产物更完整，但路径逃逸和 image/runtime 说明仍不足 |
| Templates/Examples/Docs | 5 | 4 | 2 | 示例已更新，但 tracked-only 构建失败影响可用性 |
| 合计 | 100 | 72 | 60 | 当前工作区 72%；可提交源码折减后 60% |

## 完成度详情

### 已完整实现或本轮已显著修复的模块

| 模块 | 状态 | 证据 |
|---|---|---|
| CLI 基础命令 | 完整 | `validate`、`ingest`、`evaluate`、`release`、`run --template`、`component-publish` 均注册在 `cmd/solution/main.go` |
| 示例 Manifest 基础合法性 | 完整 | 两个 `examples/*/manifest.yaml` validate 通过，已包含 `solutionType` 与 `solution.codex/v1` |
| Component publish 基础打包 | 基本完整 | `internal/registry/marketplace.go` 使用 `gopkg.in/yaml.v3` 解析，smoke 成功生成 tar.gz |
| TraceWriter 与 Logger 脱敏 | 基本完整 | Trace redactor 已用于 TraceWriter；`runtimeLogger` 已调用 `trace.RedactMap` |
| W2A Webhook 基础流 | 完整 | 鉴权、归一化、sensor 匹配、signal type 白名单、幂等、拒绝 trace 已实现 |
| Release 基础门禁 | 部分完整 | model/sensor/action/signal/knowledge/eval/observability/security 检查项存在，可阻断失败 |
| Release 基础产物 | 部分完整 | release 产物包含 `manifest.yaml`、`data/`、compose/env/README，较第 2 轮改进明显 |

### 部分实现的功能

| 功能 | 缺失细节 | 设计位置 | 优先级 |
|---|---|---|---|
| 已提交源码构建闭环 | tracked `app.go` 引用 untracked `buildModelGateway`；只提交 tracked 修改会构建失败 | 工程交付基础 | P0 |
| 知识源路径安全 | `knowledge.sources[].uri` 可使用 `../` 指向方案目录外；validate 放行，release 复制外部文件 | Manifest 路径以 Manifest 目录为基准解析；交付包自包含 | P0 |
| Release 知识质量只读门禁 | `ReleaseManifestFile` 仍在 checker 前调用 `knowledge.LoadWithOptions(... WriteReport:false)`，虽不写报告，但会重新读取/构建知识 store；设计要求 release 先只读检查已有报告 | 发布检查最小标准 | P1 |
| Eval cache/fingerprint | `ComputeFingerprint` 存在但未用于 release cache；缺 dataset fingerprint、knowledge source fingerprint、1 小时 TTL 复用 | `eval_gates_passed` | P1 |
| 模型网关配置 | 当前 runtime 总是要求 `defaultModel` 和 API key，即使 workflow 未使用 `model.generate`；缺 OpenAI-compatible base URL/env 字段接入 | RuntimeContext/model gateway | P1 |
| Delivery runtime image | compose 使用 `image: solution-runtime:<version>`，但 release 包没有说明如何获得该镜像，也没有 Dockerfile/runtime binary | Phase 4 交付 | P1 |
| 自定义组件执行 SDK | 只能加载 descriptor 并实例化已内置 factory；不能执行方案级自定义 Go `Run()` 实现 | Component SDK、组件发现层级 | P1 |
| Knowledge ingest Python Worker | Python worker 存在，但 Go ingest 对 Markdown 只 warning，不调用 worker；PDF/Word/PostgreSQL 未实现 | Phase 2 知识流水线 | P2 |
| 组件复用统计 | `ComputeReuseStats` 仍固定零值 | Phase 4 复用率 | P2 |

### 完全未实现或当前不可用的功能

| 功能 | 缺失表现 | 设计位置 | 优先级 |
|---|---|---|---|
| 生产级 eval cache | 无缓存读写，无 execution/dataset/knowledge fingerprint 失效策略 | 详细设计 `eval_gates_passed` | P1 |
| 自定义组件运行 | `components/<name>/component.yaml + Run()` 不能自动加载执行，只能声明契约或映射到内置 factory | 详细设计 `Component SDK` | P1 |
| PostgreSQL/Redis 生产级幂等与知识持久化 | W2A 幂等仍为进程内 TTL，知识 store 为内存 | 单实例约束、Phase 2 | P2 |
| PDF/Word/Markdown 正式解析流水线 | Python worker 未从 Go CLI 接入 | Phase 2 Python Worker | P2 |

## 设计一致性检查

### 代码结构

- 目录结构与技术架构保持一致，核心模块划分清晰。
- 当前工作区状态不一致：tracked 文件和 untracked 文件共同构成“可测试工作区”。这不符合“已提交源码可独立构建”的工程交付底线。

### 接口签名和数据模型

- `SolutionManifest` 已加入 `SolutionType`，与设计一致。
- `apiVersion` 当前只接受 `solution.codex/v1`，与 MVP 设计一致，但迁移兼容策略未实现。
- `RuntimeContext.Logger()` 已脱敏，符合设计方向。
- `RuntimeContext.Model()` 接入真实 gateway 的 helper 未纳入 tracked 源码，导致交付断裂。

### 关键流程

- `solution component-publish` 从固定失败变为可打包，Phase 4 阻塞项已缓解。
- `solution release` 已能生成基础自包含文件，但路径复制逻辑未限制输出目录，存在 `../` 路径逃逸。
- `knowledge_quality_passed` 已检查报告存在、TTL、fingerprint、block findings；但 release 仍在 checker 前加载知识 store，流程顺序与“先只读检查”不完全一致。
- `eval_gates_passed` 能现场执行 onRelease block gate；仍无设计要求的 cache/fingerprint 复用。

### 设计文档自身模糊或矛盾

- `delivery.releaseChecks: []` 的语义仍需明确：当前实现将空数组视作默认全量检查，非空数组视作过滤检查。设计文档示例和发布强门禁目标需要统一描述。
- 自定义 Go 组件的运行机制仍不明确：Go 源码无法在运行时安全直接加载，设计需明确 plugin、sidecar、subprocess 或仅契约发布。
- 模型 provider 的 base URL、provider 类型、fallback provider 的 Manifest/env 字段边界仍不完整。
- 交付包中的 runtime image 来源需要设计明确：预构建镜像、Dockerfile、二进制拷贝三选一。

## 致命 Bug 报告

### BUG-R3-001 只提交受版本控制源码会构建失败

严重等级：致命。

所在文件与函数：

- `internal/app/app.go`
- `BuildRuntime`
- `EvaluateManifestFile`
- `ReleaseManifestFile`

触发条件与复现路径：

1. 只复制 `git ls-files` 中的文件到临时目录。
2. 执行：

   ```powershell
   & '.\.tools\go1.21.13\go\bin\go.exe' test ./cmd/... ./internal/...
   ```

3. 结果：

   ```text
   internal\app\app.go:122:23: undefined: buildModelGateway
   internal\app\app.go:218:23: undefined: buildModelGateway
   internal\app\app.go:271:23: undefined: buildModelGateway
   ```

具体原因：

- tracked 的 `internal/app/app.go` 已改为调用 `buildModelGateway`。
- `buildModelGateway` 定义在 untracked 文件 `internal/app/model_gateway.go`，不属于已提交源码范围。
- 用户要求“只分析已提交源码”，因此当前可交付源码是不可构建状态。

影响范围：

- CLI、app、release 均无法从干净 checkout 构建。
- CI 或后续 AI 若只拿已提交文件会立即失败。

修复建议：

- 将 `internal/app/model_gateway.go` 和对应测试纳入版本控制，或把 helper 合并进 tracked 文件。
- 增加 tracked-only 构建检查到修复验收流程。
- 最终执行 `go test ./cmd/... ./internal/... -count=1`，并确认 `git ls-files --others --exclude-standard` 不包含被生产代码依赖的 Go 文件。

### BUG-R3-002 知识源相对路径可逃逸方案目录并进入发布包

严重等级：致命。

所在文件与函数：

- `internal/delivery/docker_compose.go`
- `copyRuntimeInputs`
- `internal/manifest/validator.go`
- `Validate`

触发条件与复现路径：

1. 在 Manifest 中配置：

   ```yaml
   knowledge:
     sources:
       - id: leaked
         type: jsonl
         uri: ../secret.txt
         schema: faq
   ```

2. `solution validate manifest.yaml` 通过。
3. 执行 `solution ingest manifest.yaml` 生成质量报告。
4. 执行 `solution release manifest.yaml --env poc`。
5. 实测 release 成功，并生成：

   ```text
   deploy\secret.txt
   deploy\poc\manifest.yaml
   deploy\poc\docker-compose.yaml
   ```

具体原因：

- `copyRuntimeInputs` 用 `filepath.Join(outputDir, filepath.Clean(source.URI))` 构造目标路径。
- 当 `source.URI` 为 `../secret.txt` 时，目标路径逃逸到 `outputDir` 之外。
- Validator 没有拒绝绝对路径、`..` 路径或解析后不在 `m.BaseDir` 内的知识源。
- 设计要求相对路径以 Manifest 目录为基准解析，但不应允许交付包复制方案目录外文件。

影响范围：

- 可能把本地敏感文件复制进发布目录或覆盖发布目录外文件。
- 属于安全漏洞和数据泄露风险。

修复建议：

- 在 ManifestValidator 中校验所有路径字段：拒绝绝对路径，拒绝 `..` 逃逸，解析后必须位于 Manifest `BaseDir` 内。
- 在 delivery 复制时再次防御：目标路径必须位于 `outputDir` 内；源路径必须位于 `m.BaseDir` 内。
- 对路径校验增加单元测试和 release 复现测试。

### BUG-R3-003 releaseChecks 非空时可跳过知识与评测门禁

严重等级：高。

所在文件与函数：

- `internal/release/checker.go`
- `Checker.Run`

触发条件与复现路径：

1. Manifest 配置：

   ```yaml
   delivery:
     releaseChecks:
       - model_credentials_configured
   ```

2. 即使存在 `evaluation.gates` 或 knowledge 质量报告缺失，checker 只执行模型凭证检查。
3. `knowledge_quality_passed`、`eval_gates_passed` 不会进入报告，也不会阻断 release。

具体原因：

- `Checker.Run` 将非空 `delivery.releaseChecks` 解释为精确过滤集合。
- 详细设计 Phase 4 要求发布检查覆盖模型、sensor、action、入口、知识质量、评测、可观测性和安全基线；`releaseChecks` 的配置语义不应允许普通方案绕过 P0 门禁，至少必须有强制最小集合。

影响范围：

- FDE 可以通过配置删除知识和评测门禁，导致质量报告缺失或 citation gate 失败仍可发布。

修复建议：

- 明确 `releaseChecks` 语义：建议作为“附加/显式启用检查”，P0 强制检查不可被移除。
- 或增加 `releasePolicy` 专门控制跳过项，并要求 privileged/非生产环境。
- 至少在 validate 中拒绝生产环境缺少 `knowledge_quality_passed` 和 `eval_gates_passed` 的 releaseChecks。

### BUG-R3-004 Release 产物仍不能保证可启动 runtime image

严重等级：高。

所在文件与函数：

- `internal/delivery/docker_compose.go`
- `generateComposeContent`

触发条件与复现路径：

1. 执行 `solution release ... --env poc`。
2. 生成 compose：

   ```yaml
   image: solution-runtime:<version>
   command: ["solution", "run", "/manifest/manifest.yaml", ...]
   ```

3. 产物目录没有 Dockerfile、runtime binary，也没有说明如何构建或拉取 `solution-runtime:<version>`。

具体原因：

- 第 3 轮修复移除了 `build: .`，但没有补充镜像来源。
- 设计要求 `deploy/<env>` 是可自包含启动包，或在说明中声明等价卷挂载/重建说明。

影响范围：

- 用户按 README 执行 `docker-compose up -d` 仍可能因镜像不存在失败。

修复建议：

- 选择一种明确交付策略：
  - 生成 Dockerfile 并复制/构建 runtime；
  - 或要求 release 参数传入 image，并在 README 中声明拉取方式；
  - 或将 compose 改为使用当前项目可构建上下文并复制必要文件。
- 增加测试校验 README/compose 中的 image/build 策略一致。

### BUG-R3-005 Eval gate 缺失指标时被静默跳过

严重等级：高。

所在文件与函数：

- `internal/evaluation/runner.go`
- `Runner.Run`

触发条件与复现路径：

1. Manifest 中 gate 使用已声明 metric，但该 metric 对所有 case 都 not applicable，导致 `report.Metrics` 中没有该 metric。
2. `Runner.Run` 在 gate loop 中 `if !ok { continue }`。
3. Release checker 只看 `report.GateResults` 中失败项；缺失 gate result 不会失败。

具体原因：

- 设计要求 `evaluation.gates[].metric` 不存在或无法计算应在 validate/release 中阻断。
- 当前实现将缺失指标静默跳过。

影响范围：

- 配置错误或数据集不匹配时，onRelease block gate 可能不执行，从而 release 误通过。

修复建议：

- 对 `severity: block` 且 `schedule: onRelease` 的 gate，若 metric 缺失，生成 failed GateResult 或直接返回错误。
- 增加 release checker/runner 测试覆盖缺失 metric。

## 中低风险问题概览

| 问题 | 风险 | 建议 |
|---|---|---|
| `OpenAIProvider` 先 decode body 再检查 HTTP status | 非 200 且非成功 JSON 时错误信息不稳定 | 先读取 body/status，再按状态处理 |
| `ReportPath` 从 `filepath.Dir(tracePath)` 推导，示例 tracePath 为 `.solution/traces` 时报告在 `.solution/reports` | 与文档示例 `data/poc/reports` 不完全一致 | 明确 tracePath 默认和报告目录契约 |
| `ComputeReuseStats` 仍为占位 | Phase 4 复用指标不可用 | P2 实现 |
| `markdown` ingest 只 warning | Phase 2 文档解析不可用 | 接入 Python worker |
| action 组件输出仍可包含 `status: failed` 并由 runtime 特判 | 与设计“action 不得使用 error status，系统异常通过 error 返回”存在偏差 | 梳理 action 业务失败语义 |

## 后续修复入口

本轮新建修复计划：

- `docs/superpowers/plans/2026-06-30-round3-design-review-remediation.md`

建议优先级：

1. P0：纳入/合并 `model_gateway.go`，保证 tracked-only 构建通过。
2. P0：修复知识源路径逃逸，validate 和 delivery 双重防御。
3. P0/P1：防止 `releaseChecks` 跳过强制门禁。
4. P1：缺失 eval metric 时阻断 onRelease block gate。
5. P1：明确 release runtime image 策略。
6. P2：eval cache、Python Worker、复用统计、自定义组件执行 SDK。

<p align="center">
  <img src="https://img.shields.io/badge/status-M3%E2%86%92M4-brightgreen?style=flat-square" alt="Status: M3 complete, entering M4">
  <img src="https://img.shields.io/badge/Go-1.21-00ADD8?style=flat-square&logo=go" alt="Go 1.21">
  <img src="https://img.shields.io/badge/tests-passing-brightgreen?style=flat-square" alt="Tests passing">
  <img src="https://img.shields.io/badge/license-MIT-blue?style=flat-square" alt="MIT">
</p>

# Solution-as-Code &middot; FDE 平台

**一份 YAML 定义一个 AI 解决方案。** 从知识资产、事件感知、组件编排、工作流执行到评测门禁与交付约束，全部收敛为一份可版本化、可 diff、可审查的 Manifest。一条命令启动，浏览器里直接跑 Chat / W2A / Trace 全覆盖的验证闭环。

---

## 快速开始（3 步跑通第一个方案）

```bash
# 1. 构建
go build -o bin/solution ./cmd/solution

# 2. 校验 Manifest
./bin/solution validate examples/after-sales-support/manifest.yaml

# 3. 启动 Runtime
./bin/solution run examples/after-sales-support/manifest.yaml --env=poc --addr=127.0.0.1:8080

# 打开浏览器访问 http://127.0.0.1:8080
```

**从模板直接启动（零 YAML）：**

```bash
# 从 3 种内置模板中选一个，自动加载模板自带的方案和知识源
./bin/solution run --template customer-support --addr=127.0.0.1:8080
./bin/solution run --template data-inquiry --addr=127.0.0.1:8080
./bin/solution run --template alert-escalation --addr=127.0.0.1:8080
```

**更多命令：**

| 命令 | 用途 |
|------|------|
| `solution validate manifest.yaml` | 校验 Manifest 合法性 |
| `solution ingest manifest.yaml` | 知识摄取 + 质量门禁 |
| `solution evaluate manifest.yaml --env=poc` | Golden Case 评测 |
| `solution release manifest.yaml --env=production` | 发布检查 + 生成部署产物 |
| `solution run [manifest.yaml] --template <name>` | 启动 Runtime 服务 |
| `solution component-publish ./my-component/` | 打包自定义组件 |

**Web Console：** 启动 Runtime 后在浏览器中直接填写业务字段、实时预览 Manifest、发送 Chat/W2A 消息、查看 Trace，全程无 YAML 手工操作。[用户手册](docs/user-manual.md)带 1 小时新手向导。

**平台内置 API 端点：**

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/health` | 健康检查 |
| `GET` | `/api/runtime` | 运行时元信息 |
| `POST` | `/chat` | Chat 问答入口 |
| `POST` | `/w2a/tickets` | W2A Signal 入口（由 Manifest 声明） |
| `GET` | `/api/traces` | Trace 列表（支持 `?limit=N`） |
| `GET` | `/api/traces/{traceId}` | Trace 详情 |
| `GET` | `/web/` | Web Console |

---

## 为什么你需要这个

AI 解决方案的交付通常散落在控制台配置、胶水脚本、Prompt 模板和 FDE 的个人经验里。PoC 跑通了，但走向生产时反复遇到同样的墙：知识没有被工程化、事件接入没有统一协议、模板以整项目方式复制而非组件复用、评测和可观测性是事后补上的。

Solution-as-Code 把 **解决方案本身变成一等工程对象**。你写的不是胶水代码，而是一份声明式 Manifest；平台负责校验、执行、追踪和交付。

---

## 核心亮点

### 声明式 Manifest 驱动

整个方案收敛为一份 YAML。感知层、知识源、组件、工作流、运行策略、评测、交付环境——全部在一个文件里，可 diff、可 review、可 git 版本化。

一个完整的售后助手 Manifest 长这样：

```yaml
apiVersion: solution.codex/v1
kind: Solution
solutionType: customer-support
metadata:
  name: lecharm-support-agent
  version: 0.1.0
  industry: beverage

perception:
  sensors:
    - id: ticket_webhook
      ref: "@world2agent/sensor-webhook@1.0.0"
      signalTypes:
        - ticket.created
      config:
        endpointPath: /w2a/tickets
        authTokenRef: env:TICKET_WEBHOOK_TOKEN
  triggers:
    - id: ticket_triage
      sensor: ticket_webhook
      signalType: ticket.created
      routeTo: classify_intent

knowledge:
  sources:
    - id: product_manuals
      type: jsonl
      uri: ./data/knowledge_units.jsonl
      schema: faq
  schemas:
    - id: faq
      fields:
        - question
        - answer
        - source_ref
  qualityGates:
    - type: missing_required_fields
      severity: block
      scope:
        - faq

components:
  - id: intent_classifier
    ref: registry.intent.beverage-router@1.0.0
  - id: retriever
    ref: registry.retriever.local-keyword@1.0.0
  - id: answer_generator
    ref: registry.agent.cited-answer@1.2.0

workflow:
  entrypoint: classify_intent
  onError:
    retry: 1
    fallbackNode: handoff
  nodes:
    - id: classify_intent
      component: intent_classifier
    - id: retrieve_knowledge
      component: retriever
    - id: generate_answer
      component: answer_generator
    - id: handoff
      component: human_handoff
      when: classify_intent.confidence < 0.65
      inputs:
        message: inputs.message

runtime:
  knowledgeBindings:
    - component: retriever
      sources:
        - product_manuals
  modelPolicy:
    defaultModel: gpt-4.1
    fallbackModel: gpt-4.1-mini
    maxLatencyMs: 8000
  observability:
    trace: required
    logInputs: masked

evaluation:
  datasets:
    - id: golden_cases
      uri: ./data/eval_cases.jsonl
      caseFormat: runtime_request_jsonl
  metrics:
    - citation_coverage
    - answer_accuracy
  gates:
    - metric: citation_coverage
      min: 0.95
      severity: block
      schedule: onRelease

delivery:
  environments:
    - name: poc
      type: shared_sandbox
      config:
        modelKeyRef: env:OPENAI_API_KEY
        tracePath: ./.solution/traces
  releaseChecks:
    - model_credentials_configured
    - sensor_credentials_configured
    - observability_enabled
    - security_baseline_passed
```

### W2A 协议原生集成

平台内置 [World2Agent](https://github.com/machinepulse-ai/world2agent) 协议支持。外部系统事件（工单创建、告警触发、数据变更）通过 W2A Sensor 标准化为结构化 Signal，直接驱动工作流执行。Chat 和 W2A Signal 共享同一套 Runtime 内核，不需要两套代码。

- 完整的 W2A Signal 信封校验（`w2a/0.1`）
- 内置 Webhook Sensor，声明 endpoint 即自动暴露 HTTP 入口
- Signal 幂等去重，防止重复事件触发
- Bearer Token 认证，密钥通过 `env:VAR_NAME` 引用，不写明文

### 可组合组件注册表

内置组件覆盖售后问答全链路，每个组件有明确的输入/输出 Schema，版本 pin 到 `@x.y.z`：

| 组件 | 类型 | 说明 |
|------|------|------|
| `registry.intent.support-router@1.0.0` | processor | 通用售后意图分类器 |
| `registry.intent.beverage-router@1.0.0` | processor | 饮品行业意图分类器 |
| `registry.intent.severity-beverage@1.0.0` | processor | 严重程度判定，可配置关键词 |
| `registry.retriever.local-keyword@1.0.0` | processor | 本地关键词检索，带引用溯源 |
| `registry.agent.cited-answer@1.2.0` | processor | 带引用的答案生成，可要求强制接地 |
| `registry.processor.rule-evaluator@1.0.0` | processor | 规则条件匹配引擎 |
| `registry.processor.data-query@1.0.0` | processor | 结构化数据查询 |
| `registry.action.human-handoff@1.0.0` | action | 人工升级，自动携带失败上下文 |
| `registry.action.mock-create-service-ticket@1.0.0` | action | Mock 工单创建，可模拟故障 |
| `registry.action.http-caller@1.0.0` | action | HTTP API 调用，支持模板化请求 |

支持从 `components/registry/` 目录加载自定义组件声明（`component.yaml`），无需修改平台代码。`solution component-publish` 可将组件打包为 tar.gz 发布到团队共享仓库。

### 编译型工作流引擎

工作流在启动时编译为 DAG，而非运行时解释：

- **条件跳过** — `when` 表达式在编译期解析依赖，运行时按条件跳过节点；Validator 在编译期检测不可跳过的依赖
- **自动重试** — 全局 `onError.retry` 配置，每个节点自动重试
- **Fallback 链** — 节点失败后自动路由到 fallback 节点（如人工升级）
- **输入映射** — `inputMapping` 将 Chat/Signal 请求归一化为统一的工作流输入；节点级 `inputs` 显式声明依赖
- **类型流校验** — 节点输入在运行前按组件 InputSchema 校验，编译期检查上下游组件间的输入/输出类型兼容性

### 知识工程化，不只是 RAG

知识在这里是一等资产，不是"丢进向量库就完事"：

- 多类型知识源支持：JSONL（FAQ）、CSV（表格数据）、Markdown/PDF/Word（通过 Python Worker 转换）
- 显式 Schema 声明：每条知识源必须绑定 schema，定义字段名和类型
- 每条知识单元强制要求 `source_ref` 引用字段
- 启动时自动运行质量门禁：缺失字段、空内容、无效 JSON 直接 block
- 生成结构化质量报告 `knowledge-quality.json`，可供发布检查消费
- 检索结果始终携带 `citations`，回答可追溯到原始手册章节

### 全链路 Trace

每次 Chat 或 W2A Signal 触发都生成完整的 JSON Trace：

```json
{
  "traceId": "trace_a1b2c3d4e5f6a7b8",
  "solution": "lecharm-support-agent",
  "trigger": { "type": "chat" },
  "spans": [
    { "node": "classify_intent", "component": "registry.intent.beverage-router@1.0.0", "latencyMs": 2 },
    { "node": "retrieve_knowledge", "component": "registry.retriever.local-keyword@1.0.0", "latencyMs": 5 },
    { "node": "generate_answer", "component": "registry.agent.cited-answer@1.2.0", "latencyMs": 3 }
  ],
  "status": "success",
  "latencyMs": 12
}
```

每个 span 记录节点 ID、组件引用、尝试次数、输入输出和错误信息。Trace 持久化到文件系统，提供 `/api/traces` 列表和详情接口。入口拒绝和运行时失败均采用同一 Trace Schema，输入字段自动脱敏。

### 多模式评测与发布门禁

平台内建评测引擎，将方案质量从"感觉还行"变成可量化的门禁：

- `solution evaluate` 读取 JSONL Golden Cases，按 `solutionType` 加载默认指标集
- 问答方案自动计算 `citation_coverage`（引用覆盖率）和 `answer_accuracy`（答案准确率）
- 门禁支持 `block`（阻断发布，退出码 1）和 `warn`（仅告警，退出码 0）两种语义
- `schedule: onRelease` 的门禁在 `solution release` 时强制执行；`schedule: weekly` 仅校验不阻断
- 评测结果与 Trace 关联，可下钻到具体 case 的执行细节

### 交付打包与组件共享

从 PoC 到生产，同一份 Manifest，不改写逻辑：

- `solution release --env production` 执行 8 项发布检查（凭证配置、安全基线、知识质量、评测门禁等）
- 全部检查通过后生成 `./deploy/<env>/`，含 `docker-compose.yaml`、`.env.example` 和运行说明
- Docker Compose 启动同一 Runtime 二进制，行为与 `solution run` 等价
- `solution component-publish` 将自定义组件打包为 `<name>-<version>.tar.gz`
- 复用率统计：自动计算新方案中引用已有组件和模板的比例

### 方案模板市场

平台内置 3 个可立即运行的方案模板，覆盖典型 FDE 场景：

| 模板 | solutionType | 场景 | 入口 |
|------|-------------|------|------|
| `customer-support` | 售后客服 | FAQ 问答 + 意图分类 + 工单创建 + 人工升级 | Chat + W2A ticket webhook |
| `data-inquiry` | 数据查询 | 产品信息查询 + 库存/价格搜索 | Chat only |
| `alert-escalation` | 告警升级 | 规则评估 + HTTP 回调通知 + 人工升级 | W2A alert webhook |

选择模板 → 修改知识源路径和配置 → `solution run`，即可验证一个新的 AI 解决方案。

### 环境与交付约束

Manifest 中声明多环境配置，平台按环境名解析。支持 `env:VAR_NAME` 引用环境变量，不在 Manifest 中写明文密钥。安全策略（PII 检测、注入防御、RBAC）和发布检查在 Manifest 中声明，`solution release` 强制执行。

---

## 开发阶段

项目按 4 个里程碑推进，当前处于 **M4（交付打包与组件共享）** 初期阶段：

| 里程碑 | 阶段 | 状态 | 核心能力 |
|--------|------|------|---------|
| M1 | Manifest 解析器与 Runtime 内核 | ✅ 完成 | validate / run，Chat + W2A，工作流执行，Trace |
| M2 | 多模态知识与通用组件集 | ✅ 完成 | ingest，组件注册表，通用组件库，方案模板，Python Worker |
| M3 | 多模式评测与发布门禁 | ✅ 完成 | evaluate，Golden Cases，类型流校验，评测门禁 |
| M4 | 交付打包与组件共享 | 🔄 进行中 | release 完整链路，Docker Compose，component publish，复用统计 |

**工程完成度**：核心功能模块 ~77%（Round 5 审查结论），10 轮迭代 Bug Hunt 已修复 31 个缺陷，测试全绿。

**质量状况**：
- 全量测试通过（`go test ./cmd/... ./internal/...`）
- 31 个 Bug 已修复（含 2 个致命、16 个中危、13 个低危），修复率 100%
- 4 轮设计审查完成，P0 缺陷清零
- 路径安全跨平台防御、组件契约对齐、知识绑定过滤、发布门禁强制等关键项验证通过

> 详细计划见 [开发推进计划书](docs/development-plan.md)，Bug Hunt 总结见 [docs/bug-hunt-summary-20260701.md](docs/bug-hunt-summary-20260701.md)，审查报告见 [docs/reviews/](docs/reviews/)。

---

## 项目结构

```
fde-support/
├── cmd/solution/          # CLI 入口 (Cobra)
├── internal/
│   ├── api/               # HTTP Server, SignalRouter, Runtime/Trace API
│   ├── app/               # 应用编排与模型网关装配
│   ├── delivery/          # Docker Compose 产物生成
│   ├── environment/       # 环境解析与密钥引用
│   ├── evaluation/        # 评测引擎、指标计算、门禁检查
│   ├── knowledge/         # JSONL/CSV 知识加载、质量门禁、Python Bridge
│   ├── manifest/          # Manifest 类型定义、加载、校验、类型流校验
│   ├── model/             # 模型网关（OpenAI-compatible + mock）
│   ├── registry/          # 组件注册表、内置组件、市场机制
│   ├── release/           # 发布检查链路（8 项检查）
│   ├── runtimecore/       # 工作流执行引擎
│   ├── shared/            # 错误类型、工具函数
│   ├── trace/             # Trace 写入、列表、加载、输入脱敏
│   ├── w2a/               # W2A Signal 校验、幂等、Sensor 注册
│   └── workflow/          # 工作流编译、条件表达式、when 解析
├── web/                   # 静态 Web Console (HTML/CSS/JS)
├── templates/             # 内置方案模板（客服、数据查询、告警升级）
├── examples/              # 示例方案（乐源售后、果燃售后）
├── workers/               # Python 知识工程 Worker（PDF/Word/Markdown 解析）
├── docs/                  # 设计文档、用户故事、开发计划、规格附件、审查报告
└── bin/                   # 构建产物
```

## 知识工程与 Python Worker

知识源支持多种格式。JSONL 和 CSV 由 Go 核心直接加载；Markdown、PDF、Word 通过 Python Worker 以子进程方式调用完成转换。

```bash
# 安装 Python Worker 依赖
cd workers/knowledge && uv sync
```

Worker 将文档转换为标准化 JSONL，再由 Go 侧执行质量门禁（字段缺失、内容冲突、过期检测等）。

## 技术栈

| 层面 | 选型 |
|------|------|
| 语言 | Go 1.21 + Python 3.10+ |
| CLI | Cobra |
| HTTP Router | chi/v5 |
| 配置格式 | YAML (gopkg.in/yaml.v3) |
| 知识存储 | JSONL/CSV + 内存关键词索引 |
| Trace | 本地 JSON 文件 |
| 前端 | 纯 HTML/CSS/JS，零构建 |
| Python Worker | Python 3.10+，uv 管理依赖 |

---

## 文档索引

### 设计文档
- [平台详细设计](docs/solution-as-code-fde-platform-design.md) — 设计哲学、问题陈述、MVP 范围
- [技术架构与实施规范](docs/solution-as-code-fde-platform-technical-architecture.md) — 模块分层、技术选型、演进路径
- [用户故事文档](docs/solution-as-code-userstory.md) — 7 个用户故事覆盖完整交付流程
- [开发推进计划书](docs/development-plan.md) — M1-M4 里程碑、任务拆分、验收标准、风险评估
- [World2Agent 技术全景](docs/world2agent-introduce.md) — W2A 协议核心概念与传感器开发范式
- [实现规格附件](docs/specs/) — 知识单元格式、组件规格、Trace Schema、Golden Case 格式、类型兼容性矩阵
- [设计审查报告](docs/reviews/) — Round 1-5 设计与实现审查报告

### 运维与使用文档
- [用户手册](docs/user-manual.md) — 新手 1 小时快速上手指南
- [Bug Hunt 总结](docs/bug-hunt-summary-20260701.md) — 10 轮迭代 Bug 发现与修复记录
- [修复计划归档](docs/superpowers/plans/) — Round 3-7 设计审查修复计划

### 开发命令参考

```bash
# 测试（仅核心模块）
go test ./cmd/... ./internal/... -count=1

# 测试 + 静态检查
go vet ./...

# 验证所有官方 Manifest
go run ./cmd/solution validate examples/after-sales-support/manifest.yaml
go run ./cmd/solution validate examples/guoran-support/manifest.yaml
go run ./cmd/solution validate templates/customer-support.yaml
go run ./cmd/solution validate templates/data-inquiry.yaml
go run ./cmd/solution validate templates/alert-escalation.yaml
```

PowerShell 环境：

```powershell
$env:PATH = "C:\Users\1003584\.g\versions\1.26.1\bin;$env:PATH"
go test ./cmd/... ./internal/...
```

## License

MIT

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.21-00ADD8?style=flat-square&logo=go" alt="Go 1.21">
  <img src="https://img.shields.io/badge/license-MIT-blue?style=flat-square" alt="MIT">
</p>

# Solution-as-Code &middot; FDE 平台

**一份 YAML 定义一个 AI 解决方案。** 从知识资产、感知入口、组件编排、工作流执行到评测门禁与交付约束，全部收敛为一份可版本化、可复现、可审查的 Manifest。一条命令启动，浏览器里直接验证 Chat / W2A / Trace 闭环。

---

## 为什么你需要这个

AI 解决方案的交付通常散落在控制台配置、胶水脚本、Prompt 模板和 FDE 的个人经验里。PoC 跑通了，但走向生产时反复遇到同样的墙：知识没有被工程化、事件接入没有统一协议、模板以整项目方式复制而非组件复用、评测和可观测性是事后补上的。

Solution-as-Code 把 **解决方案本身变成一等工程对象**。你写的不是胶水代码，而是一份声明式 Manifest；平台负责校验、执行、追踪和交付。

## 核心亮点

### 声明式 Manifest 驱动

整个方案收敛为一份 YAML。感知层、知识源、组件、工作流、运行策略、评测、交付环境——全部在一个文件里，可 diff、可 review、可 git 版本化。

```yaml
apiVersion: solution.ai/v1alpha1
kind: Solution
metadata:
  name: lecharm-support-agent
  version: 0.1.0
  industry: beverage

perception:
  sensors:
    - id: ticket_webhook
      ref: "@world2agent/sensor-webhook@1.0.0"
      signalTypes: [ticket.created]
      config:
        endpointPath: /w2a/tickets
        authTokenRef: env:TICKET_WEBHOOK_TOKEN

knowledge:
  sources:
    - id: product_manuals
      type: jsonl
      uri: ./data/knowledge_units.jsonl
      schema: faq
  qualityGates:
    - type: missing_required_fields
      severity: block

components:
  - id: intent_classifier
    ref: registry.intent.beverage-router@1.0.0
  - id: retriever
    ref: registry.retriever.local-keyword@1.0.0
  - id: answer_generator
    ref: registry.agent.cited-answer@1.2.0

workflow:
  entrypoint: support_agent
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
| `registry.intent.*-router` | processor | 意图分类，支持自定义意图列表 |
| `registry.intent.severity-beverage` | processor | 严重程度判定，可配置关键词 |
| `registry.retriever.local-keyword` | processor | 本地关键词检索，带引用溯源 |
| `registry.agent.cited-answer` | processor | 带引用的答案生成，可要求强制接地 |
| `registry.action.human-handoff` | action | 人工升级，自动携带失败上下文 |
| `registry.action.mock-create-service-ticket` | action | Mock 工单创建，可模拟故障 |

支持从 `components/registry/` 目录加载自定义组件声明（`component.yaml`），无需修改平台代码。

### 编译型工作流引擎

工作流在启动时编译为 DAG，而非运行时解释：

- **条件跳过** — `when` 表达式在编译期解析依赖，运行时按条件跳过节点
- **自动重试** — 全局 `onError.retry` 配置，每个节点自动重试
- **Fallback 链** — 节点失败后自动路由到 fallback 节点（如人工升级）
- **输入映射** — `inputMapping` 将 Chat/Signal 请求归一化为统一的工作流输入；节点级 `inputs` 显式声明依赖
- **类型校验** — 节点输入在运行前按组件 InputSchema 校验，类型不匹配直接报错

### 知识工程化，不只是 RAG

知识在这里是一等资产，不是"丢进向量库就完事"：

- JSONL 知识源加载，带显式 Schema 声明
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

每个 span 记录节点 ID、组件引用、尝试次数、输入输出和错误信息。Trace 持久化到文件系统，提供 `/api/traces` 列表和详情接口。

### 浏览器控制台

平台自带一个纯静态 Web Console（零构建、零依赖），让 FDE 不碰 YAML 也能完成全流程：

- **模板选择** — 售后助手 / 知识问答 / 工单受理三种预设模板
- **表单驱动** — 填写方案名、行业、知识源路径等最少字段，右侧实时预览 Manifest
- **闭环验证** — 在同一页面发送 Chat 消息、构造 W2A Signal、查看 Trace 列表和详情
- **运行时状态** — 实时显示当前 Solution、环境、Trace 路径

### 环境与交付约束

Manifest 中声明多环境配置，平台按环境名解析：

```yaml
delivery:
  environments:
    - name: poc
      type: shared_sandbox
      config:
        modelKeyRef: env:OPENAI_API_KEY
        tracePath: ./.solution/traces
    - name: staging
      type: dedicated
      config:
        modelKeyRef: env:OPENAI_API_KEY_STAGING
```

安全策略（PII 检测、注入防御、RBAC）和发布检查在 Manifest 中声明，为后续 CI/CD 门禁预留接口。

## 快速开始

```bash
# 构建
go build -o bin/solution ./cmd/solution

# 校验 Manifest
./bin/solution validate examples/after-sales-support/manifest.yaml

# 启动 Runtime
./bin/solution run examples/after-sales-support/manifest.yaml --env=poc --addr=127.0.0.1:8080
```

打开浏览器访问 `http://127.0.0.1:8080`，进入 Web Console。

### API 端点

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/health` | 健康检查 |
| `GET` | `/api/runtime` | 运行时元信息 |
| `POST` | `/chat` | Chat 问答入口 |
| `POST` | `/w2a/tickets` | W2A Signal 入口（由 Manifest 声明） |
| `GET` | `/api/traces` | Trace 列表（支持 `?limit=N`） |
| `GET` | `/api/traces/{traceId}` | Trace 详情 |

## 项目结构

```
fde-support/
├── cmd/solution/          # CLI 入口 (Cobra)
├── internal/
│   ├── api/               # HTTP Server, SignalRouter, Runtime/Trace API
│   ├── app/               # 应用启动与编排
│   ├── environment/       # 环境解析与密钥引用
│   ├── knowledge/         # JSONL 知识加载、质量门禁、关键词检索
│   ├── manifest/          # Manifest 类型定义、加载、校验
│   ├── registry/          # 组件注册表、内置组件实现
│   ├── runtimecore/       # 工作流执行引擎
│   ├── shared/            # 错误类型、工具函数
│   ├── trace/             # Trace 写入、列表、加载
│   ├── w2a/               # W2A Signal 校验、幂等、Sensor 注册
│   └── workflow/          # 工作流编译、条件表达式、路径解析
├── web/                   # 静态 Web Console (HTML/CSS/JS)
├── examples/              # 示例 Manifest 与数据
├── docs/                  # 设计文档与用户故事
└── bin/                   # 构建产物
```

## 技术栈

| 层面 | 选型 |
|------|------|
| 语言 | Go 1.21 |
| CLI | Cobra |
| HTTP Router | chi/v5 |
| 配置格式 | YAML (gopkg.in/yaml.v3) |
| 知识存储 | JSONL + 内存关键词索引 |
| Trace | 本地 JSON 文件 |
| 前端 | 纯 HTML/CSS/JS，零构建 |

## 设计文档

- [平台详细设计](docs/solution-as-code-fde-platform-design.md) — 设计哲学、问题陈述、MVP 范围
- [技术架构与实施规范](docs/solution-as-code-fde-platform-technical-architecture.md) — 模块分层、技术选型、演进路径
- [用户故事文档](docs/solution-as-code-userstory.md) — 7 个用户故事覆盖完整交付流程
- [World2Agent 技术全景](docs/world2agent-introduce.md) — W2A 协议核心概念与传感器开发范式

## License

MIT

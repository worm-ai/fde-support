# Solution-as-Code 用户手册

> 目标：新手在 1 小时内能独立创建、运行、验证一个 AI 解决方案。
> 需要的储备：会用终端、接触过 YAML、理解 AI 问答的基本概念。

---

## 时间表

| 时段 | 章节 | 目标 |
|------|------|------|
| 0-10 分钟 | [1. 理解 Solution-as-Code](#1-理解-solution-as-code) | 搞懂这个平台解决什么问题，核心概念有哪几个 |
| 10-20 分钟 | [2. 环境准备与构建](#2-环境准备与构建) | 把二进制编出来，能跑 `solution --help` |
| 20-35 分钟 | [3. 从模板启动第一个方案](#3-从模板启动第一个方案) | 选一个模板跑起来，浏览器里看到 Web Console |
| 35-50 分钟 | [4. 编写自己的 Manifest](#4-编写自己的-manifest) | 理解 Manifest 各段的意义，从零写一份自己的方案 |
| 50-60 分钟 | [5. 评测与发布](#5-评测与发布) | 跑 Golden Case 评测，执行发布检查生成部署包 |
| 附录 | [A. 常见问题排查](#a-常见问题排查) | 遇到报错该看哪里 |
| 附录 | [B. 全部命令与参数](#b-全部命令与参数) | 每条命令的完整参数和退出码说明 |
| 附录 | [C. 内置模板速查](#c-内置模板速查) | 三个内置模板的对比和适用场景 |
| 附录 | [D. 组件注册表速查](#d-组件注册表速查) | 每个组件的输入输出和使用方式 |

---

## 1. 理解 Solution-as-Code

### 1.1 这个平台解决什么问题

FDE（现场交付工程师）面对的场景通常是这样的：客户来了，要在他的工单系统里接一个 AI 客服，能在用户投诉超标、出现产品质量问题时自动回答、自动建单、必要时呼叫人工。

传统的做法是：控制台点点点 + 写胶水脚本 + 调 Prompt + 手动测试 → PoC 跑通了，但下一次换一个品牌又要全部重来一遍。知识换了、Prompt 换了、判断逻辑换了，但代码和数据流是一样的。

Solution-as-Code 的答案是：**把整个方案变成一份声明式文件（Manifest），平台只负责执行它。**

### 1.2 核心概念（读完这 5 个就能动手）

| 概念 | 一句话 | 在 Manifest 里是 |
|------|--------|------------------|
| **Manifest** | YAML 文件，描述你的方案长什么样 | 整个文件 |
| **Component** | 最小可执行单元，如"关键词检索""意图分类" | `components:` 段 |
| **Workflow** | 把 Component 串成一条执行链 | `workflow:` 段 |
| **Knowledge** | 你的业务知识库，如 FAQ 条目 | `knowledge:` 段 |
| **感知层** | 如何接收外部事件（用户 Chat 或第三方工单回调） | `perception:` 段 |

一张图看懂它们的关系：

```
用户消息 ──→ perception: ──→ workflow: ──→ component₁ ──→ component₂ ──→ component₃ ──→ 回答
                  │                 │               │              │              │
                  │                 │               ↓              ↓              ↓
                  │                 │          knowledge:  ←−  检索知识     引用知识生成答案
                  │                 │
                  │                 └── Trace（每一步都有 JSON 记录）
                  │
                  └── W2A Signal（外部系统事件也可以从这里进来，走同一条 workflow）
```

### 1.3 一个 Manifest 长什么样子（骨架）

```yaml
apiVersion: solution.codex/v1    # 固定
kind: Solution                     # 固定
solutionType: customer-support     # 方案类型

metadata:                          # 元数据：名称、版本、负责人、行业
  name: ...; version: 0.1.0; owner: ...; industry: ...

perception:                       # 感知层：谁触发、怎么触发
  sensors: [...]; triggers: [...]

knowledge:                        # 知识库：源文件、字段定义、质量门禁
  sources: [...]; schemas: [...]; qualityGates: [...]

components:                       # 组件：方案用到的功能模块
  - id: ...; ref: registry.xxx@1.0.0; config: {...}

workflow:                         # 工作流：串起来怎么执行
  entrypoint: ...; nodes: [...]; onError: {...}

runtime:                          # 运行时：模型策略、可观测性、知识绑定
  knowledgeBindings: [...]; modelPolicy: {...}; observability: {...}

evaluation:                       # 评测：Golden Cases、指标、门禁
  datasets: [...]; metrics: [...]; gates: [...]

delivery:                         # 交付：环境配置、安全检查
  environments: [...]; releaseChecks: [...]
```

你不需要立刻记住全部字段。跟着后面的教程写一个，自然就會了。

---

## 2. 环境准备与构建

### 2.1 前提条件

| 依赖 | 版本 | 检查命令 |
|------|------|---------|
| Go | 1.21+ | `go version` |
| Python（可选） | 3.10+ | `python3 --version` |
| uv（可选，用于 Python Worker） | 最新 | `uv --version` |

如果你不做知识工程（不转换 PDF/Word/Markdown），只需要 Go。

### 2.2 构建 CLI 二进制

```bash
# 进入项目目录
cd fde-support

# 编译
go build -o bin/solution ./cmd/solution

# 验证
./bin/solution --help
```

输出应该显示 6 个子命令：`validate`、`ingest`、`evaluate`、`release`、`run`、`component-publish`。

如果编译失败，检查 `go.mod` 里的版本号，通常 `go mod tidy` 能解决。

### 2.3 安装 Python Worker（可选）

```bash
cd workers/knowledge
uv sync
```

不需要启动 Worker 进程。Go CLI 在执行 `solution ingest` 时会自动启动 Python 子进程完成文档转换。

---

## 3. 从模板启动第一个方案

这是最快见到效果的路径：选一个内置模板，`solution run --template`，浏览器打开就看到了。

### 3.1 三种可选模板

| 模板名 | 场景 | 能做什么 | 知识点 |
|--------|------|---------|--------|
| `customer-support` | 售后客服 | 用户聊天 → 意图识别 → 知识检索 → 带引用回答；工单事件 → 自动分类 → 必要时转人工 | 最完整，先从这个开始 |
| `data-inquiry` | 数据查询 | 聊天 → 查询产品/库存数据 → 返回结构化结果 | 没有 W2A 入口，只有 Chat |
| `alert-escalation` | 告警升级 | 外部系统告警 → 规则匹配 → HTTP 通知 → 必要时升级 | 只有 W2A 入口，没有 Chat |

### 3.2 启动 customer-support 模板

```bash
# 模板自带了 Manifest 和知识源，不需要额外参数
./bin/solution run --template customer-support --addr=127.0.0.1:8080
```

你会看到类似这样的输出：

```
runtime: lecharm-support-agent v0.1.0
environment: poc
chat: POST /chat
w2a webhook: http://127.0.0.1:8080/w2a/tickets
trace path: ./.solution/traces
listening on http://127.0.0.1:8080
```

打开浏览器访问 `http://127.0.0.1:8080`。

### 3.3 Web Console 操作向导

Web Console 是一个纯静态页面，分 4 个区域：

1. **左侧栏**：显示运行时状态、可选模板、当前步骤
2. **中间主区域**：填写方案表单（方案名、行业、负责人、知识源路径等）
3. **右上区域**：Manifest 实时预览 + 校验状态
4. **底部**：闭环操作——发送 Chat、构造 W2A Signal、查看 Trace

**推荐操作顺序：**

1. 左侧选择"售后助手"模板 → 中间表单自动填充
2. 右下角校验区确认 6 项全绿（Ready）
3. 底部 Chat 区域输入"葡萄汁出现沉淀怎么处理？"→ 点击发送，看到带引用的回答
4. 底部 W2A 区域点击发送，模拟一个工单事件
5. 底部 Trace 区域刷新，点击每条 Trace 查看执行链路

至此，你已经体验了一个 AI 售后问答方案的完整闭环：Chat 问答 + W2A 事件 + Trace 追踪。

### 3.4 手动用 curl 验证

```bash
# 健康检查
curl http://127.0.0.1:8080/health

# 查看运行时信息
curl http://127.0.0.1:8080/api/runtime | python3 -m json.tool

# Chat 问答
curl -X POST http://127.0.0.1:8080/chat \
  -H "Content-Type: application/json" \
  -d '{"message":"葡萄汁出现沉淀怎么处理？"}'

# W2A Signal（模拟工单事件）
curl -X POST http://127.0.0.1:8080/w2a/tickets \
  -H "Content-Type: application/json" \
  -d '{
    "schema_version": "w2a/0.1",
    "signal_id": "test-001",
    "emitted_at": "2026-07-01T12:00:00Z",
    "source": {
      "sensor_id": "ticket_webhook",
      "sensor_version": "1.0.0",
      "package": "@world2agent/sensor-webhook",
      "source_type": "ticket-system"
    },
    "event": {
      "type": "ticket.created",
      "occurred_at": "2026-07-01T12:00:00Z",
      "summary": "客户反馈产品沉淀问题"
    },
    "source_event": {
      "data": {
        "ticketId": "T-001",
        "productModel": "Grape-Classic",
        "description": "葡萄汁出现大量沉淀，客户询问是否还能饮用"
      }
    }
  }'

# 查看 Trace 列表
curl http://127.0.0.1:8080/api/traces?limit=5 | python3 -m json.tool

# 查看某条 Trace 详情
curl http://127.0.0.1:8080/api/traces/trace_b9215b63572c5b8e | python3 -m json.tool
```

---

## 4. 编写自己的 Manifest

你已经见过模板怎么跑了。现在从零写一个属于你自己的方案。

### 4.1 从复制示例开始

```bash
# 复制乐源售后方案作为起点
cp examples/after-sales-support/manifest.yaml ./my-support/manifest.yaml
mkdir -p ./my-support/data

# 复制知识源（或准备你自己的）
cp examples/after-sales-support/data/lecharm/knowledge_units.jsonl ./my-support/data/

# 先校验是否可以正确解析
./bin/solution validate ./my-support/manifest.yaml
```

看到 `manifest valid` 就可以继续了。

### 4.2 逐段理解 Manifest

#### 4.2.1 metadata（元数据）

```yaml
metadata:
  name: my-support-agent          # 方案名称
  version: 0.1.0                  # 语义化版本
  owner: my-name                  # 负责人
  industry: beverage              # 行业（影响意图分类器的行为）
```

`name` 会在运行时信息中显示，`version` 会进入 Trace 和部署产物。

#### 4.2.2 perception（感知层）

```yaml
perception:
  sensors:
    - id: ticket_webhook
      ref: "@world2agent/sensor-webhook@1.0.0"  # W2A 传感器类型
      signalTypes:                                # 这个 Sensor 接收哪些信号
        - ticket.created
      config:
        endpointPath: /w2a/tickets               # HTTP 路径
        authTokenRef: env:TICKET_WEBHOOK_TOKEN   # 引用环境变量，不写明文
  triggers:
    - id: ticket_triage
      sensor: ticket_webhook       # 绑定的 sensor id
      signalType: ticket.created   # 收到这个信号类型时
      routeTo: classify_intent     # 路由到 workflow 的哪个节点
```

要点：
- `endpointPath` 以 `/` 开头，会自动注册为 HTTP 路由
- `authTokenRef` 用 `env:XXX` 引用环境变量，不暴露密码
- 每个 trigger 把一种 signal type 映射到一个 workflow 节点

#### 4.2.3 knowledge（知识源）

```yaml
knowledge:
  sources:
    - id: product_manuals          # 知识源 ID，在 runtime.knowledgeBindings 中引用
      type: jsonl                  # 类型：jsonl | csv | markdown | pdf | word
      uri: ./data/my-knowledge.jsonl  # 文件路径（相对于 Manifest）
      schema: faq                  # 使用哪个 schema
  schemas:
    - id: faq
      fields:
        - question                 # 问题
        - answer                   # 答案
        - product_model            # 产品型号
        - source_ref               # 来源引用（必填）
  qualityGates:
    - type: missing_required_fields  # 门禁类型
      severity: block              # block = 不通过就不能跑
      scope:
        - faq
```

**知识源格式示例**（JSONL，每行一条）：

```jsonl
{"source_id":"product_manuals","question":"葡萄汁沉淀是什么原因","answer":"葡萄汁中的天然果胶在低温或长期静置时可能形成沉淀，属于正常物理现象，不影响饮用。","product_model":"Grape-Classic","source_ref":"product_manual_v2_ch3"}
{"source_id":"product_manuals","question":"产品保质期多久","answer":"未开封保质期12个月，开封后建议冷藏并在3天内饮用完毕。","product_model":"All","source_ref":"product_manual_v2_ch1"}
```

要点：
- `source_ref` 是强制字段，回答会带着这个引用返回给用户
- JSONL 里每条的 `source_id` 必须匹配 `knowledge.sources[].id`
- 质量门禁在 `solution ingest` 和 `solution run` 时都会执行

#### 4.2.4 components（组件）

```yaml
components:
  - id: intent_classifier
    category: processor                           # processor | action
    ref: registry.intent.beverage-router@1.0.0    # 组件引用
    config:
      intents:                                     # 组件配置
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
    ref: registry.agent.cited-answer@1.0.0
    config:
      style: concise
      requireGrounding: true
  - id: human_handoff
    category: action
    ref: registry.action.human-handoff@1.0.0
    config:
      queue: support-l2
```

要点：
- `processor` 组件产生数据输出（如分类结果、检索结果、生成的回答）
- `action` 组件执行副作用（如创建工单、转人工、调 HTTP 接口）
- `ref` 格式是 `registry.<namespace>.<name>@<version>`，版本必须精确匹配
- `config` 因组件而异，详情查[附录 D](#d-组件注册表速查)

#### 4.2.5 workflow（工作流）

```yaml
workflow:
  entrypoint: classify_intent      # 从哪个节点开始

  onError:                         # 全局错误处理
    retry: 1                       # 失败重试次数
    fallbackNode: handoff          # 全部失败后跳转到哪个节点

  inputMapping:                    # 将不同的入口消息归一化
    chat:
      message: request.message     # Chat 入口：message 取自请求 body 的 message 字段
    w2a_signal:
      message: signal.source_event.data.description  # W2A 入口：message 取自 signal 的描述
      ticketId: signal.source_event.data.ticketId
      signalSummary: signal.event.summary

  nodes:
    - id: classify_intent          # 第一个节点
      component: intent_classifier # 使用 component id
      inputs:
        message: inputs.message    # 把输入映射里的 message 传给组件

    - id: retrieve_knowledge       # 第二个节点
      component: retriever
      inputs:
        query: inputs.message

    - id: generate_answer          # 第三个节点
      component: answer_generator
      inputs:
        message: inputs.message
        passages: retrieve_knowledge.passages       # 引用上游节点输出
        citations: retrieve_knowledge.citations

    - id: handoff                  # 人工升级节点
      component: human_handoff
      when: classify_intent.confidence < 0.65       # 条件跳过：意图分类置信度低时才执行
      inputs:
        message: inputs.message
```

要点：
- `inputs` 是工作流级别的变量池，`inputMapping` 往里填，节点从里面取
- 节点引用上游输出：`nodeId.fieldName`
- `when` 条件在编译期检查依赖关系，运行时按条件跳过
- 被 `when` 跳过的节点，下游不能引用它的输出（Validator 会拦截）

#### 4.2.6 runtime（运行时配置）

```yaml
runtime:
  knowledgeBindings:              # 知识绑定：哪个组件用哪些知识源
    - component: retriever
      sources:
        - product_manuals         # 对应 knowledge.sources[].id

  modelPolicy:
    defaultModel: gpt-4.1         # 默认模型
    fallbackModel: gpt-4.1-mini   # 备用模型
    maxLatencyMs: 8000            # 最大延迟
    maxCostPerRunUsd: 0.05        # 单次最大成本

  observability:
    trace: required               # 开启 Trace
    logInputs: masked             # 输入脱敏后记录
    logOutputs: true              # 输出记录
    retainDays: 7                 # Trace 保留天数
```

#### 4.2.7 evaluation（评测）

```yaml
evaluation:
  datasets:
    - id: golden_cases
      uri: ./data/eval_cases.jsonl     # Golden Case 文件
      caseFormat: runtime_request_jsonl  # 就是 Chat/W2A 请求的格式
  metrics:
    - citation_coverage                # 引用覆盖率
    - answer_accuracy                  # 答案准确率
  gates:
    - metric: citation_coverage
      min: 0.95                        # 达到 95% 才通过
      severity: block                  # 不通过阻断发布
      schedule: onRelease              # 在 release 时执行
```

**Golden Case 示例**（`eval_cases.jsonl`）：

```jsonl
{"request": {"method":"POST","path":"/chat","body":{"message":"葡萄汁出现沉淀怎么处理？"}},"expected":{"citations":["product_manual_v2_ch3"],"answerContains":["正常物理现象","不影响饮用"]}}
```

#### 4.2.8 delivery（交付环境）

```yaml
delivery:
  environments:
    - name: poc                        # PoC 环境
      type: shared_sandbox
      config:
        modelKeyRef: env:OPENAI_API_KEY
        tracePath: ./.solution/traces
        retainDays: 7
    - name: production                 # 生产环境
      type: dedicated
      config:
        modelKeyRef: env:OPENAI_API_KEY_PROD
        tracePath: /data/traces
        retainDays: 30
  security:
    piiDetection: required
    promptInjectionDefense: required
    rbac: required
  releaseChecks:
    - model_credentials_configured
    - sensor_credentials_configured
    - action_credentials_configured
    - observability_enabled
    - security_baseline_passed
```

### 4.3 从零到上线流程图

```
1. 确定场景 ──→ 2. 准备 JSONL 知识源
                    ↓
               3. 选模板/手写 Manifest
                    ↓
               4. solution validate   ← 回到 3 如果失败
                    ↓
               5. solution ingest     ← 知识摄取 + 质量门禁
                    ↓
               6. solution run        ← 启动 Runtime，浏览器验证
                    ↓
               7. 准备 eval_cases.jsonl
                    ↓
               8. solution evaluate   ← 跑评测，看指标
                    ↓
               9. solution release    ← 发布检查，生成部署包
```

### 4.4 复用方案到新品牌

假设你已经为品牌 A（乐源）写了一套售后问答方案，现在要为品牌 B（果燃）做一个几乎一样的。

**需要改什么：**

1. 知识源文件：换成果燃的 FAQ JSONL
2. Config 里的品牌关键词：`intents` 和 `keywords` 按需调整
3. Sensor ID 和 Sensor 名称：避免冲突
4. 评测 Golden Cases：换成针对果燃的

**不需要改的：**

- 组件引用（`registry.intent.beverage-router@1.0.0` 等）不变
- Workflow 节点和连线不变
- 环境配置模板基本不变

这就是平台复用率的来源：一份 Manifest 到另一份，通常只换知识源和少数配置项，组件和 workflow 可以原样复用。`examples/guoran-support/` 就是一个从乐源复制后调整了知识源和 Sensor 的实例。

---

## 5. 评测与发布

### 5.1 运行评测

```bash
# 需要先准备好 eval_cases.jsonl（至少 5-10 条）
./bin/solution evaluate examples/guoran-support/manifest.yaml --env=poc
```

输出示例：

```
evaluate guoran-support-agent: 8/10 cases passed
  citation_coverage: 0.9200
  answer_accuracy: 0.8000
  gate citation_coverage: FAIL (0.9200 >= 0.9500, block)
  gate answer_accuracy: PASS (0.8000 >= 0.7000, warn)
```

如果 `citation_coverage` 门禁是 `severity: block`，评测会返回退出码 1。

### 5.2 执行发布检查

```bash
./bin/solution release examples/after-sales-support/manifest.yaml --env=production
```

输出示例：

```
release production: passed=true
  model_credentials_configured: PASS model credential is configured
  sensor_credentials_configured: PASS sensor credential is configured
  action_credentials_configured: WARN action credentials may be missing; check env
  signal_ingress_reachable: PASS
  knowledge_quality_passed: PASS quality report is valid
  eval_gates_passed: PASS all onRelease block gates passed
  observability_enabled: PASS trace is enabled
  security_baseline_passed: WARN security baseline partially configured
```

全部 block 级检查通过后，生成 `./deploy/production/` 目录，包含：

```
./deploy/production/
├── docker-compose.yaml    # Docker Compose 部署文件
├── .env.example           # 环境变量模板
└── README.md              # 部署说明
```

### 5.3 发布自定义组件

如果你写了一个新的组件 `my-special-classifier`，可以打包发布：

```bash
./bin/solution component-publish ./components/my-special-classifier/
# 输出：component artifact generated at my-special-classifier-1.0.0.tar.gz
```

组件发布前会校验 `component.yaml` 的格式是否合法。

---

## 6. 进阶话题

### 6.1 知识工程 Python Worker

当你需要从 PDF、Word 或 Markdown 中提取知识时，Go CLI 会调用 Python Worker：

```bash
cd workers/knowledge
uv sync
cd ../..
./bin/solution ingest my-support/manifest.yaml --env=poc
```

Worker 行为：
- Go 将文件路径传给 Python 子进程
- Python 将文档转换为 JSONL 写入标准输出
- Python 退出码 0 表示成功；非 0 表示失败
- Go 读取输出并运行质量门禁

### 6.2 创建自定义组件

在项目根目录下创建 `components/registry/<namespace>/<name>/<version>/component.yaml`：

```yaml
apiVersion: component.codex/v1
kind: Component
metadata:
  name: my-custom-processor
  namespace: custom
  version: 1.0.0
spec:
  category: processor
  description: 我的自定义处理器
  inputSchema:
    message: string
  outputSchema:
    result: string
  requires:
    - model.generate
  configSchema:
    language:
      type: string
      default: zh
```

然后在 Manifest 中引用：`ref: registry.custom.my-custom-processor@1.0.0`

**注意**：自定义 Go 组件（用 Go 写的 `Run()` 方法）的自动执行机制还在开发中。

### 6.3 路径安全

Manifest 中的所有文件路径（`knowledge.sources[].uri`、`delivery.environments[].config.tracePath` 等）都会被校验：
- 禁止绝对路径（如 `/etc/passwd`、`C:\secret.txt`）
- 禁止 `..` 路径穿越（如 `../../secret.txt`）
- 只允许相对于 Manifest 所在目录的相对路径

如果在 `solution validate` 时收到路径相关的错误，检查 `uri` 是否写了绝对路径或 `..`。

---

## A. 常见问题排查

### A.1 `solution validate` 失败

| 错误信息 | 原因 | 解决 |
|---------|------|------|
| `unknown field` | Manifest 里有后端不认识的字段 | 检查拼写；也可能是版本不匹配 |
| `component ref not found` | 组件引用拼错或版本不存在 | 检查 `components[].ref` 是否在[附录 D](#d-组件注册表速查)里 |
| `node references undefined component` | workflow 节点引用了不存在的 component id | 确保 `workflow.nodes[].component` 等于 `components[].id` |
| `environment "" not found` | `--env` 指定的环境在 Manifest 里不存在 | 检查 `delivery.environments[].name` |
| `path not allowed` | 文件路径包含绝对路径或 `..` | 改用相对 Manifest 目录的路径 |
| `dataflow: node X references output of skippable node Y` | 下游节点引用了可能被 `when` 跳过的上游节点 | 去掉引用，或确保上游不会被跳过 |

### A.2 `solution run` 失败

| 错误信息 | 原因 | 解决 |
|---------|------|------|
| `OPENAI_API_KEY not set` | 环境变量缺失 | `export OPENAI_API_KEY=sk-...` |
| `knowledge source not readable` | 知识源文件路径不对或不存在 | 检查 `knowledge.sources[].uri` 相对于 Manifest 的路径 |
| `signal port already in use` | 端口被占用 | 换一个端口 `--addr=127.0.0.1:8081` |
| `quality gate blocked` | 知识质量门禁不通过 | 检查知识源 JSONL 格式，`solution ingest` 单独跑看报告 |

### A.3 Chat 请求没返回预期的回答

1. 检查知识源里是否有匹配的关键词。当前检索是关键词匹配，不是语义检索
2. 检查 `runtime.knowledgeBindings` 是否把正确的知识源绑到了 `retriever` 组件上
3. 检查 `components[].config.requireCitation` 是否为 `true`；如果知识源没设 `source_ref` 字段，引用会缺失
4. 打开浏览器 `/web/`，查看 Trace 详情，检查每个 Span 的输出

### A.4 W2A Signal 返回 401 或 400

| 状态码 | 原因 | 解决 |
|--------|------|------|
| 401 | Bearer Token 不匹配 | 检查 `perception.sensors[].config.authTokenRef` 指向的环境变量 |
| 400 | Signal 格式不合法 | 检查 `schema_version` 必须是 `w2a/0.1`，`emitted_at` 和 `occurred_at` 必须是 RFC3339 格式 |
| 409 | 重复的 `signal_id` | Signal 已经处理过了（幂等去重） |

---

## B. 全部命令与参数

### `solution validate`

```
solution validate <manifest.yaml>

校验 Manifest 合法性。8 阶段校验：结构 → Schema → 引用 → 密钥 → 工作流语法 → 数据流 → 组件契约 → 知识Schema。

选项：
  --json    输出 JSON 格式的校验结果

退出码：
  0    全部通过
  1    存在错误
```

### `solution ingest`

```
solution ingest <manifest.yaml>

加载知识源并运行质量门禁。会根据知识源 type 调用对应的加载器：
jsonl → Go 直接解析；markdown/pdf/word → Python Worker 子进程。

选项：
  --json    输出 JSON 格式的摄取报告

退出码：
  0    全部通过或仅有 warn
  1    存在 block 级失败
```

### `solution run`

```
solution run [manifest.yaml] [--template <name>]

启动 Runtime HTTP 服务。若指定 --template，则从 templates/ 加载模板方案。

选项：
  --env <name>       交付环境名称（默认 poc）
  --addr <host:port> HTTP 监听地址（默认 127.0.0.1:8080）
  --template <name>  模板名称：customer-support | data-inquiry | alert-escalation

快捷键：
  Ctrl+C    优雅关闭
```

### `solution evaluate`

```
solution evaluate <manifest.yaml>

对 Manifest 指定的 Golden Cases 执行评测，输出指标和门禁状态。

选项：
  --env <name>       交付环境名称（默认 poc）
  --json             输出 JSON 格式的评测报告

退出码：
  0    全部通过或仅有 warn
  1    存在 block 级失败（且 schedule=onRelease）
```

### `solution release`

```
solution release <manifest.yaml>

执行 8 项发布检查，全部通过后生成 ./deploy/<env>/ 部署产物。

选项：
  --env <name>       目标环境（默认 production）
  --json             输出 JSON 格式的发布报告

退出码：
  0    全部 block 级检查通过
  1    任一 block 级检查失败
```

### `solution component-publish`

```
solution component-publish <component-dir>

将自定义组件打包为 tar.gz 发布包。

选项：
  --json    输出 JSON 格式的结果

退出码：
  0    打包成功
  1    校验失败或打包错误
```

---

## C. 内置模板速查

### C.1 customer-support（售后客服）

**场景：** FAQ 知识问答 + 意图分类（故障排查/保修/投诉/转人工）+ 工单创建（mock）+ 人工升级。

**入口：** Chat（`POST /chat`）+ W2A ticket webhook（`POST /w2a/tickets`）

**工作流：** `classify_intent` → `retrieve_knowledge` → `generate_answer` → 低置信度时 `handoff`

**适用：** 这是最完整的模板，几乎所有售后场景都从这里开始改。

### C.2 data-inquiry（数据查询）

**场景：** 结构化产品数据查询（库存、价格、规格），无意图分类、无人工升级。

**入口：** Chat only（`POST /chat`）

**工作流：** `query_data` → `format_result`

**适用：** 单纯的数据查询场景（"这个产品多少钱？""还有库存吗？"）

### C.3 alert-escalation（告警升级）

**场景：** 外部监控系统通过 W2A 发告警 → 规则匹配（严重/中/低）→ HTTP 通知 → 严重时人工升级。

**入口：** W2A alert webhook only（`POST /w2a/alerts`）

**工作流：** `evaluate_rule` → `notify`（HTTP 回调）→ `handoff`（严重时）

**适用：** 监控告警的自动响应和分级处理。

---

## D. 组件注册表速查

### 内置 Component（processor 类型）

| ref | input | output | config |
|-----|-------|--------|--------|
| `registry.intent.support-router@1.0.0` | `message: string` | `intent: string, confidence: number` | `intents: [string]` 意图列表 |
| `registry.intent.beverage-router@1.0.0` | `message: string` | `intent: string, confidence: number` | `intents: [string]` 意图列表 |
| `registry.intent.severity-beverage@1.0.0` | `message: string` | `severity: string, confidence: number` | `keywords: {severe:[], moderate:[]}` |
| `registry.retriever.local-keyword@1.0.0` | `query: string` | `passages: [string], citations: [string]` | `topK: number, requireCitation: boolean` |
| `registry.agent.cited-answer@1.0.0` | `message: string, passages: [string], citations: [string]` | `answer: string, citations: [string]` | `style: "concise"|"detailed", requireGrounding: boolean` |
| `registry.processor.llm-extractor@1.0.0` | `text: string` | `extracted: string?` | `prompt: string, fields: [string]` |
| `registry.processor.data-query@1.0.0` | `query: string` | `rows: array, count: number, citations: array` | `table: string` |
| `registry.processor.rule-evaluator@1.0.0` | `message: string` | `matched: boolean, result: string?` | `rules: [{condition: string, action: string}]` |

### 内置 Component（action 类型）

| ref | input | output | config |
|-----|-------|--------|--------|
| `registry.action.human-handoff@1.0.0` | `message: string` | `action: string, target: string` | `queue: string` |
| `registry.action.mock-create-service-ticket@1.0.0` | `message: string` | `ticketId: string` | `system: string, apiKeyRef: string` |
| `registry.action.http-caller@1.0.0` | `url: string` | `status: number, body: string` | `method: string, headers: object` |

### 组件引用格式

```
registry.<namespace>.<name>@<major>.<minor>.<patch>
```

版本必须精确匹配。不支持 `@1.x` 或 `@latest` 这类模糊匹配。

### 自定义组件

在 `components/registry/<namespace>/<name>/<version>/component.yaml` 创建声明文件后，Manifest 中可以这样引用：

```yaml
components:
  - id: my_processor
    ref: registry.custom.my-processor@1.0.0
```

---

## 附录：术语表

| 术语 | 全称/解释 |
|------|----------|
| **Manifest** | 声明式方案描述文件，YAML 格式 |
| **Component** | 平台可执行的最小功能单元 |
| **Workflow** | 将多个 Component 串为有向无环图 (DAG) |
| **W2A** | World2Agent 协议，外部系统事件标准化为结构化 Signal |
| **Signal** | W2A 协议定义的事件信封，如 `ticket.created` |
| **Trace** | 每次执行的全链路 JSON 日志，每个 Span 对应一个节点 |
| **Span** | Trace 中的一个执行步骤，记录节点 ID、组件、耗时、输入输出 |
| **Golden Case** | 标准评测用例，JSONL 格式，包含输入和期望输出 |
| **Quality Gate** | 质量门禁，block（阻断）或 warn（告警） |
| **FDE** | Field Development Engineer，现场交付工程师 |
| **PoC** | Proof of Concept，概念验证 |
| **DAG** | Directed Acyclic Graph，有向无环图 |

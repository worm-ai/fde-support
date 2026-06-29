# 用户故事文档：Solution-as-Code FDE 平台

本文档以“乐源饮品售后智能助手”交付全过程为例，采用标准用户故事模板，展示 FDE 周远如何通过平台定义、验证、交付并持续演化一个 AI 解决方案。每个故事均包含核心的 Manifest YAML 变更和操作步骤，确保过程可复现。

---

## 跨故事约束

- 所有 Manifest 示例必须能通过 `solution validate`，不能引用未声明的组件、节点、知识源、Sensor 或环境。
- 所有敏感配置必须使用 `env:VAR_NAME` 或后续支持的密钥引用格式，不能在 Manifest 中写明文密钥。
- Chat 与 W2A Signal 触发路径必须先归一化为标准 `RuntimeRequest`，再进入同一个 WorkflowExecutor。
- `workflow.entrypoint` 必须是 `workflow.nodes` 中第一个节点的 `id`；`perception.triggers[].routeTo` 必须等于 `workflow.entrypoint`。
- `workflow.inputMapping` 在 MVP 中只支持简单字段路径；必填映射字段缺失时，SignalRouter 返回 400，不进入工作流，也不触发 fallback。
- 工作流节点必须通过 `workflow.nodes[].inputs` 显式声明组件输入；组件不能隐式读取整个 `step_outputs`。
- 组件不可恢复的系统异常必须通过返回值 `(nil, error)` 抛出；action 组件的可恢复业务失败通过 `output["status"] = "failed"` 表达，禁止使用 `"error"` 作为 status 值。processor 组件的输出不包含 `status` 字段。
- MVP 中的 W2A Sensor 采用 Runtime 内置 Webhook 入口模型：Runtime 根据 `perception.sensors[].config.endpointPath` 暴露 HTTP 入口并校验认证；独立 Sensor 进程或 SensorHub 集成属于后续能力。
- `@world2agent/sensor-webhook@1.0.0` 是 MVP `SensorRegistry` 的内置 Sensor 引用，不要求从 npm 安装，也不通过 `ComponentRegistry` 解析；版本必须显式 pin。
- `RuntimeContext.Model()` 在 Phase 1 提供基于环境密钥的最小模型网关实现（如调用 OpenAI API）；`HTTP()` 能力在 Phase 2 可用。
- W2A Sensor 只负责把外部事件标准化为 W2A Signal，不负责业务决策、工作流编排、RAG、动作执行或发布治理。
- Phase 1 只验证 Runtime 内核、统一组件接口（含 `llm-classifier`、`retriever`、`llm-generator`、`human-handoff`）、mock action、Trace、W2A Signal 入口和启动时 JSONL 知识加载；真实知识摄取流水线、真实外部系统调用、知识工作台和增量索引不进入 Phase 1。
- Phase 1 检索组件统一使用 `registry.retriever.local-keyword@1.0.0`；混合检索和向量检索属于 Phase 2+。
- Manifest 中引用的组件版本号必须与平台内置注册表中已注册的版本一致；Phase 1 内置组件版本均为 `@1.0.0`。
- 所有组件的 citations 统一使用 `string[]` 格式（`"source#ref"`），由知识加载时统一生成该格式的引用字符串。
- `knowledge.sources[].uri`、`evaluation.datasets[].uri` 等相对路径均以 Manifest 文件所在目录为基准解析。
- `sessionId` 在 MVP 中仅用于请求关联，不提供跨轮会话记忆。

---

## 故事列表

| 故事ID | 标题 | 优先级 | 故事点 | 所属模块 |
|--------|------|--------|--------|----------|
| STORY-001 | 通过声明式 Manifest 快速创建售后问答 PoC | P0 | 5 | Solution 定义与运行时 |
| STORY-002 | 用标准评测集量化方案质量并设置发布门禁 | P0 | 3 | 评测体系 |
| STORY-003 | 接入工单系统事件，由业务信号触发工作流 | P0 | 5 | W2A 感知层 |
| STORY-004 | 用 mock action 自动创建紧急工单 | P0 | 3 | 组件与工作流 |
| STORY-005 | 从 PoC 平滑发布到生产环境 | P0 | 5 | 约束性交付框架 |
| STORY-006a | 手动触发知识摄取并执行质量门禁 | P1 | 3 | 知识工程流水线 |
| STORY-006b | 知识变更自动化与专家工作台 | P2 | 5 | 知识工程流水线 |
| STORY-007 | 复用于新品牌——资产复用与快速交付 | P1 | 3 | 可组合资产与模板 |

---

## STORY-001：通过声明式 Manifest 快速创建售后问答 PoC

**基础信息**

- 故事ID：STORY-001
- 故事标题：通过声明式 Manifest 快速创建售后问答 PoC
- 所属模块：Solution 定义与运行时
- 优先级：P0
- 故事点：5
- 负责人：FDE 周远，平台 Runtime 团队
- 计划迭代：MVP Phase 1

**核心用户故事**

> 作为 **FDE**，
> 我想要 **编写一份包含知识源、组件引用和工作流的 Manifest 文件，并执行一条命令启动可交互的问答服务**，
> 以便 **在客户现场一小时内将预处理知识单元转化为可演示、带引用溯源的 PoC，无需编写胶水代码**。

**验收标准**

1. **正常场景：从零构建 PoC**
   - Given 客户提供已预处理好的 JSONL 知识单元文件 `./data/lecharm/knowledge_units.jsonl`
   - When FDE 编写如下 Manifest 文件 `lecharm-support.yaml` 并执行 `solution validate lecharm-support.yaml` 和 `solution run lecharm-support.yaml --env=poc`
   - Then 平台启动 HTTP 服务，可通过 `POST /chat` 发送 `{"message":"葡萄汁出现沉淀怎么处理？"}`
   - And 返回的 JSON 中包含 `answer`、`intent`、`confidence`、`citations` 字段，`citations` 指向手册具体章节，且 `traceId` 不为空
   - And 同一目录下的 `data/poc/traces/` 中生成对应的 JSON Trace 文件
   - And 同一运行目录下生成最小知识质量报告 `data/poc/reports/knowledge-quality.json`

2. **异常场景：组件引用或必填字段缺失**
   - Given Manifest 中 `components[].ref` 指向不存在的组件或版本
   - When 执行 `solution validate`
   - Then 校验失败，输出明确的错误信息，指出无法解析的组件引用
   - Given 缺少 `metadata.name` 字段
   - When 执行 `solution validate`
   - Then 校验失败，提示缺少必填字段

3. **边界场景：知识源文件为空**
   - Given 声明的 JSONL 知识源文件存在但没有任何知识单元
   - When 执行 `solution run`
   - Then 服务可启动，但回答问题时应返回“当前知识库为空”的降级提示，并写入 Trace，而非报错崩溃

4. **异常场景：知识单元缺少引用字段**
   - Given JSONL 知识源中某条非空记录缺少默认引用字段 `source_ref`
   - When 执行 `solution run`
   - Then 启动失败，输出 `knowledge_quality_passed` 相关的 block 级错误，并在 `data/poc/reports/knowledge-quality.json` 中记录缺失字段

**完成定义**

- [ ] FDE 能按模板编写 Manifest，通过 `validate` 校验
- [ ] `solution run` 后 `POST /chat` 返回符合 Schema 的响应
- [ ] 回答中包含引用来源，并写入 Trace
- [ ] Phase 1 JSONL 加载会校验可检索文本字段和 `source_ref`，并生成可供 release check 消费的最小质量报告
- [ ] 删除 `data/poc` 后重新运行，可重现完全相同的服务状态
- [ ] 本故事所涉及的 Manifest 示例作为官方 QuickStart 文档

**核心 Manifest 片段（初次 PoC）**

```yaml
apiVersion: solution.codex/v1
kind: Solution
metadata:
  name: lecharm-support-agent
  version: 0.1.0
  owner: fde-zhouyuan
  industry: beverage
  solutionType: customer-support

knowledge:
  sources:
    - id: product_manuals
      type: jsonl
      uri: ./data/lecharm/knowledge_units.jsonl
      schema: faq
  schemas:
    - id: faq
      fields:
        - symptom
        - cause
        - resolution
        - product_model
        - source_ref

components:
  - id: intent_classifier
    category: processor
    ref: registry.intent.beverage-router@1.0.0
    config:
      intents: [troubleshooting, warranty, complaint, human_handoff]
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

workflow:
  entrypoint: classify_intent
  onError:
    retry: 1
    fallbackNode: handoff
  inputMapping:
    chat:
      message: request.message
  nodes:
    - id: classify_intent
      component: intent_classifier
      inputs:
        message: inputs.message
    - id: retrieve_knowledge
      component: retriever
      inputs:
        query: inputs.message
    - id: generate_answer
      component: answer_generator
      inputs:
        message: inputs.message
        passages: retrieve_knowledge.passages
        citations: retrieve_knowledge.citations
    - id: handoff
      component: human_handoff
      when: classify_intent.confidence < 0.65
      inputs:
        message: inputs.message

runtime:
  knowledgeBindings:
    - component: retriever
      sources: [product_manuals]
  modelPolicy:
    defaultModel: gpt-4.1
    fallbackModel: gpt-4.1-mini
    maxLatencyMs: 8000
  observability:
    trace: required
    logInputs: masked
    retainDays: 30

delivery:
  environments:
    - name: poc
      type: shared_sandbox
      config:
        modelKeyRef: env:OPENAI_API_KEY
        tracePath: ./data/poc/traces
        retainDays: 7
```

---

## STORY-002：用标准评测集量化方案质量并设置发布门禁

**基础信息**

- 故事ID：STORY-002
- 故事标题：用标准评测集量化方案质量并设置发布门禁
- 所属模块：评测体系
- 优先级：P0
- 故事点：3
- 负责人：FDE 周远，评测引擎团队
- 计划迭代：MVP Phase 3

**核心用户故事**

> 作为 **FDE**，
> 我想要 **将客户验证过的标准问答对定义为 Golden Cases，并在 Manifest 中声明评测指标和门禁**，
> 以便 **在 PoC 阶段就用数据（citation_coverage、answer_accuracy）向客户证明方案质量，且发布前自动阻断不达标的方案**。

**验收标准**

1. **正常场景：执行评测并生成报告**
   - Given 已准备好 30 条 Golden Cases 文件 `./data/lecharm/evals/golden.jsonl`，Manifest 的 `evaluation` 段已配置
   - When 执行 `solution evaluate lecharm-support.yaml --env=poc`
   - Then 控制台输出 `citation_coverage` 和 `answer_accuracy` 的数值，并显示是否通过门禁；`severity: block` 失败返回退出码 1，`severity: warn` 失败返回退出码 0 但输出告警
   - And `--json` 输出包含 `warnings` 数组和 `warnings_exist: true|false` 字段，供 CI 机器读取告警状态
   - And 对于未通过的 case，输出具体 case ID 和预期/实际差异摘要

2. **异常场景：门禁未通过时阻断发布**
   - Given `citation_coverage` 实际值为 0.92，低于门禁 0.95
   - When 执行 `solution release lecharm-support.yaml --env=production`
   - Then 发布命令中断，输出 `eval_gates_passed: FAILED` 及未通过指标详情
   - And `schedule: weekly` 的监控性门禁不参与本次 release 阻断

3. **边界场景：评测集为空**
   - Given `datasets` 指向的 JSONL 文件为空
   - When 执行 `solution evaluate`
   - Then 提示“评测数据集为空，请至少提供一条 Golden Case”，并返回非零退出码

**完成定义**

- [ ] Golden Case 的 JSONL 格式文档已提供，并支持 `runtime_request_jsonl` 输入模型；Golden Case 中的 `expected.intent` 必须在组件声明的 `intents` 列表内
- [ ] `solution evaluate` 命令在 Phase 3 可用，输出结果可读，且 `--json` 提供 `warnings` 与 `warnings_exist`
- [ ] 门禁阻断功能有效，未通过时禁止后续流程
- [ ] 评测结果与 Trace 数据关联，可下钻分析

**Manifest 新增片段**

```yaml
evaluation:
  datasets:
    - id: golden_v1
      uri: ./data/lecharm/evals/golden.jsonl
      caseFormat: runtime_request_jsonl
  metrics:
    - citation_coverage
    - answer_accuracy
  gates:
    - metric: citation_coverage
      min: 0.95
      severity: block
      schedule: onRelease
    - metric: answer_accuracy
      min: 0.85
      severity: warn
      schedule: weekly
```

Golden Case 示例（`golden.jsonl` 中的一条）：
```json
{
  "id": "case_01",
  "trigger": {"type": "chat"},
  "request": {"message": "葡萄汁出现沉淀怎么处理？"},
  "raw_payload": {"message": "葡萄汁出现沉淀怎么处理？"},
  "expected": {
    "intent": "complaint",
    "mustCite": true,
    "answerContains": ["摇匀", "絮状", "品控"]
  }
}
```

---

## STORY-003：接入工单系统事件，由业务信号触发工作流

**基础信息**

- 故事ID：STORY-003
- 故事标题：接入工单系统事件，由业务信号触发工作流
- 所属模块：W2A 感知层
- 优先级：P0
- 故事点：5
- 负责人：FDE 周远，感知层团队
- 计划迭代：MVP Phase 1（部分）

**核心用户故事**

> 作为 **FDE**，
> 我想要 **在 Manifest 中声明 W2A Sensor 和 Trigger，将客户工单系统的新建事件路由到已有工作流**，
> 以便 **让解决方案不仅响应聊天，还能由真实业务事件自动触发，无缝嵌入客户现有业务流程**。

**验收标准**

1. **正常场景：工单创建事件触发工作流**
   - Given Manifest 的 `perception` 段声明了 `ticket_webhook` Sensor 及路由规则
   - When 向 Manifest 中声明的 `POST /w2a/tickets` 发送一条符合 W2A `schema_version: w2a/0.1` 且 `event.type: ticket.created` 的标准 Signal，并携带 `Authorization: Bearer <TICKET_WEBHOOK_TOKEN>`，Signal 至少包含 `signal_id`、`schema_version`、`emitted_at`、`source.sensor_id`、`source.sensor_version`、`source.package`、`event.type`、`event.occurred_at` 和 `source_event.data.description`
   - Then 工作流被触发，`classify_intent` 的输出为 `troubleshooting`，最终响应包含正确答案和引用
   - And 该次触发的 `traceId` 对应的 Trace 中，`trigger.type` 为 `w2a_signal`
   - And `source_event.schema` 不是必填字段；即使请求未携带内联 schema，SignalRouter 也应基于 `schema_version` 对应的预定义 envelope schema 完成协议校验

2. **异常场景：未声明的 Signal 类型被拒**
   - Given Sensor 只声明接收 `ticket.created`
   - When 发送 `ticket.closed` 类型的 Signal
   - Then 返回 400 错误，提示“Signal 类型未授权”

3. **边界场景：inputMapping 字段映射失败**
   - Given `inputMapping` 中 `message: signal.source_event.data.description`，但 Signal 不包含 `source_event.data.description` 字段
   - When 触发信号
   - Then SignalRouter 返回 400，提示“必填映射字段缺失：signal.source_event.data.description”，不进入工作流，并写入拒绝类 Trace

4. **异常场景：未知 W2A 协议版本被拒**
   - Given 平台 MVP 只支持 `schema_version: w2a/0.1`
   - When 发送 `schema_version: w2a/9.9` 的 Signal
   - Then SignalRouter 返回 400，提示“未知 W2A schema_version”，不进入工作流，并写入拒绝类 Trace

5. **边界场景：输入类型不匹配**
   - Given `signal.source_event.data.description` 被错误地传为 number
   - When 触发信号
   - Then Runtime 返回 400，提示 `INPUT_TYPE_MISMATCH`，不进入工作流

6. **边界场景：重复 Signal 不重复执行**
   - Given 已成功处理一条 `source.sensor_id: ticket_webhook`、`signal_id: sig-10086` 的 Signal
   - When 在 24 小时 TTL 内再次发送相同 `signal_id` 的 Signal
   - Then Runtime 直接返回上一次终态响应或等价响应，不重新执行工作流
   - And Trace 或审计日志中标记该请求为 duplicate

**完成定义**

- [ ] Webhook 类 Sensor 可被正确加载和启动
- [ ] Signal 路由到工作流，且输入映射生效
- [ ] 拒绝未声明的 Signal 类型
- [ ] 未知协议版本、缺失必填 envelope 字段和缺失 inputMapping 字段均返回 400
- [ ] `signal_id` 在同一环境和同一 Sensor 内具备进程内 TTL 幂等能力
- [ ] `source_event.data.description` 的类型错误会在入口层返回 400，而不是拖到组件执行阶段
- [ ] Signal 触发和 Chat 触发共用同一工作流，组件逻辑不变

**Manifest 新增片段**

```yaml
perception:
  sensors:
    - id: ticket_webhook
      ref: "@world2agent/sensor-webhook@1.0.0"
      signalTypes: [ticket.created]
      config:
        endpointPath: /w2a/tickets
        authTokenRef: env:TICKET_WEBHOOK_TOKEN
  triggers:
    - id: ticket_triage
      sensor: ticket_webhook
      signalType: ticket.created
      routeTo: classify_intent

workflow:
  inputMapping:
    chat:
      message: request.message
    w2a_signal:
      message: signal.source_event.data.description
      ticketId: signal.source_event.data.ticketId
```

---

## STORY-004：用 mock action 自动创建紧急工单

**基础信息**

- 故事ID：STORY-004
- 故事标题：用 mock action 自动创建紧急工单
- 所属模块：组件与工作流（action 组件）
- 优先级：P0
- 故事点：3
- 负责人：FDE 周远，组件注册表团队
- 计划迭代：MVP Phase 1

**核心用户故事**

> 作为 **FDE**，
> 我想要 **在工作流中加入 severity_check 和 mock create_ticket 等组件，根据业务规则模拟创建工单或发送通知**，
> 以便 **验证 AI 解决方案不仅能给出回答，还具备条件触发 action、结构化输出和 Trace 记录的执行机制**。

**验收标准**

1. **正常场景：严重客诉自动建单**
   - Given 工作流中 `check_critical` 组件接收到 `intent: complaint` 且内容包含“呕吐”“医疗”等关键词
   - When 该节点输出 `{"level":"critical"}`
   - Then `auto_create_ticket` 节点被触发，mock action 返回 `{"status":"created","ticketId":"mock-T-20001"}`
   - And 最终回答包含“已为您创建工单 mock-T-20001，专员将联系您”

2. **异常场景：外部系统不可用时降级**
   - Given mock `create_ticket` 组件被配置为模拟失败，且 `auto_create_ticket` 节点显式声明 `continueOnFailure: true`
   - When 条件满足，执行该 action
   - Then 组件返回 `{"status":"failed"}`，工作流继续执行后续节点，同时在 Trace 中记录错误信息
   - And 最终回答提示用户“系统繁忙，请稍后重试或联系人工客服”

3. **边界场景：非紧急客诉不触发 action**
   - Given 输入内容为“口感偏甜”，`check_critical` 输出 `level: normal`
   - When 工作流执行完该节点
   - Then 跳过 `auto_create_ticket`，直接进入 `generate_answer`

**完成定义**

- [ ] `severity_check`（`registry.intent.severity-beverage@1.0.0`）和 mock `create_ticket`（`registry.action.mock-create-service-ticket@1.0.0`）组件已在平台内置注册表中注册并可引用；两者均为 Phase 1 平台内置组件，无需 FDE 按 Component SDK 自定义实现
- [ ] 组件按条件正确触发/跳过，action 返回结构化输出
- [ ] action 的调用参数和返回结果完整记录在 Trace 中
- [ ] 默认 action 失败会阻断工作流；只有显式配置 `continueOnFailure: true` 的节点才允许失败后继续

**Manifest 新增组件及工作流节点**

> 以下仅展示新增部分；`human_handoff`、`intent_classifier`、`retriever`、`answer_generator` 等已有组件和工作流节点保持不变。

```yaml
components:
  # ...已有组件...
  - id: severity_check
    category: processor
    ref: registry.intent.severity-beverage@1.0.0
    config:
      criticalKeywords: ["呕吐", "医疗", "中毒", "玻璃"]
  - id: create_ticket
    category: action
    ref: registry.action.mock-create-service-ticket@1.0.0
    config:
      system: mock-ticket-system
      apiKeyRef: env:TICKET_API_KEY

workflow:
  nodes:
    # ...已有节点...
    - id: check_critical
      component: severity_check
      inputs:
        message: inputs.message
        intent: classify_intent.intent
    - id: auto_create_ticket
      component: create_ticket
      when: check_critical.level == "critical"
      continueOnFailure: true
      inputs:
        message: inputs.message
        level: check_critical.level
```

---

## STORY-005：从 PoC 平滑发布到生产环境

**基础信息**

- 故事ID：STORY-005
- 故事标题：从 PoC 平滑发布到生产环境
- 所属模块：约束性交付框架
- 优先级：P0
- 故事点：5
- 负责人：FDE 周远，交付团队
- 计划迭代：MVP Phase 4

**核心用户故事**

> 作为 **FDE**，
> 我想要 **在 Manifest 中增加 production 环境配置，执行 `solution release` 命令自动通过安全、评测、可观测性等门禁检查后生成生产部署包**，
> 以便 **PoC 的核心逻辑、知识结构、组件引用无需任何修改即可直接用于生产，消除环境漂移风险**。

**验收标准**

1. **正常场景：通过全部检查后发布**
   - Given Manifest 中已定义 `poc` 和 `production` 两套环境，`releaseChecks` 已配置
   - When 执行 `solution release lecharm-support.yaml --env=production`
   - Then 平台依次执行 `model_credentials_configured`、`sensor_credentials_configured`、`action_credentials_configured`、`signal_ingress_reachable`、`knowledge_quality_passed`、`eval_gates_passed`、`observability_enabled`、`security_baseline_passed` 检查
   - And 全部通过后，在 `./deploy/production/` 下生成 `docker-compose.yaml`、`.env.example`、运行说明和重建说明
   - And `docker-compose.yaml` 启动同一个 Runtime 二进制和同一份 Manifest/config，行为与 `solution run` 等价
   - And 工作流节点、组件引用、知识 Schema 与 PoC 完全一致

2. **异常场景：评测门禁未通过**
   - Given release 过程中现场执行的 `citation_coverage` 为 0.91，低于 0.95 门禁
   - When 执行 `solution release`
   - Then 检查序列在 `eval_gates_passed` 处失败，命令退出并提示具体指标

3. **边界场景：生产环境密钥未配置**
   - Given `production` 环境依赖 `env:OPENAI_API_KEY_PROD`，但系统环境变量中不存在该变量
   - When 执行 `solution release`
   - Then `model_credentials_configured` 检查失败，提示缺少模型密钥引用

4. **边界场景：action 密钥未配置**
   - Given `create_ticket.config.apiKeyRef: env:TICKET_API_KEY`，但目标环境缺少 `TICKET_API_KEY`
   - When 执行 `solution release`
   - Then `action_credentials_configured` 检查失败，提示缺少 action 密钥引用

5. **边界场景：生产环境使用不同模型策略**
   - Given `production.config` 中声明 `defaultModel` 或 `fallbackModel`
   - When 执行 `solution release lecharm-support.yaml --env=production`
   - Then Runtime 使用环境配置中的模型策略覆盖 `runtime.modelPolicy` 默认值
   - And 不允许通过环境配置覆盖 `components[].config`、`workflow` 或 `evaluation.gates`

**完成定义**

- [ ] `solution release` 命令可用，检查序列可观测
- [ ] 生产部署产物可一键启动，且行为与 PoC 一致
- [ ] 任何检查失败均阻断发布，给出明确原因
- [ ] 环境切换仅通过 `delivery.environments[].config` 中的白名单字段生效，不改动其他模块

**Manifest 新增片段**

```yaml
delivery:
  environments:
    - name: poc
      type: shared_sandbox
      config:
        modelKeyRef: env:OPENAI_API_KEY
        tracePath: ./data/poc/traces
        retainDays: 7
    - name: production
      type: customer_vpc
      config:
        modelKeyRef: env:OPENAI_API_KEY_PROD
        defaultModel: gpt-4.1
        fallbackModel: gpt-4.1-mini
        tracePath: /data/prod/traces
        retainDays: 90
  security:
    piiDetection: required
    promptInjectionDefense: required
    rbac: required
  releaseChecks:
    - model_credentials_configured
    - sensor_credentials_configured
    - action_credentials_configured
    - signal_ingress_reachable
    - knowledge_quality_passed
    - eval_gates_passed
    - observability_enabled
    - security_baseline_passed
```

---

## STORY-006a：手动触发知识摄取并执行质量门禁

**基础信息**

- 故事ID：STORY-006a
- 故事标题：手动触发知识摄取并执行质量门禁
- 所属模块：知识工程流水线
- 优先级：P1
- 故事点：3
- 负责人：知识工程团队，FDE 周远（操作验证）
- 计划迭代：MVP Phase 2

**核心用户故事**

> 作为 **FDE**，
> 我想要 **手动触发平台摄取预处理知识单元或本地 Markdown 知识源，生成符合 Schema 的知识单元并执行质量门禁**，
> 以便 **在不引入知识工作台和自动监听机制的前提下，验证知识工程流水线能阻止低质量知识进入检索库**。

**验收标准**

1. **正常场景：手动摄取知识并生成质量报告**
   - Given `knowledge.sources` 指向 `./data/lecharm/knowledge_units.jsonl` 或 Phase 2 支持的本地 Markdown 源
   - When FDE 执行 Phase 2 的 `solution ingest lecharm-support.yaml --env=poc`
   - Then 平台生成符合 `faq` Schema 的知识单元
   - And 输出质量报告，包含总条数、通过条数、阻断条数、警告条数
   - And 通过质量门禁的知识单元进入检索索引

2. **异常场景：关键字段缺失被阻断**
   - Given 某条知识单元缺少 `resolution` 字段
   - When 执行知识摄取
   - Then `missing_required_fields` 门禁失败，该知识单元不能进入检索库
   - And 质量报告中记录该条知识单元 ID、缺失字段和门禁类型

3. **边界场景：warn 级冲突不阻断入库**
   - Given `conflicting_answers` 检测到两条知识存在潜在冲突，且门禁 severity 为 `warn`
   - When 执行知识摄取
   - Then 摄取流程完成，但质量报告中记录 warning，供 FDE 后续处理

**完成定义**

- [ ] Phase 2 的 `solution ingest` 或等价手动摄取入口可用
- [ ] `missing_required_fields` 的 block 语义有效
- [ ] `conflicting_answers` 的 warn 语义有效
- [ ] 质量报告可被 `knowledge_quality_passed` release check 消费

**Manifest 中的知识质量门禁（`knowledge` 段下新增）**
```yaml
knowledge:
  qualityGates:
    - type: missing_required_fields
      severity: block
      scope: [faq]
    - type: conflicting_answers
      severity: warn
      scope: [faq]
```

---

## STORY-006b：知识变更自动化与专家工作台

**基础信息**

- 故事ID：STORY-006b
- 故事标题：知识变更自动化与专家工作台
- 所属模块：知识工程流水线
- 优先级：P2
- 故事点：5
- 负责人：知识工程团队，FDE 周远（操作验证）
- 计划迭代：MVP 后续

**核心用户故事**

> 作为 **领域专家（王工）**，
> 我想要 **平台自动监听知识源变化，提供冲突审阅、人工确认、审计记录和增量索引能力**，
> 以便 **业务标准变更时，AI 助手能同步最新知识，且不引入矛盾信息**。

**验收标准**

1. **正常场景：更新手册后自动刷新**
   - Given 已有知识库运行中，新版 `product_manual_grape_juice.md` 放入知识源目录
   - When 知识流水线检测到文件变更
   - Then 新手册被解析，生成符合 `faq` Schema 的知识单元
   - And `conflicting_answers` 门禁检测到与旧知识的冲突，发出告警并记录冲突详情
   - When 王工在平台知识工作台确认新标准，废弃旧知识单元
   - Then 质量门禁重新通过，索引增量更新，问答服务使用新知识

2. **异常场景：人工确认未完成**
   - Given 存在需要专家确认的冲突知识
   - When 执行生产发布
   - Then 发布检查提示存在未处理知识冲突，并阻断生产发布

3. **边界场景：知识源目录中删除手册**
   - Given 某手册被从目录中移走
   - When 流水线执行同步
   - Then 对应知识单元被标记为过期并从索引中移除，不影响其他知识

**完成定义**

- [ ] 知识流水线支持文件变更监听
- [ ] 专家确认或废弃操作有审计记录
- [ ] 索引支持增量更新
- [ ] 知识更新后评测可自动重跑，确保质量不退化

---

## STORY-007：复用于新品牌——资产复用与快速交付

**基础信息**

- 故事ID：STORY-007
- 故事标题：复用于新品牌——资产复用与快速交付
- 所属模块：可组合资产与模板
- 优先级：P1
- 故事点：3
- 负责人：FDE 周远
- 计划迭代：MVP 后持续度量

**核心用户故事**

> 作为 **FDE**，
> 我想要 **基于已交付的乐源品牌 Manifest，仅修改知识源路径、Sensor 配置和评测数据集，所有组件引用保持不变，在一天内为新品牌“果燃”交付相同能力的售后助手 PoC**，
> 以便 **组件资产跨客户复用，交付效率持续提升，组织复用率达到 60% 以上**。

**验收标准**

1. **正常场景：复制并适配 Manifest**
   - Given 已有 `lecharm-support.yaml` 及其全部组件
   - When FDE 复制为 `guoran-support.yaml`，修改 `knowledge.sources[].uri` 为 `./data/guoran/knowledge_units.jsonl`，修改 `perception.sensors[].config.endpointPath` 为新工单入口，修改 `evaluation.datasets[].uri` 为新评测集
   - Then 执行 `solution run` 后，新品牌 PoC 可正常启动，使用新知识库回答问题
   - And 组件引用列表与乐源完全一致，复用率统计 > 80%

2. **异常场景：新品牌知识源格式不兼容**
   - Given 新品牌知识源仍配置为 `type: jsonl`，但实际文件不是 JSONL 知识单元格式
   - When 执行 `solution run`
   - Then 校验或摄取失败，提示知识源类型与文件格式不兼容，不允许静默回退

3. **边界场景：新品牌无工单系统**
   - Given 客户暂时没有工单系统
   - When FDE 删除 Manifest 中 `perception` 段，同时删除 `create_ticket` 组件、`check_critical` 节点和 `auto_create_ticket` 节点
   - Then 方案退化为纯聊天助手，仍可正常启动，且不会尝试调用工单 action

**完成定义**

- [ ] 新品牌 PoC 在一天内完成交付演示
- [ ] 组件复用率自动统计并在平台资产看板中展示
- [ ] 知识切换、传感器裁剪和 action 裁剪不影响原有品牌方案

**Manifest 差异对比（guoran-support.yaml 关键修改）**
```yaml
# 仅修改以下部分；若客户无工单系统，还需删除 create_ticket 组件及相关节点
knowledge:
  sources:
    - id: product_manuals
      uri: ./data/guoran/knowledge_units.jsonl   # 修改

perception:
  sensors:
    - id: ticket_webhook
      config:
        endpointPath: /w2a/guoran-tickets   # 修改

evaluation:
  datasets:
    - id: golden_v1
      uri: ./data/guoran/evals/golden.jsonl   # 修改
```

---

## 附录：跨故事共用的 Manifest 完整示例

以下是 STORY-001 至 STORY-004 整合后的 Manifest 完整版，供实现参考：

```yaml
apiVersion: solution.codex/v1
kind: Solution
metadata:
  name: lecharm-support-agent
  version: 0.1.0
  owner: fde-zhouyuan
  industry: beverage
  solutionType: customer-support

perception:
  sensors:
    - id: ticket_webhook
      ref: "@world2agent/sensor-webhook@1.0.0"
      signalTypes: [ticket.created]
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
      uri: ./data/lecharm/knowledge_units.jsonl
      schema: faq
  schemas:
    - id: faq
      fields:
        - symptom
        - cause
        - resolution
        - product_model
        - source_ref
components:
  - id: intent_classifier
    category: processor
    ref: registry.intent.beverage-router@1.0.0
    config:
      intents: [troubleshooting, warranty, complaint, human_handoff]
  - id: retriever
    category: processor
    ref: registry.retriever.local-keyword@1.0.0
    config:
      topK: 5
      requireCitation: true
  - id: severity_check
    category: processor
    ref: registry.intent.severity-beverage@1.0.0
    config:
      criticalKeywords: ["呕吐", "医疗", "中毒", "玻璃"]
  - id: answer_generator
    category: processor
    ref: registry.agent.cited-answer@1.0.0
    config:
      style: concise
      requireGrounding: true
  - id: create_ticket
    category: action
    ref: registry.action.mock-create-service-ticket@1.0.0
    config:
      system: mock-ticket-system
      apiKeyRef: env:TICKET_API_KEY
  - id: human_handoff
    category: action
    ref: registry.action.human-handoff@1.0.0
    config:
      queue: support-l2

workflow:
  entrypoint: classify_intent
  onError:
    retry: 1
    fallbackNode: handoff
  inputMapping:
    chat:
      message: request.message
    w2a_signal:
      message: signal.source_event.data.description
      ticketId: signal.source_event.data.ticketId
  nodes:
    - id: classify_intent
      component: intent_classifier
      inputs:
        message: inputs.message
    - id: retrieve_knowledge
      component: retriever
      inputs:
        query: inputs.message
    - id: check_critical
      component: severity_check
      inputs:
        message: inputs.message
        intent: classify_intent.intent
    - id: auto_create_ticket
      component: create_ticket
      when: check_critical.level == "critical"
      continueOnFailure: true
      inputs:
        message: inputs.message
        level: check_critical.level
    - id: generate_answer
      component: answer_generator
      inputs:
        message: inputs.message
        passages: retrieve_knowledge.passages
        citations: retrieve_knowledge.citations
    - id: handoff
      component: human_handoff
      when: classify_intent.confidence < 0.65
      inputs:
        message: inputs.message

runtime:
  knowledgeBindings:
    - component: retriever
      sources: [product_manuals]
  modelPolicy:
    defaultModel: gpt-4.1
    fallbackModel: gpt-4.1-mini
    maxLatencyMs: 8000
  observability:
    trace: required
    logInputs: masked
    retainDays: 30

evaluation:
  datasets:
    - id: golden_v1
      uri: ./data/lecharm/evals/golden.jsonl
      caseFormat: runtime_request_jsonl
  metrics:
    - citation_coverage
    - answer_accuracy
  gates:
    - metric: citation_coverage
      min: 0.95
      severity: block
      schedule: onRelease
    - metric: answer_accuracy
      min: 0.85
      severity: warn
      schedule: weekly

delivery:
  environments:
    - name: poc
      type: shared_sandbox
      config:
        modelKeyRef: env:OPENAI_API_KEY
        tracePath: ./data/poc/traces
        retainDays: 7
    - name: production
      type: customer_vpc
      config:
        modelKeyRef: env:OPENAI_API_KEY_PROD
        defaultModel: gpt-4.1
        fallbackModel: gpt-4.1-mini
        tracePath: /data/prod/traces
        retainDays: 90
  security:
    piiDetection: required
    promptInjectionDefense: required
    rbac: required
  releaseChecks:
    - model_credentials_configured
    - sensor_credentials_configured
    - action_credentials_configured
    - signal_ingress_reachable
    - knowledge_quality_passed
    - eval_gates_passed
    - observability_enabled
    - security_baseline_passed
```

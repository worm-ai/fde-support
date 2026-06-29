# 规格附件 2：内置组件规格

> 对应开发计划任务 T2.3、T3.3。本文档内容全部摘录自详细设计文档与用户故事文档的已确定规则，无新增设计。

## 1. 组件接口契约

所有组件实现 `Component` 接口：

```go
type Component interface {
    ID() string
    Category() ComponentCategory  // "processor" | "action"
    Run(ctx context.Context, input map[string]any, runtime RuntimeContext) (map[string]any, error)
}
```

**组件实现必须在调用之间是无状态的。** 平台不保证组件实例的请求隔离。

## 2. 错误模型

| 错误类型 | 返回方式 | Runtime 行为 |
|---------|---------|-------------|
| 系统异常（不可恢复） | `(nil, error)` | 视为 hard failure，进入重试/fallback |
| 业务失败（可恢复） | `(output, nil)` 且 `output["status"] = "failed"` | action 节点按 `continueOnFailure` 处理 |
| 禁止 | `output["status"] = "error"` | 禁止使用 |

- processor 组件始终 hard failure，不支持 `continueOnFailure`
- action 组件默认 `continueOnFailure: false`
- 组件输出必须是一层 JSON 对象

## 3. Phase 1 内置组件清单（7 个）

### 3.1 `registry.intent.support-router@1.0.0`

| 属性 | 值 |
|------|-----|
| 组件类型 | `llm-classifier` |
| 类别 | processor |
| 功能 | 通用售后意图分类 |
| config | `intents: [troubleshooting, warranty, complaint, human_handoff]` |
| input | `message: string` |
| output | `intent: string`, `confidence: number` |
| 能力要求 | `model.generate` |

**行为**：将用户消息分类到配置的意图列表之一。通过 `context.Model().Generate()` 调用 LLM。输出 `intent` 和 `confidence`（0-1 之间的浮点数）。

### 3.2 `registry.intent.beverage-router@1.0.0`

| 属性 | 值 |
|------|-----|
| 组件类型 | `llm-classifier` |
| 类别 | processor |
| 功能 | 饮品行业意图分类 |
| config | `intents: [troubleshooting, warranty, complaint, human_handoff]` |
| input | `message: string` |
| output | `intent: string`, `confidence: number` |
| 能力要求 | `model.generate` |

**行为**：与 `support-router` 同类型，针对饮品行业使用不同的默认 prompt 模板。具体行为由 Manifest `config` 定义。

### 3.3 `registry.intent.severity-beverage@1.0.0`

| 属性 | 值 |
|------|-----|
| 组件类型 | `llm-classifier` |
| 类别 | processor |
| 功能 | 饮品场景严重度判断 |
| config | `criticalKeywords: ["呕吐", "医疗", "中毒", "玻璃"]` |
| input | `message: string`, `intent: string` |
| output | `level: string` ("critical" \| "normal") |
| 能力要求 | `model.generate` |

**行为**：判断用户消息是否属于严重客诉。基于 `config.criticalKeywords` 关键词匹配和 LLM 分类结果综合判定。输出 `level: "critical"` 或 `level: "normal"`。

### 3.4 `registry.retriever.local-keyword@1.0.0`

| 属性 | 值 |
|------|-----|
| 组件类型 | `retriever` |
| 类别 | processor |
| 功能 | 本地关键词检索 |
| config | `topK: 5`, `requireCitation: true` |
| input | `query: string` |
| output | `passages: string[]`, `citations: string[]` |
| 能力要求 | `knowledge.search` |

**行为**：调用 `context.Knowledge().Retrieve(query, topK)` 从内存关键词索引检索相关知识。返回 `passages`（检索到的文本片段）和 `citations`（`"source#ref"` 格式的引用字符串数组）。

### 3.5 `registry.agent.cited-answer@1.0.0`

| 属性 | 值 |
|------|-----|
| 组件类型 | `llm-generator` |
| 类别 | processor |
| 功能 | 带引用的回答生成 |
| config | `style: "concise"`, `requireGrounding: true` |
| input | `message: string`, `passages: string[]`, `citations: string[]` |
| output | `answer: string` |
| 能力要求 | `model.generate` |

**行为**：基于检索到的 `passages` 和 `citations`，通过 `context.Model().Generate()` 生成带引用溯源的回答。`requireGrounding: true` 时，回答必须基于提供的 passages，不得编造。

### 3.6 `registry.action.human-handoff@1.0.0`

| 属性 | 值 |
|------|-----|
| 组件类型 | `human-handoff` |
| 类别 | action |
| 功能 | 人工升级/转接 |
| config | `queue: "support-l2"` |
| input | `message: string`, `errorSummary: object?`（fallback 模式自动注入） |
| output | `status: string`, `queue: string`, `reason: string` |
| 能力要求 | 无 |

**行为**：记录人工升级请求。在 fallback 模式下从 `input.errorSummary` 或 `context.Error()` 读取错误摘要，并写入 action 输出和 Trace。

### 3.7 `registry.action.mock-create-service-ticket@1.0.0`

| 属性 | 值 |
|------|-----|
| 组件类型 | mock action |
| 类别 | action |
| 功能 | 模拟工单创建 |
| config | `system: "mock-ticket-system"`, `apiKeyRef: "env:TICKET_API_KEY"` |
| input | `message: string`, `level: string` |
| output | `status: string` ("created" \| "failed"), `ticketId: string?` |
| 能力要求 | 无 |

**行为**：模拟创建工单。正常返回 `{"status":"created","ticketId":"mock-T-xxxxx"}`。配置为模拟失败时返回 `{"status":"failed"}`。必须配合 `continueOnFailure: true` 使用以允许软失败继续。

## 4. Phase 2 扩展组件

### 4.1 `registry.action.http-caller@1.0.0`

| 属性 | 值 |
|------|-----|
| 组件类型 | `http-caller` |
| 类别 | action |
| 功能 | HTTP API 调用 |
| config | `url`, `method`, `headers`, `bodyTemplate`, `timeoutMs`, `continueOnFailure` |
| input | 模板变量（如 `phone`），用于填充 `bodyTemplate` |
| output | `status: string`, `statusCode: number`, `body: any` |
| 能力要求 | `http.call` |

**行为**：通过 `context.HTTP()` 发起 HTTP 请求。支持 `env:VAR_NAME` 引用注入认证信息。超时和网络错误返回 `{"status":"failed"}`。Trace 中记录 host、方法、状态码、耗时和脱敏后的错误。

### 4.2 其他 Phase 2 组件

| 组件类型 | 类别 | 功能 |
|---------|------|------|
| `llm-extractor` | processor | `prompt` + `schema` → 结构化提取 |
| `data-query` | processor | `source` + `query` → 结构化查询结果 |
| `rule-evaluator` | processor | `rules` 列表 → 条件匹配结果 |

# 规格附件 3：Trace JSON Schema

> 对应开发计划任务 T2.9、T5.8。本文档内容全部摘录自详细设计文档 TraceWriter 节（行 1299-1360）与技术架构文档 TraceRecord 定义（行 708-732），无新增设计。

## 1. Trace 结构

```json
{
  "traceId": "trace_abc123",
  "solution": "lecharm-support-agent",
  "version": "0.1.0",
  "environment": "poc",
  "trigger": {
    "type": "chat",
    "sensor": null,
    "signalType": null
  },
  "status": "success",
  "input": {
    "message": "葡萄汁出现沉淀怎么处理？"
  },
  "spans": [
    {
      "spanId": "span_001",
      "node": "classify_intent",
      "component": "registry.intent.beverage-router@1.0.0",
      "kind": "workflow_node",
      "attempt": 1,
      "input": {"message": "葡萄汁出现沉淀怎么处理？"},
      "output": {"intent": "complaint", "confidence": 0.91},
      "latencyMs": 1200,
      "error": null
    }
  ],
  "latencyMs": 3200,
  "error": null
}
```

## 2. 字段定义

### 2.1 顶层字段

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `traceId` | string | 是 | TraceWriter 生成的唯一标识 |
| `solution` | string | 是 | 解决方案名称（`metadata.name`） |
| `version` | string | 是 | 解决方案版本（`metadata.version`） |
| `environment` | string | 是 | 环境名（`--env` 参数值） |
| `trigger` | object | 是 | 触发信息 |
| `trigger.type` | string | 是 | `"chat"` 或 `"w2a_signal"` |
| `trigger.sensor` | string? | 否 | W2A 触发时记录 Sensor ID |
| `trigger.signalType` | string? | 否 | W2A 触发时记录 Signal 类型 |
| `status` | string | 是 | `"success"` \| `"rejected"` \| `"failed"` |
| `input` | object | 是 | 脱敏后的归一化 RuntimeRequest 输入 |
| `spans` | array | 是 | 执行 span 列表 |
| `latencyMs` | number | 是 | 请求总耗时（毫秒） |
| `error` | object? | 否 | 顶层错误摘要，`status` 为 `"rejected"` 或 `"failed"` 时必填 |

### 2.2 Span 字段

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `spanId` | string | 是 | Span 唯一标识 |
| `node` | string? | 否 | 工作流节点 ID（`kind: workflow_node` 时必填） |
| `component` | string? | 否 | 组件 ref（`kind: workflow_node` 时必填） |
| `kind` | string | 是 | `workflow_node` \| `router` \| `model` \| `knowledge` \| `http` \| `release_check` |
| `attempt` | number | 是 | 重试序号（从 1 开始） |
| `input` | object? | 否 | 脱敏后的节点输入 |
| `output` | object? | 否 | 节点输出（成功时有值） |
| `latencyMs` | number | 是 | 节点执行耗时（毫秒） |
| `error` | object? | 否 | 节点级错误（失败时有值） |

### 2.3 Error 字段

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `error.code` | string | 是 | 机器可读错误码（如 `WORKFLOW_FAILED`） |
| `error.type` | string | 是 | 稳定错误类型字符串（如 `component_error`、`timeout`、`model_timeout`） |
| `error.message` | string | 是 | 人类可读错误描述（不含 stack/密钥/PII） |
| `error.failedNode` | string? | 否 | 失败节点 ID（顶层 error 时有值） |
| `error.attempts` | number | 否 | 已重试次数（顶层 error 时有值） |
| `error.retryable` | boolean | 是 | 是否可重试 |

### 2.4 标准错误类型（`error.type`）

| 类型字符串 | 含义 |
|-----------|------|
| `component_error` | 组件返回 `(nil, error)` |
| `component_failed_status` | action 返回 `{"status":"failed"}` 且 `continueOnFailure: false` |
| `timeout` | 组件执行超时 |
| `model_timeout` | 模型调用超时 |
| `condition_error` | `when` 条件求值失败 |
| `input_mapping_error` | `inputMapping` 字段缺失或类型不匹配 |
| `unauthorized_signal` | W2A Signal 认证失败 |
| `knowledge_error` | 知识检索异常 |
| `validation_error` | 请求 Schema 校验失败 |

## 3. 三种 Trace 状态规则

### 3.1 `status: "success"`
- 工作流正常完成
- 包含每个执行节点的 span
- `error: null`

### 3.2 `status: "rejected"`
- 请求在入口层被拒绝
- 可以没有 workflow node span，但必须包含 router 或入口 span（`kind: "router"`）
- 必须写入结构化 `error`
- 触发条件：未知 W2A 版本、未授权 Signal 类型、inputMapping 缺失、请求 Schema 错误
- **认证失败只写安全审计日志，不写拒绝类 Trace**
- 无法归属到 Solution/Sensor 的未知路径或扫描流量只写安全审计日志

### 3.3 `status: "failed"`
- 工作流执行过程中发生 hard failure
- 保留已完成 span 和失败 span
- 顶层 `error` 必须与失败 span 的错误摘要一致

## 4. 脱敏规则

| 数据 | 处理方式 |
|------|---------|
| `message`、`description` 等用户输入字段 | `logInputs: masked` 时做哈希或截断处理 |
| `raw_payload` | 不写入 Trace |
| 输出中的 PII 字段（邮箱、电话等） | 写入前脱敏 |
| 密钥值（API Key、Token 等） | 不写入 Trace |
| 完整 URL query | 不写入 Trace |
| 完整外部响应体 | 不写入 Trace |
| 内部 stack trace | 只能进入本地调试日志，不进入 Trace |

## 5. Go 类型定义（摘自技术架构文档）

```go
type TraceRecord struct {
    TraceID     string
    Solution    string
    Version     string
    Environment string
    Trigger     TriggerSpec
    Input       map[string]any
    Spans       []TraceSpan
    LatencyMS   int64
    Status      string
    Error       *RuntimeErrorSummary
}
```

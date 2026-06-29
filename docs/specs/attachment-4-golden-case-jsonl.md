# 规格附件 4：Golden Case JSONL 格式规范

> 对应开发计划任务 T5.1、T5.2。本文档内容全部摘录自用户故事 STORY-002 与详细设计文档评测章节，无新增设计。

## 1. Golden Case JSONL 格式

每行一个 JSON 对象。使用 `runtime_request_jsonl` 输入模型。

### 1.1 完整字段定义

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

### 1.2 字段说明

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `id` | string | 是 | Case 唯一标识 |
| `trigger` | object | 是 | 触发方式 |
| `trigger.type` | string | 是 | `"chat"` 或 `"w2a_signal"` |
| `request` | object | 是 | 标准 RuntimeRequest 输入 |
| `raw_payload` | any | 否 | 原始请求体（用于 W2A Signal 场景的完整 Signal body） |
| `expected` | object | 是 | 期望结果 |

### 1.3 `expected` 字段

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `intent` | string | 是 | 期望的意图分类结果。必须在组件声明的 `intents` 列表内。 |
| `mustCite` | boolean | 否 | 是否要求回答包含引用。默认 `false`。 |
| `answerContains` | string[] | 否 | 期望回答中包含的词项列表。**AND 语义**：所有词项都必须出现在最终答案中，该 case 才通过。 |

## 2. 触发类型差异

### 2.1 Chat 触发

```json
{
  "id": "case_02",
  "trigger": {"type": "chat"},
  "request": {"message": "口感偏甜怎么办？"},
  "expected": {"intent": "troubleshooting"}
}
```

### 2.2 W2A Signal 触发

```json
{
  "id": "case_03",
  "trigger": {"type": "w2a_signal"},
  "request": {"message": "泵显示E42错误"},
  "raw_payload": {
    "signal_id": "sig-test-001",
    "schema_version": "w2a/0.1",
    "source": {"sensor_id": "ticket_webhook", "sensor_version": "0.1.0"},
    "event": {"type": "ticket.created"},
    "source_event": {"data": {"description": "泵显示E42错误", "ticketId": "T-10086"}}
  },
  "expected": {"intent": "troubleshooting", "mustCite": true}
}
```

## 3. 评测指标判定规则

### 3.1 `citation_coverage`

| 条件 | 结果 |
|------|------|
| `expected.mustCite: true` 且回答中 citations 非空 | 通过 |
| `expected.mustCite: true` 且回答中 citations 为空 | 不通过 |
| `expected.mustCite: false` 或未设置 | 不参与 citation_coverage 计算 |

**指标值** = 通过 case 数 / 需引用的 case 总数

### 3.2 `answer_accuracy`（规则断言，MVP 版本）

| 条件 | 结果 |
|------|------|
| `expected.answerContains` 中**所有**词项都出现在最终答案中 | 通过 |
| `expected.answerContains` 中任一词项未出现在最终答案中 | 不通过 |
| `expected.answerContains` 为空或未设置 | 该 case 不参与 answer_accuracy 计算 |

**指标值** = 通过 case 数 / 设置了 `answerContains` 的 case 总数

> **注意**：MVP 阶段的 `answer_accuracy` 使用规则断言（字符串包含检查），不使用 LLM-as-judge。后续版本可替换为 LLM-as-judge 以支持语义级别评估。

## 4. 门禁配置

```yaml
evaluation:
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

### 4.1 门禁语义

| 属性 | 值 | 说明 |
|------|-----|------|
| `severity: block` | 不通过 → 退出码 1 | 阻断后续流程 |
| `severity: warn` | 不通过 → 退出码 0 | 输出告警，不阻断 |
| `schedule: onRelease` | `solution release` 时执行 | 参与发布阻断 |
| `schedule: weekly` | MVP 不自动执行 | 仅 Schema 校验通过，不参与发布阻断 |

## 5. 评测执行流程

```
Golden Case JSONL
  → 每行解析为 RuntimeRequest
  → 进程内 WorkflowExecutor 执行（不依赖 HTTP 端口）
  → 获取 TraceRecord
  → 按指标规则判定
  → 汇总指标值
  → 对比门禁
  → 输出报告
```

## 6. 边界条件

| 条件 | 行为 |
|------|------|
| `datasets` 指向的 JSONL 文件为空 | 提示"评测数据集为空，请至少提供一条 Golden Case"，返回非零退出码 |
| `expected.intent` 不在组件声明的 `intents` 列表内 | 校验失败，提示不匹配 |
| `answerContains` 词项包含特殊字符 | 做字面字符串匹配，不做正则解析 |

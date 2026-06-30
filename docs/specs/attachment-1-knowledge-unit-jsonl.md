# 规格附件 1：知识单元 JSONL 字段规范与 Loader 行为矩阵

> 对应开发计划任务 T2.5、T3.7。本文档内容全部摘录自详细设计文档与用户故事文档的已确定规则，无新增设计。

## 1. JSONL 记录格式

每条记录是一个 JSON 对象。Phase 1 内置 Schema `faq` 的推荐字段：

| 字段 | 类型 | 说明 |
|------|------|------|
| `symptom` | string | 故障现象 |
| `cause` | string | 原因分析 |
| `resolution` | string | 解决方案 |
| `product_model` | string | 适用产品型号 |
| `source_ref` | string | 引用来源标识 |

## 2. Loader 校验规则（Phase 1 `solution run`）

### 2.1 文件级校验

| 条件 | 严重度 | 行为 |
|------|--------|------|
| 文件不存在 | `block` | 终止服务启动 |
| JSONL 无法解析（格式错误） | `block` | 终止服务启动 |
| 文件存在但记录数为 0 | `warn` | 服务可启动，检索结果为空 |

### 2.2 记录级校验

| 条件 | 严重度 | 行为 |
|------|--------|------|
| 非空记录缺少可检索文本字段 | `block` | 终止服务启动 |
| 非空记录缺少引用字段（默认字段名 `source_ref`） | `block` | 终止服务启动 |
| 空行（空白 JSON 或 `{}`） | — | 跳过，不计入总数 |

### 2.3 可检索文本字段选取规则

按以下优先级从 JSONL 记录中选取第一个非空字符串字段作为可检索文本：

```
answer → resolution → question → symptom → cause → description → content
```

Manifest 中声明的 Schema 字段（如 `faq` 的 `symptom/cause/resolution`）与 JSONL 实际字段通过上述优先级规则映射，无需严格同名。

## 3. 引用字段与 Citations 格式

- 默认引用字段名为 `source_ref`
- Loader 从每条记录读取 `source_ref` 值，统一生成为 `"source#ref"` 格式的引用字符串
- 示例：若 `source_ref` 值为 `"manual-001#section-4.2"`，则 citation 为 `"manual-001#section-4.2"`
- 所有组件的 citations 统一使用 `string[]` 格式
- 若 JSONL 记录的 `source_ref` 为空或缺失，属于缺少引用字段的 block 级错误

## 4. 最小质量报告格式

生成路径：`dirname(tracePath)/reports/knowledge-quality.json`（例如 `./data/poc/reports/knowledge-quality.json`）

```json
{
  "manifest_fingerprint": "sha256:abc123...",
  "knowledge_config_fingerprint": "sha256:def456...",
  "knowledge_sources_fingerprint": "sha256:ghi789...",
  "source_uri": "./data/lecharm/knowledge_units.jsonl",
  "source_mtime": "2026-06-29T10:00:00Z",
  "generated_at": "2026-06-29T10:05:00Z",
  "total_records": 150,
  "skipped_empty": 2,
  "valid_records": 148,
  "checks": [
    {"type": "file_exists", "severity": "block", "passed": true},
    {"type": "jsonl_parseable", "severity": "block", "passed": true},
    {"type": "min_searchable_field", "severity": "block", "passed": true, "failures": []},
    {"type": "citation_field_present", "severity": "block", "passed": true, "failures": []},
    {"type": "record_count_zero", "severity": "warn", "passed": true}
  ],
  "build_id": "20260629T100500-a1b2c3"
}
```

### 4.1 检查项定义

| 检查项 | 严重度 | 判定 |
|--------|--------|------|
| `file_exists` | block | 声明的知识源文件存在 |
| `jsonl_parseable` | block | 文件可逐行解析为 JSON |
| `min_searchable_field` | block | 每条非空记录至少有一个可检索文本字段 |
| `citation_field_present` | block | 每条非空记录有非空 `source_ref` |
| `record_count_zero` | warn | 记录总数 > 0 |

### 4.2 build_id 格式

`{生成时间UTC紧凑格式}-{manifest_fingerprint前6位}`，例如 `20260629T100500-a1b2c3`。

质量报告与知识索引使用相同 `build_id` 关联。release 检查时同时校验两者的 `build_id` 一致性。

## 5. Phase 2 扩展（`solution ingest`）

Phase 2 的 `knowledge.schemas[].fields` 支持字段对象语法 `{name, required}`：

```yaml
knowledge:
  schemas:
    - id: faq
      fields:
        - name: symptom
          required: false
        - name: resolution
          required: true
```

- `required: true` 是 `missing_required_fields` 门禁的唯一判定依据
- 未声明 `required` 的字段默认为 `required: false`，不得被该门禁阻断
- Phase 1 的字符串字段列表继续兼容，字符串字段默认 `required: false`

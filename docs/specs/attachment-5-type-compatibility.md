# 规格附件 5：组件间类型流校验规则

> 对应开发计划任务 T6.2。本文档内容全部摘录自详细设计文档与技术架构文档的已确定规则，无新增设计。

## 1. 基础类型

MVP 仅支持以下 5 类基础类型（摘自技术架构文档 7.9 节）：

| 类型 | Go 对应类型 | JSON 示例 |
|------|-----------|----------|
| `string` | `string` | `"hello"` |
| `number` | `float64` | `3.14` |
| `boolean` | `bool` | `true` |
| `object` | `map[string]any` | `{"key": "value"}` |
| `array` | `[]any` | `[1, 2, 3]` |

## 2. 校验粒度

MVP 类型流校验粒度固定为**一级扁平字段校验**（摘自详细设计文档行 1063）：

- 字段存在性：下游节点的 `inputs` 引用的字段必须在上游节点的 `outputSchema` 中存在
- 基础类型匹配：字段的类型必须与声明的 `outputSchema` 一致
- `object` 和 `array` 的内部结构不递归校验
- 如需被 `when` 或下游节点稳定引用，组件必须把嵌套值展开为一层输出字段
- 嵌套 Schema、数组元素类型和 JSON Schema 级深度校验留到后续版本

## 3. 类型兼容性矩阵

行 = 上游输出类型，列 = 下游期望类型。

| ↓上游 \ 下游→ | string | number | boolean | object | array |
|---------------|--------|--------|---------|--------|-------|
| **string** | ✅ 兼容 | ❌ 不兼容 | ❌ 不兼容 | ❌ 不兼容 | ❌ 不兼容 |
| **number** | ❌ 不兼容 | ✅ 兼容 | ❌ 不兼容 | ❌ 不兼容 | ❌ 不兼容 |
| **boolean** | ❌ 不兼容 | ❌ 不兼容 | ✅ 兼容 | ❌ 不兼容 | ❌ 不兼容 |
| **object** | ❌ 不兼容 | ❌ 不兼容 | ❌ 不兼容 | ✅ 兼容 | ❌ 不兼容 |
| **array** | ❌ 不兼容 | ❌ 不兼容 | ❌ 不兼容 | ❌ 不兼容 | ✅ 兼容 |

**规则**：MVP 不做隐式类型转换。类型必须严格匹配。

## 4. 校验时机与范围

### 4.1 校验时机

`solution validate` 阶段执行。Validator 在"工作流数据流校验"阶段检查类型兼容性。

### 4.2 校验范围

| 校验项 | 说明 |
|--------|------|
| 字段存在性 | 下游 `inputs` 引用的字段路径必须能在上游 `outputSchema` 中解析 |
| 基础类型匹配 | 字段的 primitive 类型必须一致 |
| `object`/`array` 顶层类型 | 只校验顶层类型，不递归校验内部结构 |
| 多上游引用 | 同一字段被多个上游节点输出时，所有上游输出该字段的类型必须一致 |

### 4.3 不校验的场景

| 场景 | 处理方式 |
|------|---------|
| 上游节点带 `when` 条件 | 该节点的输出不参与类型校验（因为可能不执行） |
| 上游节点引用后置节点 | 已被"可跳过依赖校验"拦截，不进入类型校验 |
| 嵌套对象内部字段 | 不递归校验，仅校验顶层 `object` 类型 |
| 数组元素类型 | 不校验，仅校验顶层 `array` 类型 |

## 5. 错误格式

类型不兼容时，Validator 返回：

```json
{
  "code": "TYPE_MISMATCH",
  "path": "workflow.nodes[2].inputs.passages",
  "message": "Node 'generate_answer' expects 'passages' as string, but upstream node 'retrieve_knowledge' outputs 'passages' as array",
  "hint": "Check the outputSchema of 'retrieve_knowledge' and the input reference in 'generate_answer'"
}
```

## 6. Schema 定义格式

组件的 `component.yaml` 中声明 `inputSchema` 和 `outputSchema`：

```yaml
# component.yaml 示例
ref: registry.retriever.local-keyword@1.0.0
category: processor
inputSchema:
  query: string
outputSchema:
  passages: array
  citations: array
```

`ComponentDescriptor.InputSchema` 和 `OutputSchema` 采用扁平 `field → primitive type` 表达（摘自技术架构文档 7.9 节）：

```go
type ComponentDescriptor struct {
    Ref          string
    Category     string
    ConfigSchema map[string]string
    InputSchema  map[string]string
    OutputSchema map[string]string
}
```

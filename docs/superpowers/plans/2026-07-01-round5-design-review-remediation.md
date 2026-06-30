# Round 5 Design Implementation Review — 开发修复计划

基于 [2026-07-01 第 5 轮设计实现审查](/Users/cc/ai/fde-support/docs/reviews/2026-06-30-round5-design-implementation-review.md)。

---

## 修复清单

### P0-1：注册 evaluate 命令的 `--env` 标志（BUG-R5-001）

**目标**：修复 evaluate 命令——`evalEnvName` 变量已声明但未通过 cobra 注册，导致命令完全不可用。

**文件**：`cmd/solution/main.go`

**具体修改**：

在 `evaluateCmd` 定义（第 81 行之后，第 100 行之前）添加：
```go
evaluateCmd.Flags().StringVar(&evalEnvName, "env", "poc", "delivery environment name")
```

**验收标准**：
- `solution evaluate manifest.yaml` 使用默认 `poc` 环境，行为与 R4 一致
- `solution evaluate manifest.yaml --env=production` 使用 production 环境配置
- 新增测试或手动验证：两个环境产生的评测结果可能因模型策略不同而不同

---

### P0-2：修复 `ReleaseManifestFile` 使用 `needsModelGateway`（BUG-R5-002）

**目标**：修复 release 命令——纯关键词检索方案（无模型需求）不应因模型密钥缺失而失败。

**文件**：`internal/app/app.go` → `ReleaseManifestFile`（第 271 行）

**具体修改**：

将：
```go
modelGateway, err := buildModelGateway(resolvedEnv, false)
```
改为：
```go
modelGateway, err := buildModelGateway(resolvedEnv, needsModelGateway(m))
```

**验收标准**：
- 无 `model.generate` 需求的方案可通过 release（不需要 OPENAI_API_KEY）
- 有 `model.generate` 需求（如 llm-classifier）的方案仍要求模型密钥
- `BuildRuntime`、`EvaluateManifestFile`、`ReleaseManifestFile` 三个入口行为一致

---

### P1-1：修复 `needsModelGateway` 性能问题（HIGH-R5-001）

**目标**：避免对每个组件都创建新的 registry 实例（文件系统扫描）。

**文件**：`internal/app/app.go` → `needsModelGateway`

**具体修改**：

将 registry 创建移到循环外：
```go
func needsModelGateway(m *manifest.SolutionManifest) bool {
    reg, err := registry.NewBuiltinComponentRegistryFromRoot(m.BaseDir)
    if err != nil {
        return false
    }
    for _, spec := range m.Components {
        compDesc, err := reg.Resolve(spec.Ref)
        if err != nil {
            continue
        }
        for _, req := range compDesc.Requires {
            if req == "model.generate" {
                return true
            }
        }
    }
    return false
}
```

**验收标准**：
- 功能不变
- 对于有 N 个组件的 Manifest，只创建 1 次 registry（而非 N 次）

---

### P1-2：修复 `scopedKnowledge` 类型安全（HIGH-R5-002）

**目标**：防止 future maintainer 修改 knowledgeStore 类型时导致 panic。

**文件**：`internal/runtimecore/executor.go` → `scopedKnowledge`

**具体修改**（推荐方案 B）：

将：
```go
return e.knowledgeStore.(*knowledge.Store).FilterBySources(binding.Sources)
```
改为：
```go
if ks, ok := e.knowledgeStore.(*knowledge.Store); ok {
    return ks.FilterBySources(binding.Sources)
}
return e.knowledge
```

**验收标准**：
- 功能不变
- 若 knowledgeStore 类型不是 `*knowledge.Store`，降级为返回全量 knowledge（而非 panic）

---

### P1-3：docker-compose.yaml 增加 runtime image 构建来源（HIGH-R5-003）

**目标**：release 产物的 compose 文件可被用户直接 `docker-compose up -d`。

**文件**：`internal/delivery/docker_compose.go`

**具体修改**（推荐方案 A）：

在 `generateComposeContent` 中增加 build 指令：
```yaml
services:
  solution-runtime:
    build:
      context: .
      dockerfile: Dockerfile
    # image: solution-runtime:<version>  # 可选，用于 registry push
    command: ...
```

同时生成 `Dockerfile`：
```dockerfile
FROM golang:1.21 AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o solution ./cmd/solution

FROM alpine:3.19
COPY --from=builder /app/solution /usr/local/bin/solution
ENTRYPOINT ["solution"]
```

**验收标准**：
- `docker-compose config` 通过
- 生成产物包含 Dockerfile 或 README 中包含明确的镜像获取方式

---

## P2 任务

| 任务 | 描述 |
|---|---|
| P2-1 | 实现 eval cache：execution/dataset/knowledge fingerprint + TTL 复用 |
| P2-2 | `executeNodeWithRetry` 对确定性错误（input_mapping/input_type_mismatch）直接失败不重试 |
| P2-3 | `human_handoff` 状态改为 `"handed_off"` 匹配设计示例 |
| P2-4 | 自定义组件运行时执行 |
| P2-5 | Python Worker 接入 |
| P2-6 | PostgreSQL schema + W2A 幂等持久化 |
| P2-7 | `solution destroy` 命令 |
| P2-8 | 组件复用统计 |

---

## 执行顺序建议

```
P0-1 (eval --env flag) ──→ P0-2 (release needsModelGateway)
                                  ↓
P1-1 (needsModelGateway perf) ←── P1-2 (scopedKnowledge safety)
P1-3 (compose image)
                                  ↓
                            P2 tasks...
```

---

## 提示词

```
你是资深 Go 工程师。请严格按照以下修复计划逐任务执行修复，每个任务修复完成后运行对应测试确认通过，再继续下一个任务。

修复计划文件：docs/superpowers/plans/2026-07-01-round5-design-review-remediation.md
设计文档：docs/solution-as-code-fde-platform-design.md

执行规则：
1. 先执行 P0 任务（P0-1 → P0-2），再执行 P1 任务（P1-1 → P1-3），最后执行 P2 任务
2. 每个任务修复完成后必须运行 go test ./cmd/... ./internal/... -count=1 确保无回归
3. 不要在同一次修复中同时修改超过 2 个不相关的文件
4. 遇到不确定的点先查阅设计文档再做修改
5. 修复完成后向用户报告结果和测试状态
```

**关键提醒**：
- P0-1 添加一行 `evaluateCmd.Flags().StringVar(&evalEnvName, "env", "poc", "delivery environment name")`
- P0-2 将 `buildModelGateway(resolvedEnv, false)` 改为 `buildModelGateway(resolvedEnv, needsModelGateway(m))`
- 这两个 P0 修复加起来不到 3 行代码，但修复了两个致命 Bug

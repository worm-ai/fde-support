# 2026-07-01 Round 5 Design Implementation Review

## 审查摘要

**审查基准**：`docs/solution-as-code-fde-platform-design.md` + 技术架构 + 开发计划 + 规格附件

**审查范围**：
- 仅分析受版本控制的项目源码（当前 15 个修改文件）
- 忽略第三方库、自动生成代码、运行期产物

**验证命令与结果**：

| 验证项 | 命令 | 结果 |
|---|---|---|
| 全工作区测试 | `go test ./cmd/... ./internal/... -count=1` | **全部通过** |
| 示例 validate | `solution validate examples/{after-sales-support,guoran-support}/manifest.yaml` | 通过 |
| 路径逃逸防御 | `/etc/passwd`、`C:\secret.txt`、`../secret.txt` | **全部通过** |

**整体结论**：

- Round 5 相较 Round 4 取得 **重大进展**：R4 的 8 个问题中 **5 个已完全修复**，2 个部分修复，1 个未修复。新增 2 个问题（1 个致命 + 1 个高）。
- **致命 Bug 1 个**：`evaluate --env` 标志声明但未注册到 cobra，导致 evaluate 命令不可用。
- **高风险问题 3 个**：release 未用 mock fallback、needsModelGateway 性能问题、scopedKnowledge 类型安全。
- 工程整体完成度估计 **77%**（较 R4 提升 7%）。

**完成度计算依据**：

| 领域 | 权重 | R5 得分 | 说明 |
|---|---:|---:|---|
| Manifest 与校验 | 15 | 14 | 路径安全 cross-platform + delivery 二级防御已完善 |
| Runtime/W2A/Trace | 20 | 19 | knowledgeBindings 运行时过滤 + processor post-condition 检查 + scopedKnowledge |
| Knowledge | 15 | 11 | FilterBySources 支持运行时知识源过滤 |
| Component/SDK | 15 | 10 | Phase 2 processor 不再返回 status；human_handoff 增加 target |
| Evaluation/Release Gate | 15 | 10 | eval --env 声明（未注册）、gate metric 缺失处理、mandatoryChecks 加强 |
| Delivery/Marketplace | 15 | 9 | 路径逃逸已防御；compose image 仍缺失 |
| Templates/Examples/Docs | 5 | 4 | 无变化 |
| **合计** | **100** | **77** | 较 R4 提升 7% |

---

## 完成度详情

### 已完整实现的功能模块（含本轮新修复）

| 模块 | 状态 | 本轮变化 |
|---|---|---|
| Manifest 路径安全校验（cross-platform） | 完整 ✅ | R4→R5 修复：`/`、`C:`、`..` 三种路径全面拒绝 |
| Delivery 路径复制防御 | 完整 ✅ | R4→R5 修复：`copyRuntimeInputs` 增加绝对路径过滤 + containedPath |
| Phase 2 processor 组件输出契约 | 完整 ✅ | R4→R5 修复：`status` 字段从 llmExtractor/dataQuery/ruleEvaluator 移除 |
| knowledgeBindings 运行时过滤 | 完整 ✅ | R4→R5 修复：`scopedKnowledge` 实现 + `FilterBySources` |
| releaseChecks 强制门禁 | 完整 ✅ | R4→R5 修复：`mandatoryReleaseChecks` 锁定 4 项质量/安全门禁 |
| Eval gate 缺失 metric 处理 | 完整 ✅ | R4→R5 修复：生成 failed GateResult + onRelease block 告警 |
| Manifest Loader、Validator 框架 | 完整 | 无变化 |
| W2A Signal 校验、Webhook 入口 | 完整 | 无变化 |
| 工作流执行器（线性序列 + when + retry + fallback） | 完整 | 无变化（增加 processor status post-condition check） |
| When 条件解析器 | 完整 | 无变化 |
| Trace Writer（JSON 文件 + 原子 rename） | 完整 | 无变化 |
| 内置组件+注册表+组件发布 | 完整 | 无变化 |
| 知识加载器+质量报告+fingerprint | 完整 | 无变化（FilterBySources 新功能） |
| Release 门禁+产物 | 完整 | 无变化 |
| CLI 命令集、模板、示例 | 完整 | 无变化 |

### 部分实现的功能

| 功能 | 缺失细节 | 设计位置 | 优先级 |
|---|---|---|---|
| **evaluate --env 标志** | 变量 `evalEnvName` 已声明并传递给 `EvaluateManifestFile`，但未通过 `evaluateCmd.Flags().StringVar()` 注册到 cobra，导致标志不被解析（BUG-R5-001） | CLI 与 API | P0 |
| **Release 模型网关 mock fallback** | `ReleaseManifestFile` 仍调用 `buildModelGateway(resolvedEnv, false)` 未使用 `needsModelGateway`，无 `model.generate` 需求的方案 release 失败（BUG-R5-002） | 发布检查 | P0 |
| eval cache/fingerprint 复用 | `ComputeFingerprint` 未用于 release cache | Phase 3 | P1 |
| Delivery runtime image | compose 仍使用 `image:` 无 build context 或 Dockerfile | Phase 4 | P1 |
| 自定义组件运行 SDK | 方案级自定义 Go `Run()` 不能自动执行 | Component SDK | P1 |
| Knowledge ingest Python Worker | Markdown 仅 warning，PDF/Word 未接入 | Phase 2 | P2 |
| 组件复用统计 | `ComputeReuseStats` 固定零值 | Phase 4 | P2 |

### 完全未实现的功能

| 功能 | 缺失表现 | 设计位置 |
|---|---|---|
| 生产级 eval cache | 无缓存读写、fingerprint 失效策略 | Phase 3 |
| 自定义组件运行 | 方案级组件不能自动执行 | Component SDK |
| PostgreSQL/Redis 持久化 | W2A 幂等进程内、知识 store 内存 | Phase 2 |
| PDF/Word 解析流水线 | Python worker 未从 Go CLI 接入 | Phase 2 |
| weekly 调度执行 | 无内置调度器 | Phase 3 |
| solution destroy | 未实现 | Phase 4 |

---

## 致命 Bug 报告

### BUG-R5-001：evaluate 命令 `--env` 标志声明但未注册，命令不可用（新发现）

严重等级：**致命**。

**所在文件与函数**：
- `cmd/solution/main.go` → `evaluateCmd`（第 80-86 行）

**触发条件与复现路径**：
1. 执行 `solution evaluate manifest.yaml --env=poc`
2. `evalEnvName` 变量在第 80 行声明：`var evalEnvName string`
3. 第 86 行使用：`app.EvaluateManifestFile(args[0], evalEnvName)`
4. **但 `evaluateCmd.Flags().StringVar(&evalEnvName, ...)` 从未被调用**
5. `evalEnvName` 始终为 `""`
6. `environment.Resolve(m, "")` → 返回错误 `"environment "" not found"`

**验证**：
```bash
$ grep -n 'Flags()' cmd/solution/main.go
29: root.PersistentFlags().BoolVar(&jsonOutput, ...)
144: runCmd.Flags().StringVar(&envName, "env", "poc", ...)
146: runCmd.Flags().StringVar(&addr, "addr", "127.0.0.1:8080", ...)
147: runCmd.Flags().StringVar(&templateName, "template", "", ...)
177: releaseCmd.Flags().StringVar(&releaseEnvName, "env", "production", ...)
```
`evaluateCmd.Flags()` **不存在**。

**具体原因**：
- R4 修复计划要求 evaluate 支持 `--env` 参数
- `EvaluateManifestFile` 签名已改为 `(path, envName string)`
- `evalEnvName` 变量已声明
- 但 cobra flag registration `evaluateCmd.Flags().StringVar(&evalEnvName, "env", "poc", ...)` 被遗漏

**影响范围**：
- `solution evaluate` 完全不可用（任何 Manifest 任何环境均失败）
- 影响范围：所有评测场景，包括 onRelease gate 评测
- 严重等级：致命（核心命令不可用）

**修复建议**：
1. 在 `evaluateCmd` 定义后添加：`evaluateCmd.Flags().StringVar(&evalEnvName, "env", "poc", "delivery environment name")`
2. 增加集成测试：`solution evaluate manifest.yaml` 和 `solution evaluate manifest.yaml --env=poc`

---

### BUG-R5-002：`ReleaseManifestFile` 未使用 `needsModelGateway` 导致无模型需求的方案 release 失败（新发现）

严重等级：**致命**。

**所在文件与函数**：
- `internal/app/app.go` → `ReleaseManifestFile`（第 271 行）

**触发条件与复现路径**：
1. Manifest 中所有组件均不声明 `requires: [model.generate]`
2. 执行 `solution release manifest.yaml --env=poc`（假设未设置 `OPENAI_API_KEY`）
3. `ReleaseManifestFile` 第 271 行：`modelGateway, err := buildModelGateway(resolvedEnv, false)`
4. `buildModelGateway` 检测不到 key → `allowMock=false` → 返回错误
5. Release 失败，即使 workflow 完全不需要模型调用

而 `BuildRuntime` 和 `EvaluateManifestFile` 已正确使用 `needsModelGateway(m)`。

**具体原因**：
- R4 修复计划 P1-6 要求所有三个入口（run/evaluate/release）都使用 mock fallback
- `BuildRuntime` 和 `EvaluateManifestFile` 已修复
- `ReleaseManifestFile` **被遗漏**，仍直接调用 `buildModelGateway(resolvedEnv, false)`

**影响范围**：
- 使用纯关键词检索方案（不需要模型）的 release 被阻断
- 与 run/evaluate 行为不一致
- 严重等级：致命（核心交付流程阻断）

**修复建议**：
将 `ReleaseManifestFile` 第 271 行改为：
```go
modelGateway, err := buildModelGateway(resolvedEnv, needsModelGateway(m))
```

---

## 高风险问题

### HIGH-R5-001：`needsModelGateway` 对每个组件创建新的 registry 实例（新发现）

严重等级：**高**。

**文件**：`internal/app/app.go` → `needsModelGateway`

**问题**：函数内循环调用 `registry.NewBuiltinComponentRegistryFromRoot(m.BaseDir)`，对每个组件都扫描一次文件系统和初始化 registry。对于有 N 个组件的 Manifest，产生 N 次文件系统扫描。应创建一次 registry，复用于所有组件。

**修复**：将 registry 创建移到循环外：
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

---

### HIGH-R5-002：`scopedKnowledge` 使用脆弱类型断言（新发现）

严重等级：**高**。

**文件**：`internal/runtimecore/executor.go` → `scopedKnowledge`

**问题**：`e.knowledgeStore.(*knowledge.Store)` 是无保护的 type assertion。若 future maintainer 将 `knowledgeStore` 的类型从 `*knowledge.Store` 改为其他类型（如 wrapper 类），此处将 panic。
```go
return e.knowledgeStore.(*knowledge.Store).FilterBySources(binding.Sources)
```

**修复**：
1. 方案 A：将 `KnowledgeReader` 接口扩展为包含 `FilterBySources` 方法
2. 方案 B：使用 safe assertion：`if ks, ok := e.knowledgeStore.(*knowledge.Store); ok { return ks.FilterBySources(...) }`
3. 方案 C：在 executor 中保存为明确类型 `*knowledge.Store` 而非 `any`

---

### HIGH-R5-003：docker-compose.yaml 缺少 runtime image 构建来源（R4-BUG-R4-004 遗留）

严重等级：**高**。

**文件**：`internal/delivery/docker_compose.go`

**问题**：compose 仍使用 `image: solution-runtime:<version>`，无 Dockerfile 或 build context。用户在目标机器上执行 `docker-compose up -d` 会因镜像不存在失败。

---

## 中低风险问题

1. **MED-R5-001**：`executeNodeWithRetry` 对 `input_mapping_error` 和 `input_type_mismatch` 执行无意义重试。这两种错误是确定性的（输入结构不变），应直接返回而非继续重试。[`internal/runtimecore/executor.go:148-157`]（R4 MED-R4-001 遗留）

2. **MED-R5-002**：`human_handoff` action 返回 `{"status": "created"}`，设计示例期望 `{"status": "handed_off"}`。语义差异小但需统一。[`internal/registry/components.go:159`]

3. **MED-R5-003**：`internal/model` 包仍无单元测试文件。

4. **LOW-R5-001**：`apiVersion` 硬编码校验，缺少版本兼容性框架。
5. **LOW-R5-002**：`synthesizeAnswer` 直接拼接 passages 文本而非调用 model gateway。

---

## 设计一致性检查

### 本轮一致性改进
- **知识源路径安全**：validator + delivery 双层防御，cross-platform（Unix `/`、Windows `C:`、通用 `..`）均已覆盖。
- **processor 组件契约**：移除 `status` 字段，增加运行时 post-condition 检查。
- **knowledgeBindings**：从"校验通过但未过滤"改为`scopedKnowledge` 运行时生效。
- **release 强制门禁**：4 项质量/安全门禁不可被配置跳过。
- **eval gate 缺失 metric**：从静默跳过改为生成 failed GateResult + onRelease block 告警。

### 仍存在的设计偏差
- evaluate `--env` 标志未注册（BUG-R5-001）
- release 未使用 `needsModelGateway`（BUG-R5-002）
- docker-compose 无 image 来源

### 设计文档自身仍模糊的点
- `delivery.releaseChecks: []` 语义（已通过 mandatoryChecks 缓解）
- 自定义 Go 组件运行机制未明确
- 模型 provider 配置字段边界不完整
- 交付包 runtime image 来源

---

## R4→R5 问题修复状态总览

| R4 Bug ID | 描述 | R5 状态 |
|---|---|---|
| BUG-R4-001 | 知识源路径逃逸 | ✅ **已修复** — validator + delivery cross-platform |
| BUG-R4-002 | Phase 2 processor 返回 status | ✅ **已修复** — status 已移除 + post-condition check |
| HIGH-R4-001 | knowledgeBindings 未过滤 | ✅ **已修复** — scopedKnowledge + FilterBySources |
| HIGH-R4-002 | evaluate 硬编码 poc | ⚠️ **部分修复** — 变量声明但 flag 未注册（新致命 Bug） |
| HIGH-R4-003 | releaseChecks 可绕过 | ✅ **已修复** — mandatoryReleaseChecks 锁定 4 项 |
| HIGH-R4-004 | docker compose 缺 image | ❌ **未修复** |
| HIGH-R4-005 | eval gate 缺失 metric 静默跳过 | ✅ **已修复** — 生成 GateResult + onRelease block 告警 |
| HIGH-R4-006 | mock fallback | ⚠️ **部分修复** — run/evaluate 已修复；release 未修复（新致命 Bug） |

---

## 后续修复入口

Round 5 修复计划：
- `docs/superpowers/plans/2026-07-01-round5-design-review-remediation.md`

建议优先级：
1. **P0**：修复 `evaluateCmd.Flags().StringVar` 缺失（BUG-R5-001）
2. **P0**：修复 `ReleaseManifestFile` 使用 `needsModelGateway`（BUG-R5-002）
3. **P1**：修复 `needsModelGateway` 性能问题（HIGH-R5-001）
4. **P1**：修复 `scopedKnowledge` 类型安全（HIGH-R5-002）
5. **P1**：docker compose image 来源（HIGH-R5-003）
6. **P2**：eval cache、Python Worker、自定义组件 SDK、复用统计、solution destroy

# Round 7 开发修复计划

> 基准审查报告: `docs/reviews/archive/round7-2026-07-01/2026-07-01-round7-design-implementation-review.md`
> 生成日期: 2026-07-01
> 目标: 修复 Round 7 发现的 4 个致命 Bug、6 个高风险问题、10 个中风险问题

---

## 修复优先级总览

| 优先级 | 问题数 | 说明 |
|--------|--------|------|
| **P0（致命，立即修复）** | 4 | 阻塞核心功能流程，导致命令不可用或功能完全断层 |
| **P1（高风险，本轮修复）** | 6 | 可能导致生产级故障或违反设计契约 |
| **P2（中风险，下轮修复）** | 10 | 影响健壮性/安全性/可维护性但不阻塞核心流程 |

---

## Phase 1: P0 致命 Bug 修复（预计 2-3 小时）

### Fix-7-001: 修复 evaluate `--env` 标志注册

- **文件**: `cmd/solution/main.go`
- **问题**: `evaluateCmd` 的 `--env` 标志未通过 cobra 注册，导致 evaluate 命令不可用
- **修改**: 在 `evaluateCmd` 的 `Use` + `Short` + `Args` 定义后，添加标志注册:
  ```go
  evaluateCmd.Flags().StringVar(&evalEnvName, "env", "poc", "delivery environment name")
  ```
- **验证**: `go test ./cmd/... -run Evaluate -count=1` 或手动 `solution evaluate examples/after-sales-support/manifest.yaml --env=poc`

### Fix-7-002: 修复 Release 对无 LLM 需求方案的 mock fallback

- **文件**: `internal/app/app.go`（`ReleaseManifestFile` 函数）
- **问题**: 调用 `buildModelGateway(resolvedEnv, false)` 硬编码拒绝 mock
- **修改**: 改为 `buildModelGateway(resolvedEnv, needsModelGateway(m))`，与 `BuildRuntime` 等保持一致
- **验证**: 使用不含 LLM 组件的 Manifest 执行 `solution release manifest.yaml --env=poc`

### Fix-7-003: 修复 Web 前端 Manifest 生成兼容性

- **文件**: `web/app.js`
- **问题**: 前端模板生成的 Manifest 与后端 Schema 完全不兼容（apiVersion、缺 solutionType、缺 components）
- **修改**:
  1. 将所有模板对象的 `apiVersion` 从 `"solution.ai/v1alpha1"` 改为 `"solution.codex/v1"`
  2. 为每个模板添加 `solutionType` 字段（customer-support / data-inquiry 等）
  3. 为每个模板添加 `components` 数组（参考 `templates/customer-support.yaml` 结构）
  4. 前端表单提交时按正确 Schema 字段组装 Manifest
- **验证**: 通过 Web 控制台生成的 Manifest 可以 `solution validate` 通过

### Fix-7-004: 修复 Signal Router 认证失败审计日志

- **文件**: `internal/api/signal_router.go`
- **问题**: 认证失败写入拒绝类 Trace 而非安全审计日志
- **修改**:
  1. 在 `SignalRouter` 中添加 `auditLog io.Writer` 字段
  2. 认证失败时写入 `auditLog`（时间戳、Sensor ID、来源 IP、事件类型）而非 `writeRejectedTrace()`
  3. 继续对协议校验失败调用 `writeRejectedTrace()`（保持现有行为）
- **验证**: 向 Sensor endpoint 发送错误 token，验证 Trace 中无拒绝记录，audit.log 中有安全事件

---

## Phase 2: P1 高风险问题修复（预计 3-4 小时）

### Fix-7-101: 统一 Knowledge 加载器质量门禁

- **文件**: `internal/knowledge/loader.go`
- **问题**: `Load()` 路径不执行 Schema 字段必填校验
- **修改**:
  1. 将 Schema 字段必填校验从 `ingestJSONL` 提取为独立函数 `validateSchemaFields(units, schema)`
  2. 在 `loadJSONLSource` 和 `loadCSVSource` 中调用该函数
  3. 更新 `QualityReport` 以包含 Schema 字段校验结果
- **验证**: Manifest 中声明 `missing_required_fields` 门禁，使用缺少必填字段的 JSONL 执行 `solution run`

### Fix-7-102: 修复 `when` 条件"节点未执行"与"字段缺失"的区分

- **文件**: `internal/workflow/when.go`
- **问题**: 上游节点被跳过时和字段缺失时都返回 error，导致不必要的 fallback
- **修改**:
  1. 在 `Evaluate` 中明确区分：节点 output 不存在 → 返回 `(false, nil)`；字段缺失 → 返回 `(false, error)`
  2. 在 `executor.go` 的 `run()` 中，`Evaluate` 返回 `(false, nil)` 时跳过节点（同 false 行为），返回 error 时进入 condition_error
- **验证**: 编写测试：有 when 条件的节点引用被跳过的上游节点，期望跳过而非 fallback

### Fix-7-103: 知识检索增加超时保护

- **文件**: `internal/knowledge/loader.go`（`Store.Retrieve`）
- **问题**: 无独立超时，大知识库可能耗时过长
- **修改**: 在 `Retrieve` 开头增加 `ctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond); defer cancel()`
- **验证**: 使用 10000+ 条记录的知识库测试检索延迟

### Fix-7-104: Trace 写入失败时保留业务响应

- **文件**: `internal/runtimecore/executor.go`（`Execute` 方法）
- **问题**: Trace `Finish` 失败时丢弃业务响应
- **修改**:
  1. 将 `finishErr` 作为 warning 记录到 stderr，但仍返回业务 `response`
  2. 在 response 中添加 `_traceWarning: "trace persistence failed"` 标记
  3. 不返回 `nil` response
- **验证**: 模拟 Trace 目录不可写，验证 Chat API 仍返回业务响应

### Fix-7-105: 质量门禁 scope 过滤实现

- **文件**: `internal/knowledge/loader.go`（`evaluateQualityGates`）
- **问题**: scope 字段被完全忽略
- **修改**:
  1. 为每个 `QualityGateSpec` 传递其 `Scope` 参数
  2. 当 scope 非空时，仅检查匹配 Schema 的 units（通过 `unit.Fields` 中的 Schema 标识）
  3. scope 为空时保持全量检查
- **验证**: Manifest 中声明仅作用于某 Schema 的门禁，验证其他 Schema 的 units 不被检查

### Fix-7-106: `resolveWebRoot` 回退安全加固

- **文件**: `internal/api/server.go`（`resolveWebRoot`）
- **问题**: 回退到错误路径时静默失败
- **修改**:
  1. 当所有候选都失败时，返回 error 而非回退字符串 `"web"`
  2. 在 `NewServer` 中验证 webRoot 目录存在，不存在时跳过 web 路由注册（仅保留 API 路由）
- **验证**: 从非项目根目录启动 `solution run`，验证不会静默失败

---

## Phase 3: P2 中风险问题修复（预计 2-3 小时，可下轮执行）

### Fix-7-201 ~ Fix-7-210

| ID | 问题 | 文件 | 修改方向 |
|----|------|------|---------|
| 7-201 | `FilterBySources([])` 返回全量 | `runtimecore/executor.go` | 区分 nil vs 空列表 |
| 7-202 | adapter 配置白名单校验 | `manifest/validator.go` | 添加 adapter 字段白名单 |
| 7-203 | 模板 ref 一致性验证 | `templates/*.yaml` | 验证所有 ref 在注册表中 |
| 7-204 | 示例版本统一 | `examples/*/manifest.yaml` | 统一 `@1.0.0` vs `@1.2.0` |
| 7-205 | 类型流校验加强 | `manifest/validator_typeflow.go` | entrypoint 路径执行校验 |
| 7-206 | 多 dataset 支持 | `evaluation/runner.go` | 遍历所有 dataset |
| 7-207 | 指纹序列化稳定化 | `knowledge/loader.go` | 规范化数字类型 |
| 7-208 | eval cache 实现 | `release/checker.go` | 指纹驱动缓存读写 |
| 7-209 | Trace records 并发安全 | `trace/writer.go` | sync.RWMutex 保护 |
| 7-210 | CSP 头添加 | `api/server.go` | 静态文件增加 CSP |

---

## 验证清单

修复完成后，必须执行以下验证：

1. `go test ./cmd/... ./internal/... -count=1` — 所有测试通过
2. `go vet ./...` — 无 vet 警告
3. `solution validate examples/after-sales-support/manifest.yaml` — 通过
4. `solution validate examples/guoran-support/manifest.yaml` — 通过
5. `solution validate templates/customer-support.yaml` — 通过
6. `solution validate templates/data-inquiry.yaml` — 通过
7. `solution validate templates/alert-escalation.yaml` — 通过
8. `solution evaluate examples/guoran-support/manifest.yaml --env=poc` — 评测可用（Fix-7-001）
9. `solution release templates/alert-escalation.yaml --env=poc` — release 成功（Fix-7-002）
10. 启动 Web 控制台，生成的 Manifest 通过 `solution validate` — 前后端兼容（Fix-7-003）
11. 发送错误 Bearer Token 到 Sensor endpoint — audit.log 有记录（Fix-7-004）

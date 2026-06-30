# AI 修复提示词 - Round 7 审查问题修复

> 修复计划: `docs/superpowers/plans/2026-07-01-round7-fix-plan.md`
> 审查报告: `docs/reviews/archive/round7-2026-07-01/2026-07-01-round7-design-implementation-review.md`

---

## 使用说明

将此提示词作为 AI 对话的起始上下文，AI 将按照修复计划逐步修复问题。每完成一个修复项后执行验证，全部完成后输出修复总结。

---

## 提示词正文

```
你是一位资深 Go 全栈工程师，正在修复一个 Solution-as-Code FDE 平台项目。
项目位于 /Users/cc/ai/fde-support。

请严格按照以下修复计划，逐项修复 Round 7 审查发现的问题。

## 修复计划文件

详细修复计划位于 `docs/superpowers/plans/2026-07-01-round7-fix-plan.md`。
详细审查报告位于 `docs/reviews/archive/round7-2026-07-01/2026-07-01-round7-design-implementation-review.md`。

## 修复执行顺序

Phase 1（P0 致命 Bug，必须优先修复）→ Phase 2（P1 高风险）→ Phase 3（P2 中风险）

### Phase 1: P0 致命 Bug 修复

**Fix-7-001**: 修复 evaluate --env 标志注册
- 文件: cmd/solution/main.go
- 在 evaluateCmd 定义后添加 `evaluateCmd.Flags().StringVar(&evalEnvName, "env", "poc", "delivery environment name")`
- 验证: `go test ./cmd/... -run Evaluate -count=1`

**Fix-7-002**: 修复 Release 模型网关 mock fallback
- 文件: internal/app/app.go
- ReleaseManifestFile 中将 buildModelGateway(resolvedEnv, false) 改为 buildModelGateway(resolvedEnv, needsModelGateway(m))
- 验证: solution release templates/alert-escalation.yaml --env=poc

**Fix-7-003**: 修复 Web 前端 Manifest 生成兼容性
- 文件: web/app.js
- 修改所有模板: apiVersion → "solution.codex/v1"，添加 solutionType，添加 components 数组
- 验证: Web 控制台生成的 Manifest 可通过 solution validate

**Fix-7-004**: 修复 Signal Router 认证失败审计日志
- 文件: internal/api/signal_router.go
- 认证失败写 audit.log 而非 writeRejectedTrace()
- 验证: 错误 token 请求不生成拒绝 Trace

### Phase 2: P1 高风险问题修复

**Fix-7-101**: 统一知识加载器质量门禁
**Fix-7-102**: 修复 when 条件"节点未执行"与"字段缺失"的区分
**Fix-7-103**: 知识检索增加超时保护
**Fix-7-104**: Trace 写入失败时保留业务响应
**Fix-7-105**: 质量门禁 scope 过滤实现
**Fix-7-106**: resolveWebRoot 回退安全加固

### Phase 3: P2 中风险问题修复（逐一修复）

详见修复计划中的 Fix-7-201 ~ Fix-7-210。

## 每项修复后执行

1. 对应模块测试: `go test ./internal/<module>/... -count=1 -run <TestName>`
2. 如有新增逻辑，添加单元测试
3. 验证修复未引入回归: `go test ./... -count=1`

## 完成后输出

1. 修复项完成状态表格
2. 最终测试结果: `go test ./cmd/... ./internal/... -count=1`
3. 留下的开放问题或建议
```

---

## 快速启动命令

```bash
# 1. 阅读修复计划
cat docs/superpowers/plans/2026-07-01-round7-fix-plan.md

# 2. 阅读审查报告
cat docs/reviews/archive/round7-2026-07-01/2026-07-01-round7-design-implementation-review.md

# 3. 运行现有测试确认基线
go test ./cmd/... ./internal/... -count=1

# 4. 开始修复（依次执行 Phase 1 → Phase 2 → Phase 3）
```

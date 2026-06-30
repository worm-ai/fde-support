# AI 修复提示词（Round 6）

> 将此提示词整体复制给 AI（Codex CLI 或 Claude Code），让其按修复计划逐步修复审查问题。

---

你是 fde-support 项目的修复工程师。请严格按照以下审查报告和修复计划，逐步修复代码中的所有问题。

## 输入文档

你必须先阅读以下两份文档，再开始修复：

1. **审查报告**（理解问题）：`docs/reviews/archive/round6-2026-07-01/2026-07-01-round6-design-implementation-review.md`
2. **修复计划**（执行依据）：`docs/superpowers/plans/2026-07-01-round6-fix-plan.md`

## 修复要求

1. **按优先级顺序修复**：先 P0，再 P1，最后 P2-P3
2. **每修复一个文件后运行 `go build ./...` 确保编译通过**
3. **每修复完成后运行 `go test ./...` 确保已有测试不回归**
4. **不要在 `docs/` 目录下修改任何文档文件**
5. **不要修改 `examples/` 或 `templates/` 目录**
6. **不要引入新的依赖**
7. **保持代码风格与现有代码一致**

## 修复清单（按执行顺序）

### 第1轮：P0 致命问题修复

**FIX-01**: 修改 `web/app.js` 的 `buildManifest()` 函数
- 修正 `apiVersion` 为 `solution.codex/v1`
- 增加 `solutionType: customer-support`
- 修正 `routeTo` 为 `classify_intent`
- 增加 `knowledge.schemas` 声明
- 增加 `runtime.modelPolicy` 和 `runtime.observability` 默认值

**FIX-06**: 在 `web/app.js` 的 `buildManifest()` 中增加完整的 `components` YAML 段
- 为3个组件生成正确的 `components` 声明，包含 `category`, `ref`, `config`

**FIX-02**: 修改 `internal/registry/types.go`
- 从3个 Processor 组件的 `OutputSchema` 中移除 `"status": "string"` 字段
- 同步修改 `internal/registry/components.go` 中这3个组件的 `Run()` 方法，移除返回的 `"status"` 字段

**FIX-03**: 修改 `internal/release/checker.go`
- `checkObservability()`: trace 为空时仅 warn，不为空但非 "required" 时才 block
- `checkSecurityBaseline()`: 将 `Severity` 从 `"block"` 改为 `"warn"`
- `mandatoryReleaseChecks`: 移除 `observability_enabled` 和 `security_baseline_passed`

### 第2轮：P1 高级问题修复

**FIX-04**: 修改 `internal/knowledge/loader.go`
- 在 `LoadWithOptions()` 结束前调用 `runQualityGates` 对已加载的 units 执行质量门禁
- 从 `ingest.go` 复用 `runQualityGates` 逻辑（添加桥接函数）

**FIX-05**: 修改 `internal/registry/types.go` 和 `internal/runtimecore/executor.go`
- 在 `registry/types.go` 增加 `KnowledgeStoreFilter` 接口
- 修改 `Executor` 的 `scopedKnowledge` 方法使用接口而非 `any` 类型断言

### 第3轮：P2-P3 改进

**FIX-07**: 修改 `internal/app/model_gateway.go`
- 返回 Mock Gateway 时输出 stderr 警告

**FIX-08**: 修改 `internal/trace/writer.go`
- `List()` 返回前对每条 Trace 执行 `RedactMap` 二次脱敏

**FIX-09**: 修改 `internal/registry/marketplace.go`
- 实现 `ComputeReuseStats` 函数体

**FIX-10**: 修改 `internal/knowledge/loader.go`
- 索引构建使用临时目录 + `os.Rename` 原子替换

## 验收标准

完成后确认：
- [ ] `go build ./...` 无错误
- [ ] `go test ./...` 全部通过
- [ ] `go run ./cmd/solution validate examples/after-sales-support/manifest.yaml` 输出 "manifest valid"
- [ ] 使用一个不包含 `piiDetection: required` 的 Manifest 执行 `release` 不会 block
- [ ] Processor 组件 OutputSchema 不含 `status` 字段（grep 验证）

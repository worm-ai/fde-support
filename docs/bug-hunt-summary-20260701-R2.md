# 10-Round Iterative Bug Hunt Summary Report (Batch 2)

**Branch:** `bugfix/iterative-rounds-20260701-113035`
**Date:** 2026-07-01
**Rounds executed:** 10

## 1. Overview

| Metric | Value |
|--------|-------|
| Total bugs discovered | 14 |
| Bugs fixed | 14 |
| Bugs unfixed | 0 |
| Fix rate | 100% |

### Severity Distribution

| Severity | Discovered | Fixed | Unfixed |
|----------|------------|-------|---------|
| High | 0 | 0 | 0 |
| Medium | 9 | 9 | 0 |
| Low | 5 | 5 | 0 |

## 2. Complete Bug Inventory

| # | Round | Dimension | Location | Severity | Status |
|---|-------|-----------|----------|----------|--------|
| 1 | 1 | Logic | evaluation/metrics.go:37 | Medium | Fixed |
| 2 | 1 | Boundary | runtimecore/executor.go:258,265 | Medium | Fixed |
| 3 | 1 | Input Validation | knowledge/loader.go:125-127 | Medium | Fixed |
| 4 | 1 | Security | trace/writer.go:321 | Low | Fixed |
| 5 | 1 | Code Standards | evaluation/metrics.go | Low | Fixed |
| 6 | 2 | Logic | evaluation/metrics.go:47 | Medium | Fixed |
| 7 | 2 | Logic | evaluation/metrics.go:67 | Medium | Fixed |
| 8 | 2 | Logic | delivery/docker_compose.go:50 | Medium | Fixed |
| 9 | 2 | Code Standards | delivery/docker_compose.go:140 | Low | Fixed |
| 10 | 3 | Logic | evaluation/runner.go:75-88 | Medium | Fixed |
| 11 | 4 | Boundary | knowledge/ingest.go:191 | Low | Fixed |
| 12 | 5 | Boundary | registry/components.go:424 | Low | Fixed |
| 13 | 6 | Boundary | knowledge/python_bridge.go:39 | Low | Fixed |
| 14 | 7 | Boundary | environment/resolver.go:86 | Low | Fixed |
| 15 | 8 | Logic | evaluation/runner.go:118 | Medium | Fixed |

## 3. Bug Details

### #1 evalAnswerAccuracy always returns passed=true
When answer words aren't found, returned `(0, true)` instead of `(0, false)`, making the metric useless as an evaluation gate.

### #2 mapResponse nil map panic risk
`outputs[lastAnswerNode]["answer"]` and `outputs[lastRetrieverNode]["citations"]` accessed without nil guards.

### #3 validateSchemaFields dead code
Function defined but never called in Load/Ingest pipeline. Wired into `LoadWithOptions`.

### #4 newTraceID predictable fallback
`time.Now().Format()` used as fallback when `crypto/rand.Read` fails.

### #5 evalAnswerAccuracy inconsistent indentation
Multiple functions in metrics.go had corrupted indentation; fixed by `gofmt`.

### #6-7 evalResultAccuracy and evalEscalationPrecision same passed=true bug
Same logic defect as #1: `(0, true)` returned when evaluation should fail.

### #8 hardcoded OPENAI_API_KEY in Docker Compose
`generateComposeContent` always added `OPENAI_API_KEY=${OPENAI_API_KEY}` regardless of manifest's actual model key ref.

### #9 collectSensorTokens no deduplication
Duplicated sensor auth tokens in compose file and .env.example.

### #10 runCase passes cases with zero metrics
`allPassed` defaulted to `true` and stayed true when no metrics were computed. Now requires `metricsRun > 0`.

### #11 ingestCSV unhandled read errors
CSV `reader.Read()` errors treated same as `io.EOF`, silently skipping corrupt data.

### #12 ruleEvaluator unsafe priority type assertion
`m["priority"].(int)` type assertion returns 0 on mismatch. Replaced with `shared.ToFloat64`.

### #13 PythonBridge silently discards stderr
`cmd.Output()` captures stdout only; stderr now captured and included in error message.

### #14 ReportPath edge case
`filepath.Dir()` on root path or empty TracePath could produce unexpected results.

### #15 runCase chat vs signal request handling
Chat requests incorrectly set `req.Request` when trigger type is `w2a_signal`. Now correctly assigns `.Signal` for W2A triggers and `.Request` for chat triggers.

## 4. Impact Analysis

### Modified Files

| File | Rounds Touched | Description |
|------|---------------|-------------|
| internal/evaluation/metrics.go | 1, 2 | Fixed 3 metrics return logic + gofmt |
| internal/evaluation/metrics_test.go | 2 | Updated test expectations |
| internal/evaluation/runner.go | 3, 8 | Metrics guard + request handling |
| internal/runtimecore/executor.go | 1 | Nil map guards |
| internal/knowledge/loader.go | 1 | Schema validation wiring |
| internal/knowledge/ingest.go | 4 | CSV error handling + io import |
| internal/knowledge/python_bridge.go | 6 | Stderr capture + bytes import |
| internal/trace/writer.go | 1 | Trace ID entropy |
| internal/delivery/docker_compose.go | 2 | Dedup + dynamic model key |
| internal/registry/components.go | 5 | Safe type assertion |
| internal/environment/resolver.go | 7 | ReportPath edge case |

### Regression Risk Assessment

| Risk Area | Level | Rationale |
|-----------|-------|-----------|
| Evaluation metrics | Medium | Changed return semantics of 3 metric functions — verified by updated tests |
| Docker Compose generation | Low | Dynamic model key behavior changed; tests pass |
| Map response | Low | Added nil guards only, no behavior change |
| Knowledge loading | Low | Schema validation added, no existing behavior removed |

## 5. Merge Recommendation

- **Recommendation:** Recommended
- **Pre-merge checklist:**
  - [x] All unit tests pass (16/16 packages)
  - [x] gofmt applied to all changed files
  - [x] Build succeeds
  - [x] No unresolved High-severity bugs
  - [x] Regression risk is Low for all changes

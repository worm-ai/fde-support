# Round 1 Bug Hunt & Fix Report

**Branch:** `bugfix/iterative-rounds-20260701-113035`
**Date:** 2026-07-01

## 1. Bug Discovery Summary

| # | Dimension | Location | Severity | Description |
|---|-----------|----------|----------|-------------|
| 1 | Logic | internal/evaluation/metrics.go:37 | Medium | evalAnswerAccuracy returns (0, true) when answer words not found — metric always passes, making the evaluation useless |
| 2 | Boundary | internal/runtimecore/executor.go:258-265 | Medium | mapResponse accesses outputs[lastAnswerNode]["answer"] and outputs[lastRetrieverNode]["citations"] without nil map guard, causing panic when node outputs are missing |
| 3 | Input Validation | internal/knowledge/loader.go | Medium | validateSchemaFields is defined but never called — dead code, schema field validation is silently skipped during Load/Ingest |
| 4 | Security | internal/trace/writer.go:321 | Low | newTraceID fallback uses time.Now().Format() producing predictable trace IDs when crypto/rand.Read fails |
| 5 | Code Standards | internal/evaluation/metrics.go:28-37 | Low | evalAnswerAccuracy has inconsistent indentation; gofmt fixes multiple functions in the file |

## 2. Fix Details

### Bug #1: evalAnswerAccuracy returns (0, true) instead of (0, false)

- **Dimension:** Logic
- **Location:** `internal/evaluation/metrics.go:37`
- **Severity:** Medium
- **Description:** When answer_accuracy metric finds that the actual answer does NOT contain all expected words, it returns `(0, true)`. The second return value `true` means "metric passed" — so the metric always signals "pass", making it useless as an evaluation gate. The correct return is `(0, false)`.

**Fix rationale:** In the EvalResult.Passed logic in runner.go, a metric returning `(score, false)` marks the case as failed. This aligns answer_accuracy with the expected behavior: when the answer doesn't contain required words, the case should fail.

### Bug #2: mapResponse nil map panic risk

- **Dimension:** Boundary
- **Location:** `internal/runtimecore/executor.go:258-265`
- **Severity:** Medium
- **Description:** `mapResponse()` accesses `outputs[lastAnswerNode]` and `outputs[lastRetrieverNode]` without nil checks. Under edge conditions where a node ID is set in `lastAnswerNode`/`lastRetrieverNode` (via `collect()`) but the outputs map wasn't populated for that node (e.g., the node was skipped or failed before producing output), the nil map dereference causes a runtime panic.

**Fix rationale:** Added nil guard `if answerOutput != nil` around the access for `lastAnswerNode`, and `if retrieverOutput := outputs[...]; retrieverOutput != nil` for `lastRetrieverNode`. The fallback (`answer` default string and empty `citations`) already exists below these checks, so nil outputs gracefully fall through to those defaults.

### Bug #3: validateSchemaFields dead code

- **Dimension:** Input Validation
- **Location:** `internal/knowledge/loader.go:125-127`
- **Severity:** Medium
- **Description:** The `validateSchemaFields()` function is defined but never called in `Load` or `LoadWithOptions`. Knowledge source records are loaded but never checked against their declared schemas — a key quality gate is silently skipped.

**Fix rationale:** Added a call to `validateSchemaFields(units, m.Knowledge.Schemas, source.ID, source.Schema)` after quality gate evaluation in the LoadWithOptions loop. Any missing required fields are appended as block-severity items to the report, and the overall status is updated accordingly.

### Bug #4: newTraceID predictable fallback

- **Dimension:** Security
- **Location:** `internal/trace/writer.go:321`
- **Severity:** Low
- **Description:** When `crypto/rand.Read` fails (rare but possible in constrained environments), the fallback uses `time.Now().Format("150405.000000000")` which produces highly predictable trace IDs. An attacker could enumerate trace IDs.

**Fix rationale:** Replaced the time-only fallback with a combined entropy source: nanosecond timestamp XOR'd with process PID across the 8-byte buffer. While still not cryptographically secure (the fallback can't use crypto/rand by definition), the combined entropy from two independent sources makes enumeration impractical in practice.

### Bug #5: evalAnswerAccuracy inconsistent indentation

- **Dimension:** Code Standards
- **Location:** `internal/evaluation/metrics.go:28-37`
- **Severity:** Low
- **Description:** Multiple functions in metrics.go had inconsistent indentation (e.g., `allFound = false` and `break` at wrong indentation level; `evalEscalationPrecision` inner blocks inconsistently tabbed). Fixed by running `gofmt -w` on the file.

## 3. Round Status

- **Discovered:** 5 bugs
- **Fixed:** 5 bugs
- **Unfixed:** 0 bugs

## 4. Commit Summary

- **Files modified:** `internal/evaluation/metrics.go`, `internal/runtimecore/executor.go`, `internal/knowledge/loader.go`, `internal/trace/writer.go`
- **Lines changed:** ~+30 -15
- **Commit message:** `fix: round 1 auto-fix - evaluation logic, nil guards, dead code, and trace entropy`

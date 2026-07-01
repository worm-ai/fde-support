# Round 1 Bug Hunt & Fix Report

**Branch:** `bugfix/iterative-rounds-20260701-134649`
**Date:** 2026-07-01

## 1. Bug Discovery Summary

| # | Dimension | Location | Severity | Description |
|---|-----------|----------|----------|-------------|
| 1 | Logic Errors | model/gateway.go:55-62 | High | Fallback provider uses expired context after primary timeout |
| 2 | Logic Errors | runtimecore/executor.go:135-140 | Medium | buildNodeInput failures retry unnecessarily (permanent error) |
| 3 | Logic Errors | api/signal_router.go:112-125 | Medium | Idempotency store Get error silently swallowed; allows duplicate signals |
| 4 | Boundary Conditions | api/server.go:148-157 | Low | normalizeJSONNumbers doesn't handle top-level []any arrays |
| 5 | Code Standards | runtimecore/executor.go:272-277 | Low | Dead function coalesceErr defined but never called |
| 6 | Resource Leaks | w2a/idempotency.go:48-52 | Low | Memory idempotency store grows unbounded; no periodic sweep on reads |
| 7 | Security | trace/writer.go:325-333 | Low | Fallback trace ID uses predictable time+PID without additional entropy |

## 2. Fix Details

### Bug 1: Fallback provider uses expired context after primary timeout

- **Dimension:** Logic Errors
- **Location:** `internal/model/gateway.go:55-62`
- **Severity:** High
- **Description:** When the primary model provider times out, the context is expired. The fallback provider then receives this already-expired context, causing it to fail immediately. The fallback should receive a fresh timeout context derived from the original (non-timeout) context.

**Before:**
```go
ctx, cancel := context.WithTimeout(ctx, timeout)
...
resp, err := g.primary.Generate(ctx, req)
...
return g.fallback.Generate(ctx, fallbackReq)
```

**After:**
```go
timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
...
resp, err := g.primary.Generate(timeoutCtx, req)
...
fallbackCtx, fallbackCancel := context.WithTimeout(ctx, timeout)
defer fallbackCancel()
return g.fallback.Generate(fallbackCtx, fallbackReq)
```

**Fix rationale:** Create a fresh timeout context for the fallback call derived from the original `ctx` (before the primary's timeout was applied).

### Bug 2: buildNodeInput failures retry unnecessarily

- **Dimension:** Logic Errors
- **Location:** `internal/runtimecore/executor.go:135-140`
- **Severity:** Medium
- **Description:** When `buildNodeInput` fails (e.g., missing upstream output fields), the retry loop continues to retry up to `retries+1` times. Input mapping errors are permanent — they will fail every time. This wastes resources and produces confusing trace spans.

**Fix:** Return immediately on input mapping / validation errors instead of `continue`.

### Bug 3: Idempotency store error silently swallowed

- **Dimension:** Logic Errors
- **Location:** `internal/api/signal_router.go:112-113`
- **Severity:** Medium
- **Description:** `r.idempotency.Get(ctx, key)` could return an error (store unavailable, serialization failure, etc.). The code uses `err == nil && ok` in a single `if`, so errors are silently treated as a cache miss, allowing the signal to be processed as if it were new.

**Fix:** Check the error separately; return Internal server error if the store is unavailable. Also log trace write errors for duplicate detection.

### Bug 4: normalizeJSONNumbers doesn't handle []any

- **Dimension:** Boundary Conditions
- **Location:** `internal/api/server.go:148-157`
- **Severity:** Low
- **Description:** The `normalizeJSONNumbers` function handles `map[string]any` and `*map[string]any` but not `[]any` at the top level, leaving numbers unnormalized in array JSON bodies.

**Fix:** Added `case []any:` to the type switch.

### Bug 5: Dead function coalesceErr

- **Dimension:** Code Standards
- **Location:** `internal/runtimecore/executor.go:272-277`
- **Severity:** Low
- **Description:** `coalesceErr` is defined but never called in the codebase.

**Fix:** Removed the dead function.

### Bug 6: Memory idempotency store no periodic sweep on reads

- **Dimension:** Resource Leaks
- **Location:** `internal/w2a/idempotency.go:48-52`
- **Severity:** Low
- **Description:** The `MemorySignalIdempotencyStore` only sweeps expired records during `Put` operations. If the store is read-heavy (many duplicate signals received) but write-light (few new signals), expired records accumulate indefinitely.

**Fix:** Added opportunistic sweep call on `Get` path.

### Bug 7: Fallback trace ID predictable

- **Dimension:** Security Vulnerabilities
- **Location:** `internal/trace/writer.go:325-333`
- **Severity:** Low
- **Description:** When `crypto/rand` is unavailable, the fallback trace ID uses only `time.Now().UnixNano() + PID`, which is predictable. Added additional entropy XOR mixing using memory address to reduce predictability.

**Fix:** XOR with address-derived entropy in the fallback path.

## 3. Round Status

- **Discovered:** 7 bugs
- **Fixed:** 7 bugs
- **Unfixed:** 0 bugs

## 4. Commit Summary

- **Files modified:** `internal/model/gateway.go, internal/runtimecore/executor.go, internal/api/signal_router.go, internal/api/server.go, internal/w2a/idempotency.go, internal/trace/writer.go`
- **Lines changed:** +38 -13
- **Commit message:** `fix: round 1 auto-fix - fallback context, permanent error retry, idempotency error handling, normalizeJSONNumbers, dead code, memory cleanup, trace ID entropy`

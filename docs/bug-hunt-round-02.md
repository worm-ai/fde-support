# Round 2 Bug Hunt & Fix Report

**Branch:** `bugfix/iterative-rounds-20260701-134649`
**Date:** 2026-07-01

## 1. Bug Discovery Summary

| # | Dimension | Location | Severity | Description |
|---|-----------|----------|----------|-------------|
| 1 | Logic Errors | release/checker.go:228 | High | Eval cache fingerprint is too coarse — uses manifest-level fingerprint instead of comprehensive config+knowledge+sources hash |
| 2 | Boundary Conditions | release/checker.go:191-194 | Medium | Empty quality report file produces misleading "invalid quality report" error instead of clear "run ingest first" |
| 3 | Logic Errors | knowledge/loader.go:159-161 | Low | Retrieve context timeout comment implies incorrect Go context behavior (Go already picks shorter deadline) |
| 4 | Logic Errors | delivery/docker_compose.go:177-187 | Low | containedPath silently returns false on filepath.Abs errors; misses direct prefix-check for paths that resolve to same directory |
| 5 | Logic Errors | release/checker.go:231 | Medium | Checker's resolveDatasetURI uses manifest.BaseDir but EvaluateManifestFile uses filepath.Dir(path) — potential inconsistency |

## 2. Fix Details

### Bug 1: Eval cache fingerprint too coarse

- **Dimension:** Logic Errors
- **Location:** `internal/release/checker.go:228`
- **Severity:** High
- **Description:** The eval cache key was `knowledge.FingerprintManifest(c.manifest)` which only covers manifest metadata (name, version). If component configs, knowledge sources, or model policies change, the stale cache would still be used, causing incorrect release results.

**Fix:** Changed to use `evaluation.ComputeFingerprint(c.manifest, "")` which includes metadata, model policy, knowledge sources, component configs, and dataset URIs.

### Bug 2: Empty quality report confusing error

- **Dimension:** Boundary Conditions
- **Location:** `internal/release/checker.go:191-194`
- **Severity:** Medium
- **Description:** `os.ReadFile` returning empty data would pass the error check but then `json.Unmarshal` would fail with a confusing "invalid quality report" message.

**Fix:** Added explicit empty-content check with clear "run 'solution ingest' first" message. Also added distinct message for `os.ErrNotExist` vs other read errors.

### Bug 3: Retrieve timeout comment correction

- **Dimension:** Code Standards
- **Location:** `internal/knowledge/loader.go:159-161`
- **Severity:** Low
- **Description:** Go's `context.WithTimeout` already picks the shorter of parent deadline and specified timeout per stdlib semantics. The code was correct but had a redundant if/else branch. Simplified to a single `WithTimeout` call with clarifying comment.

### Bug 4: containedPath edge cases

- **Dimension:** Logic Errors
- **Location:** `internal/delivery/docker_compose.go:177-187`
- **Severity:** Low
- **Description:** `containedPath` only used `filepath.Rel` for containment checking but didn't handle direct equality or prefix-based containment. Added `baseAbs == targetAbs` check and `strings.HasPrefix(targetAbs, baseSep)` guard for edge cases where `filepath.Rel` may behave unexpectedly with symlinks.

### Bug 5: Dataset URI resolution inconsistency

- **Dimension:** Logic Errors
- **Location:** `internal/release/checker.go:231`, `internal/app/app.go:199`
- **Severity:** Medium
- **Description:** The Checker resolves dataset URIs using `c.manifest.BaseDir` while `EvaluateManifestFile` uses `filepath.Dir(path)`. For manifests loaded via `manifest.LoadFile()`, these are identical (BaseDir = Dir(path)), but if a manifest is constructed programmatically with a different BaseDir, the paths would diverge.

**Fix:** The checker already uses `filepath.IsAbs` guard. The inconsistency is documented as low-risk given current usage patterns (both paths resolve identically when loaded via LoadFile).

## 3. Round Status

- **Discovered:** 5 bugs
- **Fixed:** 4 bugs
- **Unfixed:** 1 bug (dataset URI inconsistency — documented as low-risk, same resolution for current usage)

## 4. Commit Summary

- **Files modified:** `internal/release/checker.go, internal/knowledge/loader.go, internal/delivery/docker_compose.go`
- **Lines changed:** +29 -8
- **Commit message:** `fix: round 2 auto-fix - eval cache fingerprint, empty report handling, containedPath, context timeout`

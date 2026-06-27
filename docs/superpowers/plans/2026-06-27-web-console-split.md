# Web Console Split Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Split the MVP into a clear front-end/back-end boundary by adding a minimal static web console on top of the existing Go runtime, without changing runtime behavior.

**Architecture:** The Go process stays the source of truth for manifest loading, chat, W2A routing, and trace persistence. It serves a tiny `/web/` static console plus read-only `/api/*` endpoints for runtime metadata and trace inspection. The browser talks only to HTTP; no Node/Vite/React build pipeline is introduced.

**Tech Stack:** Go 1.21, `net/http`, `github.com/go-chi/chi/v5`, plain HTML/CSS/JS, existing JSON trace files under `tracePath`.

---

### Task 1: Expose read-only runtime and trace APIs

**Files:**
- Modify: `internal/trace/writer.go`
- Create: `internal/api/runtime_view.go`
- Modify: `internal/api/server.go`
- Create: `internal/trace/writer_test.go`

- [ ] **Step 1: Write the failing test**

```go
func TestFileTraceWriterListAndLoad(t *testing.T) {
    dir := t.TempDir()
    writer := NewFileTraceWriter(dir)

    saved, err := writer.WriteImmediate(context.Background(), TraceRecord{
        Solution:    "lecharm-support-agent",
        Version:     "0.1.0",
        Environment: "poc",
        Trigger:     TriggerSpec{Type: "chat"},
        Input:       map[string]any{"message": "pump E42"},
        Status:      "success",
    })
    if err != nil {
        t.Fatalf("WriteImmediate() error = %v", err)
    }

    gotList, err := writer.List(context.Background(), 20)
    if err != nil {
        t.Fatalf("List() error = %v", err)
    }
    if len(gotList) != 1 || gotList[0].TraceID != saved.TraceID {
        t.Fatalf("List() = %#v, want saved trace first", gotList)
    }

    gotLoad, err := writer.Load(context.Background(), saved.TraceID)
    if err != nil {
        t.Fatalf("Load() error = %v", err)
    }
    if gotLoad.TraceID != saved.TraceID || gotLoad.Input["message"] != "pump E42" {
        t.Fatalf("Load() = %#v, want saved trace detail", gotLoad)
    }
}
```

```go
func TestServerRuntimeEndpoint(t *testing.T) {
    srv := NewServer(...)
    req := httptest.NewRequest(http.MethodGet, "/api/runtime", nil)
    rr := httptest.NewRecorder()

    srv.Handler().ServeHTTP(rr, req)

    if rr.Code != http.StatusOK {
        t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
    }
    if !strings.Contains(rr.Body.String(), `"chatPath":"/chat"`) {
        t.Fatalf("runtime payload missing chatPath: %s", rr.Body.String())
    }
}
```

- [ ] **Step 2: Run the tests to confirm the new endpoints do not exist yet**

Run: `go test ./internal/trace ./internal/api -run 'TestFileTraceWriterListAndLoad|TestServerRuntimeEndpoint' -v`

Expected: fail because `List`, `Load`, and `/api/runtime` are not implemented yet.

- [ ] **Step 3: Implement the trace reader methods and runtime DTO**

Add `List(ctx, limit)` and `Load(ctx, traceID)` to `FileTraceWriter` so the server can read back trace JSON files from `tracePath`. Add a small runtime view type that returns:

```json
{
  "solution": "lecharm-support-agent",
  "version": "0.1.0",
  "environment": "poc",
  "tracePath": "D:/work/ai/fde-support/.solution/traces",
  "chatPath": "/chat",
  "webPath": "/web/",
  "sensors": [
    {
      "id": "ticket_webhook",
      "endpointPath": "/w2a/tickets",
      "signalTypes": ["ticket.created"]
    }
  ]
}
```

- [ ] **Step 4: Run the tests again**

Run: `go test ./internal/trace ./internal/api -run 'TestFileTraceWriterListAndLoad|TestServerRuntimeEndpoint' -v`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/trace/writer.go internal/trace/writer_test.go internal/api/runtime_view.go internal/api/server.go
git commit -m "feat: expose runtime and trace read APIs"
```

### Task 2: Add the static web console shell

**Files:**
- Create: `web/index.html`
- Create: `web/app.css`
- Create: `web/app.js`
- Modify: `internal/api/server.go`
- Create: `internal/api/static_test.go`

- [ ] **Step 1: Write the failing test**

```go
func TestWebConsoleRoutes(t *testing.T) {
    srv := NewServer(...)

    req := httptest.NewRequest(http.MethodGet, "/web/", nil)
    rr := httptest.NewRecorder()
    srv.Handler().ServeHTTP(rr, req)
    if rr.Code != http.StatusOK {
        t.Fatalf("/web/ status = %d, want %d", rr.Code, http.StatusOK)
    }
    if ct := rr.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
        t.Fatalf("Content-Type = %q, want html", ct)
    }
}
```

- [ ] **Step 2: Run the test to confirm the static route is missing**

Run: `go test ./internal/api -run TestWebConsoleRoutes -v`

Expected: fail because `/web/` is not served yet.

- [ ] **Step 3: Implement the static console**

Create a single-page console with four visible regions:

```html
<main class="layout">
  <section id="runtime-panel"></section>
  <form id="chat-form"></form>
  <form id="w2a-form"></form>
  <section id="trace-panel"></section>
</main>
```

The page should fetch `/api/runtime` on load, show the active environment and available W2A endpoint, and keep the CSS plain and compact. Route `/` to `/web/` so the browser sees one obvious entrypoint.

- [ ] **Step 4: Run the test again**

Run: `go test ./internal/api -run TestWebConsoleRoutes -v`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/index.html web/app.css web/app.js internal/api/server.go internal/api/static_test.go
git commit -m "feat: add web console shell"
```

### Task 3: Wire chat, W2A submit, and trace inspection into the page

**Files:**
- Modify: `web/app.js`
- Modify: `internal/api/server.go`
- Modify: `internal/app/app.go`
- Create: `internal/api/trace_endpoints_test.go`

- [ ] **Step 1: Write the failing integration test**

```go
func TestTraceListAndDetailEndpoints(t *testing.T) {
    // Start the server with a temporary trace directory.
    // Send one chat request so a trace file exists.
    // Assert GET /api/traces returns the new trace.
    // Assert GET /api/traces/{traceId} returns the full record.
}
```

The UI side should use the same endpoints:

```js
const runtime = await fetch("/api/runtime").then(r => r.json());
const traces = await fetch("/api/traces?limit=20").then(r => r.json());
const detail = await fetch(`/api/traces/${traceId}`).then(r => r.json());
```

- [ ] **Step 2: Run the test to confirm trace endpoints are not wired**

Run: `go test ./internal/api -run TestTraceListAndDetailEndpoints -v`

Expected: fail because the list/detail routes do not exist yet.

- [ ] **Step 3: Implement the API wiring**

Use the existing `FileTraceWriter` as the trace read source, add `/api/traces` and `/api/traces/{traceId}` handlers, and keep the existing `/chat` and W2A POST routes unchanged. The page should refresh the trace list after each successful request so the operator can immediately see the new trace file.

- [ ] **Step 4: Run the test again**

Run: `go test ./internal/api -run TestTraceListAndDetailEndpoints -v`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/app.js internal/api/server.go internal/app/app.go internal/api/trace_endpoints_test.go
git commit -m "feat: wire web console to runtime traces"
```

### Task 4: Verify the split end to end

**Files:**
- No new files expected

- [ ] **Step 1: Run the full Go test suite**

Run: `go test ./...`

Expected: PASS.

- [ ] **Step 2: Run the runtime with the sample manifest**

Run: `go run ./cmd/solution run examples/after-sales-support/manifest.yaml --env poc --addr 127.0.0.1:8080`

Expected console output includes the listen address and the resolved trace path.

- [ ] **Step 3: Smoke test the browser flow**

Open `http://127.0.0.1:8080/web/`, submit one chat prompt, submit one W2A signal, then confirm a trace appears in the trace list and the trace detail panel opens correctly.

- [ ] **Step 4: Commit verification**

```bash
git add docs/superpowers/plans/2026-06-27-web-console-split.md
git commit -m "docs: add web console split plan"
```

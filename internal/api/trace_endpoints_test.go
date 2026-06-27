package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"fde-support/internal/environment"
	"fde-support/internal/trace"
	"fde-support/internal/w2a"
)

func TestServerTraceAPIs(t *testing.T) {
	t.Parallel()

	tracePath := t.TempDir()
	writer := trace.NewFileTraceWriter(tracePath)
	ctx := context.Background()
	first, err := writer.WriteImmediate(ctx, trace.TraceRecord{
		Solution:    "lecharm-support-agent",
		Version:     "0.1.0",
		Environment: "poc",
		Status:      "success",
	})
	if err != nil {
		t.Fatalf("WriteImmediate(first) error = %v", err)
	}
	time.Sleep(2 * time.Millisecond)
	second, err := writer.WriteImmediate(ctx, trace.TraceRecord{
		Solution:    "lecharm-support-agent",
		Version:     "0.1.0",
		Environment: "poc",
		Status:      "failed",
	})
	if err != nil {
		t.Fatalf("WriteImmediate(second) error = %v", err)
	}
	server := NewServer(testManifest(), environment.ResolvedEnvironment{
		EnvironmentName: "poc",
		TracePath:       tracePath,
	}, nil, w2a.NewMemorySignalIdempotencyStore(), writer)

	listReq := httptest.NewRequest(http.MethodGet, "/api/traces?limit=1", nil)
	listRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("GET /api/traces status = %d, want %d; body = %s", listRec.Code, http.StatusOK, listRec.Body.String())
	}
	var traces []trace.TraceRecord
	if err := json.Unmarshal(listRec.Body.Bytes(), &traces); err != nil {
		t.Fatalf("decode traces payload: %v", err)
	}
	if got := len(traces); got != 1 {
		t.Fatalf("traces len = %d, want 1", got)
	}
	if traces[0].TraceID != second.TraceID {
		t.Fatalf("traces[0].TraceID = %q, want newest %q", traces[0].TraceID, second.TraceID)
	}

	loadReq := httptest.NewRequest(http.MethodGet, "/api/traces/"+first.TraceID, nil)
	loadRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(loadRec, loadReq)
	if loadRec.Code != http.StatusOK {
		t.Fatalf("GET /api/traces/{traceId} status = %d, want %d; body = %s", loadRec.Code, http.StatusOK, loadRec.Body.String())
	}
	var loaded trace.TraceRecord
	if err := json.Unmarshal(loadRec.Body.Bytes(), &loaded); err != nil {
		t.Fatalf("decode trace payload: %v", err)
	}
	if loaded.TraceID != first.TraceID {
		t.Fatalf("loaded TraceID = %q, want %q", loaded.TraceID, first.TraceID)
	}

	missingReq := httptest.NewRequest(http.MethodGet, "/api/traces/trace_missing", nil)
	missingRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(missingRec, missingReq)
	if missingRec.Code != http.StatusNotFound {
		t.Fatalf("GET missing trace status = %d, want %d", missingRec.Code, http.StatusNotFound)
	}
}

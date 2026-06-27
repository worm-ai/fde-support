package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"fde-support/internal/environment"
	"fde-support/internal/trace"
	"fde-support/internal/w2a"
)

func TestServerServesWebConsole(t *testing.T) {
	t.Parallel()

	tracePath := t.TempDir()
	server := NewServer(testManifest(), environment.ResolvedEnvironment{
		EnvironmentName: "poc",
		TracePath:       tracePath,
	}, nil, w2a.NewMemorySignalIdempotencyStore(), trace.NewFileTraceWriter(tracePath))

	req := httptest.NewRequest(http.MethodGet, "/web/", nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /web/ status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	contentType := rec.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/html") {
		t.Fatalf("GET /web/ Content-Type = %q, want text/html", contentType)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "<!doctype html>") || !strings.Contains(body, "FDE Support Console") {
		t.Fatalf("GET /web/ body does not look like console HTML: %s", body)
	}
}

package runtimecore

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func TestRuntimeLoggerRedactsSensitiveFields(t *testing.T) {
	stderr := captureStderr(t, func() {
		runtimeLogger{traceID: "trace-1"}.Info("", "call", map[string]any{
			"Authorization": "Bearer secret",
			"phone":         "13800138000",
			"email":         "user@example.com",
			"raw_payload":   map[string]any{"token": "secret"},
		})
	})
	if strings.Contains(stderr, "Bearer secret") || strings.Contains(stderr, "13800138000") || strings.Contains(stderr, "user@example.com") || strings.Contains(stderr, "raw_payload") {
		t.Fatalf("stderr not redacted: %s", stderr)
	}
	if !strings.Contains(stderr, "[trace-1]") {
		t.Fatalf("stderr missing trace id: %s", stderr)
	}
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stderr = w
	fn()
	_ = w.Close()
	os.Stderr = old
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	_ = r.Close()
	return buf.String()
}

package trace

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFileTraceWriterListReturnsNewestFirstAndHonorsLimit(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tracePath := t.TempDir()
	writer := NewFileTraceWriter(tracePath)

	first, err := writer.WriteImmediate(ctx, TraceRecord{
		Solution:    "support",
		Version:     "0.1.0",
		Environment: "poc",
		Status:      "success",
	})
	if err != nil {
		t.Fatalf("WriteImmediate(first) error = %v", err)
	}
	time.Sleep(2 * time.Millisecond)
	second, err := writer.WriteImmediate(ctx, TraceRecord{
		Solution:    "support",
		Version:     "0.1.0",
		Environment: "poc",
		Status:      "failed",
	})
	if err != nil {
		t.Fatalf("WriteImmediate(second) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(tracePath, first.TraceID+".json.tmp"), []byte("{"), 0o644); err != nil {
		t.Fatalf("write partial temp trace file: %v", err)
	}

	records, err := writer.List(ctx, 1)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if got := len(records); got != 1 {
		t.Fatalf("List() len = %d, want 1", got)
	}
	if records[0].TraceID != second.TraceID {
		t.Fatalf("List()[0].TraceID = %q, want newest %q", records[0].TraceID, second.TraceID)
	}

	records, err = writer.List(ctx, 0)
	if err != nil {
		t.Fatalf("List(no limit) error = %v", err)
	}
	if got := len(records); got != 2 {
		t.Fatalf("List(no limit) len = %d, want 2", got)
	}
	if records[0].TraceID != second.TraceID || records[1].TraceID != first.TraceID {
		t.Fatalf("List(no limit) order = [%q, %q], want [%q, %q]", records[0].TraceID, records[1].TraceID, second.TraceID, first.TraceID)
	}
}

func TestRedactValueMasksSensitiveKeysAndPII(t *testing.T) {
	t.Parallel()

	input := map[string]any{
		"message": "call me at 13800138000",
		"apiKey":  "sk-live-secret",
		"nested": map[string]any{
			"authToken": "token-123",
		},
	}
	got := RedactValue(input).(map[string]any)
	if got["apiKey"] == "sk-live-secret" {
		t.Fatalf("apiKey was not redacted: %#v", got)
	}
	if got["message"] == "call me at 13800138000" {
		t.Fatalf("message PII was not redacted: %#v", got)
	}
	nested := got["nested"].(map[string]any)
	if nested["authToken"] == "token-123" {
		t.Fatalf("nested token was not redacted: %#v", got)
	}
}

func TestRedactValueMasksInlineSensitivePhrases(t *testing.T) {
	t.Parallel()

	got, ok := RedactValue("my phone is 13800138000 and token is abc").(string)
	if !ok {
		t.Fatalf("RedactValue returned %T, want string", got)
	}
	for _, leaked := range []string{"13800138000", "abc"} {
		if strings.Contains(got, leaked) {
			t.Fatalf("redacted string leaked %q: %s", leaked, got)
		}
	}
}

func TestFileTraceWriterRedactsRecordAndSpanData(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	writer := NewFileTraceWriter(t.TempDir())

	started, err := writer.Start(ctx, TraceRecord{
		Solution:    "support",
		Version:     "0.1.0",
		Environment: "poc",
		Trigger:     TriggerSpec{Type: "chat"},
		Input: map[string]any{
			"message":     "phone 13800138000 email user@example.com",
			"raw_payload": map[string]any{"password": "clear-text-password"},
		},
	})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if err := writer.AppendSpan(ctx, started.TraceID, TraceSpan{
		Node:      "generate_answer",
		Component: "answer_generator",
		Input:     map[string]any{"apiKey": "sk-live-secret"},
		Output:    map[string]any{"answer": "email user@example.com"},
	}); err != nil {
		t.Fatalf("AppendSpan() error = %v", err)
	}
	finished, err := writer.Finish(ctx, started.TraceID, "success", nil, time.Millisecond)
	if err != nil {
		t.Fatalf("Finish() error = %v", err)
	}

	bytes, _ := json.Marshal(finished)
	text := string(bytes)
	for _, leaked := range []string{"13800138000", "user@example.com", "clear-text-password", "sk-live-secret", "raw_payload"} {
		if strings.Contains(text, leaked) {
			t.Fatalf("trace leaked %q: %s", leaked, text)
		}
	}
}

func TestFileTraceWriterLoadReturnsRecordAndMissingError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	writer := NewFileTraceWriter(t.TempDir())

	written, err := writer.WriteImmediate(ctx, TraceRecord{
		Solution:    "support",
		Version:     "0.1.0",
		Environment: "poc",
		Trigger:     TriggerSpec{Type: "chat"},
		Input:       map[string]any{"message": "hello"},
		Status:      "success",
	})
	if err != nil {
		t.Fatalf("WriteImmediate() error = %v", err)
	}

	loaded, err := writer.Load(ctx, written.TraceID)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded.TraceID != written.TraceID {
		t.Fatalf("Load().TraceID = %q, want %q", loaded.TraceID, written.TraceID)
	}
	if loaded.Trigger.Type != "chat" {
		t.Fatalf("Load().Trigger.Type = %q, want chat", loaded.Trigger.Type)
	}

	if _, err := writer.Load(ctx, "trace_missing"); err == nil {
		t.Fatalf("Load(missing) error = nil, want error")
	}
}

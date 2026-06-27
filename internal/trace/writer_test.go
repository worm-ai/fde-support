package trace

import (
	"context"
	"testing"
	"time"
)

func TestFileTraceWriterListReturnsNewestFirstAndHonorsLimit(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	writer := NewFileTraceWriter(t.TempDir())

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

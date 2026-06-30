package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"fde-support/internal/environment"
	"fde-support/internal/manifest"
	"fde-support/internal/runtimecore"
	"fde-support/internal/trace"
	"fde-support/internal/w2a"
)

func TestHandleSignalDuplicateWritesAuditTrace(t *testing.T) {
	t.Parallel()

	router := NewSignalRouter(
		&manifest.SolutionManifest{
			Metadata: manifest.MetadataSpec{Name: "lecharm-support-agent", Version: "0.1.0"},
		},
		environment.ResolvedEnvironment{EnvironmentName: "poc"},
		&stubExecutor{response: map[string]any{
			"traceId":   "trace-original",
			"answer":    "ok",
			"citations": []any{"manual-001#section-4.2"},
		}},
		w2a.NewMemorySignalIdempotencyStore(),
		&stubTraceWriter{},
	)

	sensor := manifest.SensorSpec{
		ID:          "ticket_webhook",
		SignalTypes: []string{"ticket.created"},
	}
	payload := map[string]any{
		"signal_id":      "sig-10086",
		"schema_version": w2a.SchemaVersion01,
		"emitted_at":     1719400000000,
		"source": map[string]any{
			"sensor_id":      "ticket_webhook",
			"sensor_version": "0.1.0",
			"source_type":    "ticket-system",
			"user_identity":  "customer-10086",
			"package":        "@world2agent/sensor-webhook",
		},
		"event": map[string]any{
			"type":        "ticket.created",
			"occurred_at": 1719400000000,
			"summary":     "Customer reported pump E42 error",
		},
		"source_event": map[string]any{
			"data": map[string]any{
				"ticketId":     "T-10086",
				"productModel": "PX-9",
				"description":  "The pump shows E42 after restart. What should I do?",
			},
		},
	}

	resp1, status1, err := router.HandleSignal(context.Background(), sensor, payload, "")
	if err != nil {
		t.Fatalf("first HandleSignal() error = %v", err)
	}
	if status1 != http.StatusOK {
		t.Fatalf("first HandleSignal() status = %d, want %d", status1, http.StatusOK)
	}
	if resp1["traceId"] != "trace-original" {
		t.Fatalf("first HandleSignal() traceId = %#v, want trace-original", resp1["traceId"])
	}

	resp2, status2, err := router.HandleSignal(context.Background(), sensor, payload, "")
	if err != nil {
		t.Fatalf("second HandleSignal() error = %v", err)
	}
	if status2 != http.StatusOK {
		t.Fatalf("second HandleSignal() status = %d, want %d", status2, http.StatusOK)
	}
	if resp2["duplicate"] != true {
		t.Fatalf("second HandleSignal() duplicate = %#v, want true", resp2["duplicate"])
	}
	if resp2["traceId"] != "trace-original" {
		t.Fatalf("second HandleSignal() traceId = %#v, want trace-original", resp2["traceId"])
	}

	exec := router.executor.(*stubExecutor)
	if exec.calls != 1 {
		t.Fatalf("executor calls = %d, want 1", exec.calls)
	}

	writer := router.traceWriter.(*stubTraceWriter)
	if got := len(writer.records); got != 1 {
		t.Fatalf("duplicate trace writes = %d, want 1", got)
	}
	if !writer.records[0].Duplicate {
		t.Fatalf("duplicate trace record not marked duplicate")
	}
	if writer.records[0].Trigger.Type != "w2a_signal" || writer.records[0].Trigger.Sensor != "ticket_webhook" {
		t.Fatalf("duplicate trace trigger = %#v, want w2a_signal/ticket_webhook", writer.records[0].Trigger)
	}
}

func TestRejectedSignalTraceDoesNotStoreRawPayload(t *testing.T) {
	t.Parallel()

	writer := &stubTraceWriter{}
	router := NewSignalRouter(
		&manifest.SolutionManifest{
			Metadata: manifest.MetadataSpec{Name: "lecharm-support-agent", Version: "0.1.0"},
		},
		environment.ResolvedEnvironment{EnvironmentName: "poc"},
		&stubExecutor{response: map[string]any{"traceId": "trace-original"}},
		w2a.NewMemorySignalIdempotencyStore(),
		writer,
	)

	sensor := manifest.SensorSpec{
		ID:          "ticket_webhook",
		SignalTypes: []string{"ticket.created"},
	}
	payload := map[string]any{
		"signal_id":      "sig-secret",
		"schema_version": w2a.SchemaVersion01,
		"source_event": map[string]any{
			"data": map[string]any{
				"password": "clear-text-password",
			},
		},
	}

	_, _, appErr := router.HandleSignal(context.Background(), sensor, payload, "")
	if appErr == nil {
		t.Fatalf("expected malformed signal to be rejected")
	}
	if len(writer.records) != 1 {
		t.Fatalf("trace writes = %d, want 1", len(writer.records))
	}
	record := writer.records[0]
	if _, ok := record.Input["raw_payload"]; ok {
		t.Fatalf("rejected trace stored raw_payload: %#v", record.Input)
	}
	bytes, _ := json.Marshal(record)
	if strings.Contains(string(bytes), "clear-text-password") {
		t.Fatalf("rejected trace leaked password: %s", string(bytes))
	}
}

type stubExecutor struct {
	mu       sync.Mutex
	calls    int
	response map[string]any
}

func (e *stubExecutor) Execute(ctx context.Context, req runtimecore.RuntimeRequest) (map[string]any, *trace.TraceRecord, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.calls++
	return copyMap(e.response), nil, nil
}

type stubTraceWriter struct {
	mu      sync.Mutex
	records []trace.TraceRecord
}

func (w *stubTraceWriter) Start(ctx context.Context, meta trace.TraceRecord) (*trace.TraceRecord, error) {
	panic("unexpected Start call in stubTraceWriter")
}

func (w *stubTraceWriter) AppendSpan(ctx context.Context, traceID string, span trace.TraceSpan) error {
	panic("unexpected AppendSpan call in stubTraceWriter")
}

func (w *stubTraceWriter) Finish(ctx context.Context, traceID string, status string, errSummary *trace.RuntimeErrorSummary, latency time.Duration) (*trace.TraceRecord, error) {
	panic("unexpected Finish call in stubTraceWriter")
}

func (w *stubTraceWriter) WriteImmediate(ctx context.Context, record trace.TraceRecord) (*trace.TraceRecord, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if record.Status == "" {
		record.Status = "success"
	}
	if record.TraceID == "" {
		record.TraceID = "trace-audit"
	}
	w.records = append(w.records, record)
	return &record, nil
}

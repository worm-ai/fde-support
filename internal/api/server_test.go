package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"fde-support/internal/environment"
	"fde-support/internal/manifest"
	"fde-support/internal/shared"
	"fde-support/internal/trace"
	"fde-support/internal/w2a"
)

func TestServerRuntimeAPI(t *testing.T) {
	t.Parallel()

	tracePath := t.TempDir()
	server := NewServer(testManifest(), environment.ResolvedEnvironment{
		EnvironmentName: "poc",
		TracePath:       tracePath,
	}, nil, w2a.NewMemorySignalIdempotencyStore(), trace.NewFileTraceWriter(tracePath))

	req := httptest.NewRequest(http.MethodGet, "/api/runtime", nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/runtime status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var payload runtimeView
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode runtime payload: %v", err)
	}
	if payload.Solution != "lecharm-support-agent" {
		t.Fatalf("solution = %q, want lecharm-support-agent", payload.Solution)
	}
	if payload.Version != "0.1.0" {
		t.Fatalf("version = %q, want 0.1.0", payload.Version)
	}
	if payload.Environment != "poc" {
		t.Fatalf("environment = %q, want poc", payload.Environment)
	}
	if payload.TracePath != tracePath {
		t.Fatalf("tracePath = %q, want %q", payload.TracePath, tracePath)
	}
	if payload.ChatPath != "/chat" {
		t.Fatalf("chatPath = %q, want /chat", payload.ChatPath)
	}
	if payload.WebPath != "/web/" {
		t.Fatalf("webPath = %q, want /web/", payload.WebPath)
	}
	if got := len(payload.Sensors); got != 1 {
		t.Fatalf("sensors len = %d, want 1", got)
	}
	sensor := payload.Sensors[0]
	if sensor.ID != "ticket_webhook" {
		t.Fatalf("sensor ID = %q, want ticket_webhook", sensor.ID)
	}
	if sensor.EndpointPath != "/w2a/tickets" {
		t.Fatalf("sensor endpointPath = %q, want /w2a/tickets", sensor.EndpointPath)
	}
	if got := len(sensor.SignalTypes); got != 1 || sensor.SignalTypes[0] != "ticket.created" {
		t.Fatalf("sensor signalTypes = %#v, want [ticket.created]", sensor.SignalTypes)
	}
}

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

func TestServerW2ASensorPreservesCachedIdempotencyStatus(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := w2a.NewMemorySignalIdempotencyStore()
	signalID := "sig-cached-400"
	key := w2a.SignalIdempotencyKey{Environment: "poc", SensorID: "ticket_webhook", SignalID: signalID}
	if err := store.Put(ctx, key, w2a.IdempotencyRecord{
		Response:   map[string]any{"error": shared.BadRequest("SIGNAL_TYPE_NOT_ALLOWED", "event.type", "signal type is not authorized for this sensor")},
		HTTPStatus: http.StatusBadRequest,
	}, time.Hour); err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	tracePath := t.TempDir()
	server := NewServer(testManifest(), environment.ResolvedEnvironment{
		EnvironmentName: "poc",
		TracePath:       tracePath,
	}, nil, store, trace.NewFileTraceWriter(tracePath))

	req := httptest.NewRequest(http.MethodPost, "/w2a/tickets", strings.NewReader(`{
		"signal_id": "sig-cached-400",
		"schema_version": "w2a/0.1",
		"emitted_at": 1719400000000,
		"source": {
			"sensor_id": "ticket_webhook",
			"sensor_version": "0.1.0",
			"source_type": "ticket-system",
			"user_identity": "customer-10086",
			"package": "@world2agent/sensor-webhook"
		},
		"event": {
			"type": "ticket.created",
			"occurred_at": 1719400000000,
			"summary": "Customer reported pump E42 error"
		}
	}`))
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("POST /w2a/tickets status = %d, want %d; body = %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response payload: %v", err)
	}
	if payload["duplicate"] != true {
		t.Fatalf("duplicate = %#v, want true", payload["duplicate"])
	}
}

func testManifest() *manifest.SolutionManifest {
	return &manifest.SolutionManifest{
		Metadata: manifest.MetadataSpec{
			Name:    "lecharm-support-agent",
			Version: "0.1.0",
		},
		Perception: manifest.PerceptionSpec{
			Sensors: []manifest.SensorSpec{
				{
					ID:          "ticket_webhook",
					SignalTypes: []string{"ticket.created"},
					Config: map[string]any{
						"endpointPath": "/w2a/tickets",
					},
				},
			},
		},
	}
}

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
	"fde-support/internal/model"
	"fde-support/internal/registry"
	"fde-support/internal/runtimecore"
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

func TestServerChatTraceRedactsSensitiveInput(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tracePath := t.TempDir()
	writer := trace.NewFileTraceWriter(tracePath)
	env := environment.ResolvedEnvironment{
		EnvironmentName: "poc",
		TracePath:       tracePath,
	}
	m := chatTraceManifest()
	executor, err := runtimecore.NewExecutor(m, env, registry.NewBuiltinComponentRegistry(), nil, writer, model.NewMockGateway(), nil)
	if err != nil {
		t.Fatalf("NewExecutor() error = %v", err)
	}
	server := NewServer(m, env, executor, w2a.NewMemorySignalIdempotencyStore(), writer)

	req := httptest.NewRequest(http.MethodPost, "/chat", strings.NewReader(`{"message":"my phone is 13800138000 and token is abc","sessionId":"s1"}`))
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /chat status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode chat response: %v", err)
	}
	traceID, ok := payload["traceId"].(string)
	if !ok || traceID == "" {
		t.Fatalf("traceId = %#v, want non-empty string", payload["traceId"])
	}
	record, err := writer.Load(ctx, traceID)
	if err != nil {
		t.Fatalf("Load(%q) error = %v", traceID, err)
	}
	bytes, err := json.Marshal(record)
	if err != nil {
		t.Fatalf("marshal trace: %v", err)
	}
	text := string(bytes)
	if strings.Contains(text, "13800138000") || strings.Contains(text, "abc") {
		t.Fatalf("trace leaked sensitive data: %s", text)
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

func chatTraceManifest() *manifest.SolutionManifest {
	return &manifest.SolutionManifest{
		Metadata: manifest.MetadataSpec{
			Name:    "lecharm-support-agent",
			Version: "0.1.0",
		},
		Workflow: manifest.WorkflowSpec{
			InputMapping: map[string]map[string]string{
				"chat": {
					"message":   "request.message",
					"sessionId": "request.sessionId",
				},
			},
		},
		Runtime: manifest.RuntimeSpec{
			Observability: manifest.ObservabilitySpec{Trace: "required"},
		},
	}
}

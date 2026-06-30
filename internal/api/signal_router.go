package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"fde-support/internal/environment"
	"fde-support/internal/manifest"
	"fde-support/internal/runtimecore"
	"fde-support/internal/shared"
	"fde-support/internal/trace"
	"fde-support/internal/w2a"
)

type SignalRouter struct {
	manifest    *manifest.SolutionManifest
	env         environment.ResolvedEnvironment
	executor    runtimecore.ExecutorLike
	idempotency w2a.SignalIdempotencyStore
	traceWriter trace.TraceWriter
	adapter     w2a.WebhookAdapter
	auditLog    io.Writer
}

func NewSignalRouter(m *manifest.SolutionManifest, env environment.ResolvedEnvironment, executor runtimecore.ExecutorLike, store w2a.SignalIdempotencyStore, traceWriter trace.TraceWriter, auditLog io.Writer) *SignalRouter {
	return &SignalRouter{
		manifest:    m,
		env:         env,
		executor:    executor,
		idempotency: store,
		traceWriter: traceWriter,
		adapter:     w2a.NewWebhookAdapter(),
		auditLog:    auditLog,
	}
}

func (r *SignalRouter) HandleChat(ctx context.Context, payload map[string]any) (map[string]any, *shared.AppError) {
	message, ok := payload["message"].(string)
	if !ok || strings.TrimSpace(message) == "" {
		return nil, shared.BadRequest("CHAT_MESSAGE_REQUIRED", "message", "message is required")
	}
	req := runtimecore.RuntimeRequest{
		Trigger:    runtimecore.TriggerSpec{Type: "chat"},
		Request:    map[string]any{"message": message},
		RawPayload: payload,
	}
	if sessionID, ok := payload["sessionId"].(string); ok {
		req.Request["sessionId"] = sessionID
	}
	response, _, err := r.executor.Execute(ctx, req)
	if err != nil {
		return nil, shared.AsAppError(err)
	}
	return response, nil
}

func (r *SignalRouter) HandleSignal(ctx context.Context, sensor manifest.SensorSpec, payload map[string]any, authorization string) (map[string]any, int, *shared.AppError) {
	expectedToken, _ := sensor.Config["authTokenRef"].(string)
	if expectedToken != "" {
		tokenValue, ok := r.env.ResolveSecret(expectedToken)
		if !ok || !bearerMatches(authorization, tokenValue) {
			appErr := shared.Unauthorized("UNAUTHORIZED_SIGNAL", "Authorization", "invalid bearer token")
			if r.auditLog != nil {
				fmt.Fprintf(r.auditLog, "%s [SECURITY] auth_failure sensor=%s\n", time.Now().UTC().Format(time.RFC3339), sensor.ID)

			}
			return nil, appErr.HTTPStatus, appErr
		}
	}

	signal, appErr := r.adapter.Normalize(payload)
	if appErr != nil {
		_ = r.writeRejectedTrace(ctx, sensor, payload, appErr)
		return nil, appErr.HTTPStatus, appErr
	}
	if signal.SensorID != sensor.ID {
		appErr := shared.BadRequest("W2A_SENSOR_MISMATCH", "source.sensor_id", "source.sensor_id must match endpoint sensor")
		_ = r.writeRejectedTrace(ctx, sensor, payload, appErr)
		_ = r.putRejection(ctx, signal, sensor, appErr)
		return nil, appErr.HTTPStatus, appErr
	}
	if !contains(sensor.SignalTypes, signal.EventType) {
		appErr := shared.BadRequest("SIGNAL_TYPE_NOT_ALLOWED", "event.type", "signal type is not authorized for this sensor")
		_ = r.writeRejectedTrace(ctx, sensor, payload, appErr)
		_ = r.putRejection(ctx, signal, sensor, appErr)
		return nil, appErr.HTTPStatus, appErr
	}

	key := w2a.SignalIdempotencyKey{Environment: r.env.EnvironmentName, SensorID: signal.SensorID, SignalID: signal.SignalID}
	if record, ok, err := r.idempotency.Get(ctx, key); err == nil && ok {
		response := copyMap(record.Response)
		response["duplicate"] = true
		_, _ = r.traceWriter.WriteImmediate(ctx, trace.TraceRecord{
			Solution:    r.manifest.Metadata.Name,
			Version:     r.manifest.Metadata.Version,
			Environment: r.env.EnvironmentName,
			Trigger:     trace.TriggerSpec{Type: "w2a_signal", Sensor: sensor.ID, SignalType: signal.EventType},
			Input:       map[string]any{"sensor": sensor.ID, "signal_id": signal.SignalID},
			Status:      "success",
			Duplicate:   true,
		})
		return response, record.HTTPStatus, nil
	}

	req := runtimecore.RuntimeRequest{
		Trigger: runtimecore.TriggerSpec{
			Type:       "w2a_signal",
			Sensor:     sensor.ID,
			SignalType: signal.EventType,
		},
		Signal:     payload,
		RawPayload: payload,
	}
	response, _, err := r.executor.Execute(ctx, req)
	if err != nil {
		appErr := shared.AsAppError(err)
		if appErr.HTTPStatus >= 400 && appErr.HTTPStatus < 500 {
			_ = r.idempotency.Put(ctx, key, w2a.IdempotencyRecord{Response: map[string]any{"error": appErr}, HTTPStatus: appErr.HTTPStatus}, 24*time.Hour)
		}
		return nil, appErr.HTTPStatus, appErr
	}
	_ = r.idempotency.Put(ctx, key, w2a.IdempotencyRecord{Response: response, HTTPStatus: http.StatusOK}, 24*time.Hour)
	return response, http.StatusOK, nil
}

func (r *SignalRouter) writeRejectedTrace(ctx context.Context, sensor manifest.SensorSpec, payload map[string]any, appErr *shared.AppError) error {
	input := map[string]any{"sensor": sensor.ID}
	if payload != nil {
		if signalID, ok := payload["signal_id"].(string); ok {
			input["signal_id"] = signalID
		}
		if schemaVersion, ok := payload["schema_version"].(string); ok {
			input["schema_version"] = schemaVersion
		}
		if event, ok := payload["event"].(map[string]any); ok {
			if eventType, ok := event["type"].(string); ok {
				input["event_type"] = eventType
			}
		}
	}
	_, err := r.traceWriter.WriteImmediate(ctx, trace.TraceRecord{
		Solution:    r.manifest.Metadata.Name,
		Version:     r.manifest.Metadata.Version,
		Environment: r.env.EnvironmentName,
		Trigger:     trace.TriggerSpec{Type: "w2a_signal", Sensor: sensor.ID},
		Input:       input,
		Status:      "rejected",
		Error:       &trace.RuntimeErrorSummary{Type: appErr.Code, Message: appErr.Message},
	})
	return err
}

func (r *SignalRouter) putRejection(ctx context.Context, signal w2a.SignalEnvelope, sensor manifest.SensorSpec, appErr *shared.AppError) error {
	key := w2a.SignalIdempotencyKey{Environment: r.env.EnvironmentName, SensorID: sensor.ID, SignalID: signal.SignalID}
	return r.idempotency.Put(ctx, key, w2a.IdempotencyRecord{Response: map[string]any{"error": appErr}, HTTPStatus: appErr.HTTPStatus}, 24*time.Hour)
}

func bearerMatches(header, expected string) bool {
	const prefix = "Bearer "
	return strings.HasPrefix(header, prefix) && strings.TrimSpace(strings.TrimPrefix(header, prefix)) == expected
}

func contains(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

func copyMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

package w2a

import (
	"fmt"

	"fde-support/internal/shared"
)

const SchemaVersion01 = "w2a/0.1"

type SignalEnvelope struct {
	SignalID      string
	SchemaVersion string
	SensorID      string
	EventType     string
	EventSummary  string
	Raw           map[string]any
}

type WebhookAdapter struct{}

func NewWebhookAdapter() WebhookAdapter {
	return WebhookAdapter{}
}

func (WebhookAdapter) Normalize(raw map[string]any) (SignalEnvelope, *shared.AppError) {
	if raw == nil {
		return SignalEnvelope{}, shared.BadRequest("W2A_FIELD_MISSING", "body", "request body is required")
	}
	cloned := cloneMap(raw)
	if sourceEvent, ok := cloned["source_event"].(map[string]any); ok {
		delete(sourceEvent, "schema")
		cloned["source_event"] = sourceEvent
	}
	return ValidateSignal(cloned)
}

func ValidateSignal(raw map[string]any) (SignalEnvelope, *shared.AppError) {
	schemaVersion, ok := raw["schema_version"].(string)
	if !ok || schemaVersion == "" {
		return SignalEnvelope{}, shared.BadRequest("W2A_FIELD_MISSING", "schema_version", "schema_version is required")
	}
	if schemaVersion != SchemaVersion01 {
		return SignalEnvelope{}, shared.BadRequest("W2A_SCHEMA_VERSION_UNSUPPORTED", "schema_version", "unknown W2A schema_version")
	}
	signalID, ok := raw["signal_id"].(string)
	if !ok || signalID == "" {
		return SignalEnvelope{}, shared.BadRequest("W2A_FIELD_MISSING", "signal_id", "signal_id is required")
	}
	if _, ok := raw["emitted_at"]; !ok {
		return SignalEnvelope{}, shared.BadRequest("W2A_FIELD_MISSING", "emitted_at", "emitted_at is required")
	}
	source, ok := raw["source"].(map[string]any)
	if !ok {
		return SignalEnvelope{}, shared.BadRequest("W2A_FIELD_MISSING", "source", "source is required")
	}
	sensorID, ok := source["sensor_id"].(string)
	if !ok || sensorID == "" {
		return SignalEnvelope{}, shared.BadRequest("W2A_FIELD_MISSING", "source.sensor_id", "source.sensor_id is required")
	}
	for _, field := range []string{"sensor_version", "package", "source_type", "user_identity"} {
		if value, ok := source[field].(string); !ok || value == "" {
			return SignalEnvelope{}, shared.BadRequest("W2A_FIELD_MISSING", "source."+field, fmt.Sprintf("source.%s is required", field))
		}
	}
	event, ok := raw["event"].(map[string]any)
	if !ok {
		return SignalEnvelope{}, shared.BadRequest("W2A_FIELD_MISSING", "event", "event is required")
	}
	eventType, ok := event["type"].(string)
	if !ok || eventType == "" {
		return SignalEnvelope{}, shared.BadRequest("W2A_FIELD_MISSING", "event.type", "event.type is required")
	}
	if _, ok := event["occurred_at"]; !ok {
		return SignalEnvelope{}, shared.BadRequest("W2A_FIELD_MISSING", "event.occurred_at", "event.occurred_at is required")
	}
	summary, _ := event["summary"].(string)
	return SignalEnvelope{
		SignalID:      signalID,
		SchemaVersion: schemaVersion,
		SensorID:      sensorID,
		EventType:     eventType,
		EventSummary:  summary,
		Raw:           raw,
	}, nil
}

func cloneMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for key, value := range in {
		switch v := value.(type) {
		case map[string]any:
			out[key] = cloneMap(v)
		default:
			out[key] = value
		}
	}
	return out
}

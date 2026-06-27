package w2a

import "testing"

func TestWebhookAdapterNormalizesSignalAndDropsInlineSchema(t *testing.T) {
	t.Parallel()

	adapter := NewWebhookAdapter()
	payload := map[string]any{
		"signal_id":      "sig-10086",
		"schema_version": SchemaVersion01,
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
			"schema": map[string]any{
				"type": "object",
			},
			"data": map[string]any{
				"ticketId": "T-10086",
				"description": "The pump shows E42 after restart.",
			},
		},
	}

	envelope, err := adapter.Normalize(payload)
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}
	if envelope.SignalID != "sig-10086" {
		t.Fatalf("SignalID = %q, want sig-10086", envelope.SignalID)
	}
	if envelope.SensorID != "ticket_webhook" {
		t.Fatalf("SensorID = %q, want ticket_webhook", envelope.SensorID)
	}
	if envelope.EventType != "ticket.created" {
		t.Fatalf("EventType = %q, want ticket.created", envelope.EventType)
	}
	sourceEvent, ok := envelope.Raw["source_event"].(map[string]any)
	if !ok {
		t.Fatalf("Raw.source_event type = %T, want map[string]any", envelope.Raw["source_event"])
	}
	if _, ok := sourceEvent["schema"]; ok {
		t.Fatalf("source_event.schema should be dropped from normalized envelope")
	}
}

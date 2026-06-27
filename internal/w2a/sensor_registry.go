package w2a

import (
	"fmt"
	"strings"
)

const BuiltinWebhookSensorRef = "@world2agent/sensor-webhook@1.0.0"

type SensorDescriptor struct {
	Ref     string
	Kind    string
	BuiltIn bool
	Version string
}

type SensorRegistry interface {
	Resolve(ref string) (SensorDescriptor, error)
}

type BuiltinSensorRegistry struct {
	sensors map[string]SensorDescriptor
}

func NewBuiltinSensorRegistry() *BuiltinSensorRegistry {
	return &BuiltinSensorRegistry{
		sensors: map[string]SensorDescriptor{
			BuiltinWebhookSensorRef: {
				Ref:     BuiltinWebhookSensorRef,
				Kind:    "webhook",
				BuiltIn: true,
				Version: "1.0.0",
			},
		},
	}
}

func (r *BuiltinSensorRegistry) Resolve(ref string) (SensorDescriptor, error) {
	desc, ok := r.sensors[ref]
	if !ok {
		return SensorDescriptor{}, fmt.Errorf("unknown sensor ref %q", ref)
	}
	if !strings.Contains(ref, "@1.0.0") {
		return SensorDescriptor{}, fmt.Errorf("sensor ref %q must pin version", ref)
	}
	return desc, nil
}

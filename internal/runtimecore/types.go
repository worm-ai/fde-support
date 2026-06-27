package runtimecore

import (
	"fde-support/internal/registry"
)

type RuntimeRequest struct {
	Trigger    TriggerSpec    `json:"trigger"`
	Request    map[string]any `json:"request,omitempty"`
	Signal     map[string]any `json:"signal,omitempty"`
	RawPayload any            `json:"raw_payload,omitempty"`
}

type TriggerSpec struct {
	Type       string `json:"type"`
	Sensor     string `json:"sensor,omitempty"`
	SignalType string `json:"signalType,omitempty"`
}

type ExecutionResult struct {
	Response map[string]any `json:"response"`
	TraceID  string         `json:"traceId"`
}

type runtimeContext struct {
	environment string
	knowledge   registry.KnowledgeReader
	request     RuntimeRequest
	errSummary  *registry.RuntimeErrorSummary
	actions     []registry.ActionSummary
}

func (c runtimeContext) Environment() string {
	return c.environment
}

func (c runtimeContext) Knowledge() registry.KnowledgeReader {
	return c.knowledge
}

func (c runtimeContext) Request() registry.RuntimeRequestMetadata {
	return registry.RuntimeRequestMetadata{
		TriggerType: c.request.Trigger.Type,
		Sensor:      c.request.Trigger.Sensor,
		SignalType:  c.request.Trigger.SignalType,
	}
}

func (c runtimeContext) Error() *registry.RuntimeErrorSummary {
	return c.errSummary
}

func (c runtimeContext) Actions() []registry.ActionSummary {
	return append([]registry.ActionSummary(nil), c.actions...)
}

package runtimecore

import (
	"fde-support/internal/registry"
	"fde-support/internal/trace"
	"fmt"
	"os"
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
	environment  string
	knowledge    registry.KnowledgeReader
	request      RuntimeRequest
	errSummary   *registry.RuntimeErrorSummary
	modelGateway registry.ModelGateway
	httpGateway  registry.HTTPCaller
	actions      []registry.ActionSummary
	logger       runtimeLogger
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

func (c runtimeContext) Model() registry.ModelGateway {
	return c.modelGateway
}

func (c runtimeContext) HTTP() registry.HTTPCaller {
	return c.httpGateway
}

func (c runtimeContext) Logger() registry.Logger {
	return c.logger
}

type runtimeLogger struct {
	traceID string
}

func (l runtimeLogger) Info(traceID string, msg string, fields map[string]any) {
	fmt.Fprintf(os.Stderr, "[%s] INFO: %s %v\n", l.effectiveTraceID(traceID), msg, trace.RedactMap(fields))
}

func (l runtimeLogger) Error(traceID string, msg string, fields map[string]any) {
	fmt.Fprintf(os.Stderr, "[%s] ERROR: %s %v\n", l.effectiveTraceID(traceID), msg, trace.RedactMap(fields))
}

func (l runtimeLogger) Debug(traceID string, msg string, fields map[string]any) {
	fmt.Fprintf(os.Stderr, "[%s] DEBUG: %s %v\n", l.effectiveTraceID(traceID), msg, trace.RedactMap(fields))
}

func (l runtimeLogger) effectiveTraceID(traceID string) string {
	if l.traceID != "" {
		return l.traceID
	}
	return traceID
}

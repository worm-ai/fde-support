package manifest

import (
	"path/filepath"
	"testing"

	"fde-support/internal/registry"
	"fde-support/internal/w2a"
)

func TestM1ExamplesValidate(t *testing.T) {
	paths := []string{
		filepath.Join("..", "..", "examples", "after-sales-support", "manifest.yaml"),
		filepath.Join("..", "..", "examples", "guoran-support", "manifest.yaml"),
		filepath.Join("..", "..", "templates", "customer-support.yaml"),
	}
	validateManifestFiles(t, paths)
}

func TestM2TemplatesValidate(t *testing.T) {
	t.Skip("enable after M2 component descriptors and source type validation are implemented")
}

func TestManifestValidator(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		mutate   func(*SolutionManifest)
		wantCode string
	}{
		{
			name: "valid manifest",
		},
		{
			name: "unknown component reference",
			mutate: func(m *SolutionManifest) {
				m.Workflow.Nodes[1].Component = "missing_component"
			},
			wantCode: "UNKNOWN_COMPONENT",
		},
		{
			name: "plaintext sensor secret is rejected",
			mutate: func(m *SolutionManifest) {
				m.Perception.Sensors[0].Config["authTokenRef"] = "plaintext-token"
			},
			wantCode: "INVALID_SECRET_REF",
		},
		{
			name: "trigger signal type must be allowed by sensor",
			mutate: func(m *SolutionManifest) {
				m.Perception.Triggers[0].SignalType = "ticket.closed"
			},
			wantCode: "TRIGGER_SIGNAL_TYPE_NOT_ALLOWED",
		},
		{
			name: "when implicit variable is rejected",
			mutate: func(m *SolutionManifest) {
				m.Workflow.Nodes[3].When = `confidence < 0.65`
			},
			wantCode: "INVALID_WHEN",
		},
		{
			name: "input cannot reference a skipped upstream node",
			mutate: func(m *SolutionManifest) {
				m.Workflow.Nodes = append(m.Workflow.Nodes, WorkflowNodeSpec{
					ID:        "post_handoff_answer",
					Component: "answer_generator",
					Inputs: map[string]string{
						"message":   "inputs.message",
						"passages":  "retrieve_knowledge.passages",
						"citations": "handoff.citations",
					},
				})
			},
			wantCode: "WORKFLOW_UNSAFE_DEPENDENCY",
		},
		{
			name: "fallback node cannot reference upstream output",
			mutate: func(m *SolutionManifest) {
				m.Workflow.Nodes[3].Inputs["message"] = "classify_intent.intent"
			},
			wantCode: "FALLBACK_INPUT_UNSAFE",
		},
		{
			name: "knowledge binding source must exist",
			mutate: func(m *SolutionManifest) {
				m.Runtime.KnowledgeBindings[0].Sources = []string{"missing_source"}
			},
			wantCode: "UNKNOWN_KNOWLEDGE_SOURCE",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			m := minimalManifest()
			if tt.mutate != nil {
				tt.mutate(&m)
			}

			errs := NewValidator(registry.NewBuiltinComponentRegistry(), w2a.NewBuiltinSensorRegistry()).Validate(&m)
			if tt.wantCode == "" {
				if len(errs) > 0 {
					t.Fatalf("Validate() returned errors: %#v", errs)
				}
				return
			}
			if !hasCode(errs, tt.wantCode) {
				t.Fatalf("Validate() missing code %q in %#v", tt.wantCode, errs)
			}
		})
	}
}

func validateManifestFiles(t *testing.T, paths []string) {
	t.Helper()

	for _, path := range paths {
		path := path
		t.Run(path, func(t *testing.T) {
			m, err := LoadFile(path)
			if err != nil {
				t.Fatalf("load manifest: %v", err)
			}
			errs := NewValidator(registry.NewBuiltinComponentRegistry(), w2a.NewBuiltinSensorRegistry()).Validate(m)
			if len(errs) > 0 {
				t.Fatalf("expected manifest to validate, got %#v", errs)
			}
		})
	}
}

func minimalManifest() SolutionManifest {
	return SolutionManifest{
		APIVersion: "solution.ai/v1alpha1",
		Kind:       "Solution",
		Metadata: MetadataSpec{
			Name:    "lecharm-support-agent",
			Version: "0.1.0",
			Owner:   "fde-zhouyuan",
		},
		Perception: PerceptionSpec{
			Sensors: []SensorSpec{
				{
					ID:          "ticket_webhook",
					Ref:         "@world2agent/sensor-webhook@1.0.0",
					SignalTypes: []string{"ticket.created"},
					Config: map[string]any{
						"endpointPath": "/w2a/tickets",
						"authTokenRef": "env:TICKET_WEBHOOK_TOKEN",
					},
				},
			},
			Triggers: []TriggerRouteSpec{
				{
					ID:         "ticket_triage",
					Sensor:     "ticket_webhook",
					SignalType: "ticket.created",
					RouteTo:    "classify_intent",
				},
			},
		},
		Knowledge: KnowledgeSpec{
			Sources: []KnowledgeSourceSpec{
				{ID: "product_manuals", Type: "jsonl", URI: "./data/lecharm/knowledge_units.jsonl", Schema: "faq"},
			},
			Schemas: []KnowledgeSchemaSpec{
				{ID: "faq", Fields: []string{"question", "answer", "source_ref"}},
			},
		},
		Components: []ComponentSpec{
			{ID: "intent_classifier", Category: "processor", Ref: "registry.intent.beverage-router@1.0.0", Config: map[string]any{"intents": []any{"troubleshooting", "complaint", "human_handoff"}}},
			{ID: "retriever", Category: "processor", Ref: "registry.retriever.local-keyword@1.0.0", Config: map[string]any{"topK": 5, "requireCitation": true}},
			{ID: "answer_generator", Category: "processor", Ref: "registry.agent.cited-answer@1.0.0", Config: map[string]any{"style": "concise", "requireGrounding": true}},
			{ID: "human_handoff", Category: "action", Ref: "registry.action.human-handoff@1.0.0", Config: map[string]any{"queue": "support-l2"}},
		},
		Workflow: WorkflowSpec{
			Entrypoint: "classify_intent",
			OnError: OnErrorSpec{
				Retry:        1,
				FallbackNode: "handoff",
			},
			InputMapping: map[string]map[string]string{
				"chat": {
					"message": "request.message",
				},
				"w2a_signal": {
					"message":  "signal.source_event.data.description",
					"ticketId": "signal.source_event.data.ticketId",
				},
			},
			Nodes: []WorkflowNodeSpec{
				{ID: "classify_intent", Component: "intent_classifier", Inputs: map[string]string{"message": "inputs.message"}},
				{ID: "retrieve_knowledge", Component: "retriever", Inputs: map[string]string{"query": "inputs.message"}},
				{ID: "generate_answer", Component: "answer_generator", Inputs: map[string]string{"message": "inputs.message", "passages": "retrieve_knowledge.passages", "citations": "retrieve_knowledge.citations"}},
				{ID: "handoff", Component: "human_handoff", When: "classify_intent.confidence < 0.65", Inputs: map[string]string{"message": "inputs.message"}},
			},
		},
		Runtime: RuntimeSpec{
			KnowledgeBindings: []KnowledgeBindingSpec{
				{Component: "retriever", Sources: []string{"product_manuals"}},
			},
			ModelPolicy: ModelPolicySpec{
				DefaultModel: "gpt-4.1",
				MaxLatencyMs: 8000,
			},
			Observability: ObservabilitySpec{
				Trace:     "required",
				LogInputs: "masked",
			},
		},
		Delivery: DeliverySpec{
			Environments: []EnvironmentSpec{
				{Name: "poc", Type: "shared_sandbox", Config: map[string]any{"modelKeyRef": "env:OPENAI_API_KEY", "tracePath": "./data/poc/traces", "retainDays": 7}},
			},
		},
	}
}

func hasCode(errs []ValidationError, code string) bool {
	for _, err := range errs {
		if err.Code == code {
			return true
		}
	}
	return false
}

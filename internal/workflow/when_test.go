package workflow

import "testing"

func TestParseWhen(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		expr     string
		wantNode string
		wantOp   string
		wantErr  bool
	}{
		{
			name:     "string equality",
			expr:     `classify_intent.intent == "troubleshooting"`,
			wantNode: "classify_intent",
			wantOp:   "==",
		},
		{
			name:     "number comparison",
			expr:     "classify_intent.confidence < 0.65",
			wantNode: "classify_intent",
			wantOp:   "<",
		},
		{
			name:     "boolean comparison",
			expr:     "create_ticket.ok == true",
			wantNode: "create_ticket",
			wantOp:   "==",
		},
		{
			name:    "implicit variable is rejected",
			expr:    `intent == "troubleshooting"`,
			wantErr: true,
		},
		{
			name:    "nested path is rejected",
			expr:    `classify_intent.intent.value == "troubleshooting"`,
			wantErr: true,
		},
		{
			name:    "unsupported operator is rejected",
			expr:    `classify_intent.intent in ["troubleshooting"]`,
			wantErr: true,
		},
		{
			name:    "logical expression is rejected",
			expr:    `classify_intent.intent == "troubleshooting" && retrieve_knowledge.count > 0`,
			wantErr: true,
		},
		{
			name:    "inputs access is rejected",
			expr:    `inputs.message == "x"`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ParseWhen(tt.expr)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseWhen(%q) expected error", tt.expr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseWhen(%q) unexpected error: %v", tt.expr, err)
			}
			if got.Left.NodeID != tt.wantNode {
				t.Fatalf("node id = %q, want %q", got.Left.NodeID, tt.wantNode)
			}
			if got.Op != tt.wantOp {
				t.Fatalf("op = %q, want %q", got.Op, tt.wantOp)
			}
		})
	}
}

func TestWhenConditionEvaluate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		expr    string
		outputs map[string]map[string]any
		want    bool
		wantErr bool
	}{
		{
			name: "string equality true",
			expr: `classify_intent.intent == "troubleshooting"`,
			outputs: map[string]map[string]any{
				"classify_intent": {"intent": "troubleshooting"},
			},
			want: true,
		},
		{
			name: "number comparison true",
			expr: "classify_intent.confidence < 0.65",
			outputs: map[string]map[string]any{
				"classify_intent": {"confidence": 0.44},
			},
			want: true,
		},
		{
			name: "boolean mismatch false",
			expr: "create_ticket.ok == true",
			outputs: map[string]map[string]any{
				"create_ticket": {"ok": false},
			},
			want: false,
		},
		{
			name: "missing node is an error",
			expr: "classify_intent.confidence < 0.65",
			outputs: map[string]map[string]any{
				"other": {"confidence": 0.44},
			},
			wantErr: true,
		},
		{
			name: "type mismatch is an error",
			expr: "classify_intent.confidence < 0.65",
			outputs: map[string]map[string]any{
				"classify_intent": {"confidence": "high"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cond, err := ParseWhen(tt.expr)
			if err != nil {
				t.Fatalf("ParseWhen(%q) unexpected error: %v", tt.expr, err)
			}
			got, err := cond.Evaluate(tt.outputs)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Evaluate(%q) expected error", tt.expr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Evaluate(%q) unexpected error: %v", tt.expr, err)
			}
			if got != tt.want {
				t.Fatalf("Evaluate(%q) = %v, want %v", tt.expr, got, tt.want)
			}
		})
	}
}

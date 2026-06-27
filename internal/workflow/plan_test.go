package workflow

import "testing"

func TestCompileWorkflowPropagatesMaySkipThroughDependents(t *testing.T) {
	t.Parallel()

	plan, issues := CompileWorkflow([]NodeSpec{
		{
			ID:        "classify_intent",
			Component: "intent_classifier",
			Inputs: map[string]string{
				"message": "inputs.message",
			},
		},
		{
			ID:        "handoff",
			Component: "human_handoff",
			When:      `classify_intent.intent == "human_handoff"`,
			Inputs: map[string]string{
				"message": "inputs.message",
			},
		},
		{
			ID:        "summary",
			Component: "answer_generator",
			Inputs: map[string]string{
				"message":   "handoff.message",
				"passages":  "inputs.passages",
				"citations": "inputs.citations",
			},
		},
		{
			ID:        "final_action",
			Component: "human_handoff",
			Inputs: map[string]string{
				"message": "summary.answer",
			},
		},
	})
	if len(issues) > 0 {
		t.Fatalf("CompileWorkflow() issues = %#v", issues)
	}

	if plan.Nodes[1].WhenCondition == nil {
		t.Fatalf("handoff when condition not compiled")
	}
	if !plan.Nodes[1].MaySkip {
		t.Fatalf("handoff should be marked may-skip")
	}
	if !plan.Nodes[2].MaySkip {
		t.Fatalf("summary should inherit may-skip from handoff")
	}
	if !plan.Nodes[3].MaySkip {
		t.Fatalf("final_action should inherit may-skip transitively")
	}
}

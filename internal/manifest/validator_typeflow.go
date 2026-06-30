package manifest

import (
	"fmt"
	"strings"

	"fde-support/internal/registry"
	"fde-support/internal/workflow"
)

// compatibleTypes is the 5x5 type compatibility matrix (flat primitive types only).
// Rows = upstream output type, Columns = downstream expected type.
var compatibleTypes = map[string]map[string]bool{
	"string":  {"string": true},
	"number":  {"number": true},
	"boolean": {"boolean": true},
	"object":  {"object": true},
	"array":   {"array": true},
}

// validateTypeFlow checks that upstream node outputs are type-compatible with downstream node inputs.
// It skips nodes with `when` conditions (may not execute) and only validates flat primitive types.
func validateTypeFlow(nodes []WorkflowNodeSpec, descriptors map[string]registry.ComponentDescriptor, plan *workflow.Plan, add func(string, string, string)) {
	// Build a map of node_id -> output schema
	nodeOutputs := map[string]map[string]string{}
	for _, node := range nodes {
		desc, ok := descriptors[node.Component]
		if !ok {
			continue
		}
		nodeOutputs[node.ID] = desc.OutputSchema
	}

	for i, node := range nodes {
		// Skip nodes with when conditions - their outputs may not be available
		if node.When != "" {
			continue
		}
		desc, ok := descriptors[node.Component]
		if !ok {
			continue
		}
		// For each input reference that points to an upstream node
		for target, ref := range node.Inputs {
			root, field, err := workflow.ParseSimplePath(ref)
			if err != nil || root == "inputs" {
				continue
			}
			// Find the upstream node
			upstreamIdx, ok := plan.NodeByID[root]
			if !ok || upstreamIdx >= i {
				continue
			}
			upstreamNode := nodes[upstreamIdx]
			// Skip upstream nodes with when conditions
			if upstreamNode.When != "" {
				continue
			}
			upstreamOutputs := nodeOutputs[upstreamNode.ID]
			if upstreamOutputs == nil {
				continue
			}
			upstreamType, hasType := upstreamOutputs[field]
			if !hasType {
				// Upstream output schema doesn't declare this field
				continue
			}
			upstreamType = strings.TrimSuffix(upstreamType, "?")

			// Check what type the downstream expects
			downstreamType, hasInput := desc.InputSchema[target]
			if !hasInput {
				// Downstream input schema doesn't declare this field
				continue
			}
			downstreamType = strings.TrimSuffix(downstreamType, "?")

			// Check compatibility
			if compatible, ok := compatibleTypes[upstreamType]; ok && compatible[downstreamType] {
				continue
			}

			path := fmt.Sprintf("workflow.nodes[%d].inputs.%s", i, target)
			add("TYPE_MISMATCH", path,
				fmt.Sprintf("node %q expects %q as %s, but upstream node %q outputs %q as %s",
					node.ID, target, downstreamType, upstreamNode.ID, field, upstreamType))
		}
	}
}

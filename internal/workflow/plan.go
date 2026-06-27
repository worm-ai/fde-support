package workflow

import "fmt"

type NodeSpec struct {
	ID                string
	Component         string
	When              string
	Inputs            map[string]string
	ContinueOnFailure bool
}

type CompiledNode struct {
	NodeSpec
	WhenCondition *WhenCondition
	Dependencies  map[string]bool
	MaySkip       bool
}

type Plan struct {
	Nodes    []CompiledNode
	NodeByID map[string]int
}

type CompileIssue struct {
	Code    string
	Path    string
	Message string
}

func CompileWorkflow(nodes []NodeSpec) (*Plan, []CompileIssue) {
	plan := &Plan{
		Nodes:    make([]CompiledNode, len(nodes)),
		NodeByID: map[string]int{},
	}
	issues := make([]CompileIssue, 0)

	for i, node := range nodes {
		if node.ID == "" {
			issues = append(issues, CompileIssue{
				Code:    "MISSING_REQUIRED_FIELD",
				Path:    fmt.Sprintf("workflow.nodes[%d].id", i),
				Message: "workflow node id is required",
			})
			continue
		}
		if _, exists := plan.NodeByID[node.ID]; exists {
			issues = append(issues, CompileIssue{
				Code:    "DUPLICATE_ID",
				Path:    fmt.Sprintf("workflow.nodes[%d].id", i),
				Message: "workflow node id must be unique",
			})
			continue
		}
		plan.NodeByID[node.ID] = i
	}

	for i, node := range nodes {
		compiled := CompiledNode{
			NodeSpec:     node,
			Dependencies: map[string]bool{},
		}
		if node.When != "" {
			cond, err := ParseWhen(node.When)
			if err == nil {
				compiled.WhenCondition = cond
				compiled.Dependencies[cond.Left.NodeID] = true
			}
		}
		for _, ref := range node.Inputs {
			root, _, err := ParseSimplePath(ref)
			if err == nil && root != "inputs" {
				compiled.Dependencies[root] = true
			}
		}
		if node.When != "" && compiled.WhenCondition == nil {
			issues = append(issues, CompileIssue{
				Code:    "INVALID_WHEN",
				Path:    fmt.Sprintf("workflow.nodes[%d].when", i),
				Message: "when expression is invalid",
			})
		}
		plan.Nodes[i] = compiled
	}

	for i := range plan.Nodes {
		compiled := &plan.Nodes[i]
		if compiled.WhenCondition != nil {
			compiled.MaySkip = true
		}
		for dep := range compiled.Dependencies {
			if dep == "inputs" {
				continue
			}
			depIndex, ok := plan.NodeByID[dep]
			if !ok || depIndex >= i {
				continue
			}
			if plan.Nodes[depIndex].MaySkip {
				compiled.MaySkip = true
				break
			}
		}
	}

	return plan, issues
}

func (p *Plan) Node(id string) (CompiledNode, bool) {
	if p == nil {
		return CompiledNode{}, false
	}
	index, ok := p.NodeByID[id]
	if !ok {
		return CompiledNode{}, false
	}
	return p.Nodes[index], true
}

package manifest

import (
	"fmt"
	"path/filepath"
	"strings"

	"fde-support/internal/registry"
	"fde-support/internal/shared"
	"fde-support/internal/w2a"
	"fde-support/internal/workflow"
)

type ValidationError struct {
	Code    string `json:"code"`
	Path    string `json:"path"`
	Message string `json:"message"`
	Hint    string `json:"hint,omitempty"`
}

type Validator struct {
	components registry.ComponentRegistry
	sensors    w2a.SensorRegistry
}

// M2: supported platform capabilities
var supportedCapabilities = map[string]bool{
	"model.generate":   true,
	"knowledge.search": true,
	"knowledge.query":  true, // M2
	"http.call":        true, // M2
}

var supportedReleaseChecks = map[string]bool{
	"model_credentials_configured":  true,
	"sensor_credentials_configured": true,
	"action_credentials_configured": true,
	"signal_ingress_reachable":      true,
	"knowledge_quality_passed":      true,
	"eval_gates_passed":             true,
	"observability_enabled":         true,
	"security_baseline_passed":      true,
}

var supportedSolutionTypes = map[string]bool{
	"customer-support": true,
	"data-inquiry":     true,
	"alert-escalation": true,
	"approval-flow":    true,
}

func validateComponentRequires(componentID string, desc registry.ComponentDescriptor, path string, add func(string, string, string)) {
	for _, req := range desc.Requires {
		available, known := supportedCapabilities[req]
		if !known {
			add("UNKNOWN_COMPONENT_REQUIRES", path+".requires", "component requires unknown capability: "+req)
			continue
		}
		if !available {
			add("COMPONENT_REQUIRES_UNAVAILABLE", path+".requires", "component requires capability that is not available in this phase: "+req)
		}
	}
}

func NewValidator(components registry.ComponentRegistry, sensors w2a.SensorRegistry) *Validator {
	return &Validator{components: components, sensors: sensors}
}

func (v *Validator) Validate(m *SolutionManifest) []ValidationError {
	var errs []ValidationError
	add := func(code, path, message string) {
		errs = append(errs, ValidationError{Code: code, Path: path, Message: message})
	}

	if strings.TrimSpace(m.APIVersion) == "" {
		add("MISSING_REQUIRED_FIELD", "apiVersion", "apiVersion is required")
	} else if m.APIVersion != "solution.codex/v1" {
		add("UNSUPPORTED_API_VERSION", "apiVersion", "apiVersion must be solution.codex/v1")
	}
	if m.Kind != "Solution" {
		add("INVALID_KIND", "kind", "kind must be Solution")
	}
	if strings.TrimSpace(m.SolutionType) == "" {
		add("MISSING_REQUIRED_FIELD", "solutionType", "solutionType is required")
	} else if !supportedSolutionTypes[m.SolutionType] {
		add("UNSUPPORTED_SOLUTION_TYPE", "solutionType", "solutionType is not supported")
	}
	if strings.TrimSpace(m.Metadata.Name) == "" {
		add("MISSING_REQUIRED_FIELD", "metadata.name", "metadata.name is required")
	}
	if strings.TrimSpace(m.Metadata.Version) == "" {
		add("MISSING_REQUIRED_FIELD", "metadata.version", "metadata.version is required")
	}
	if strings.TrimSpace(m.Workflow.Entrypoint) == "" {
		add("MISSING_REQUIRED_FIELD", "workflow.entrypoint", "workflow.entrypoint is required")
	}
	if len(m.Workflow.Nodes) > 0 && m.Workflow.Entrypoint != "" && m.Workflow.Entrypoint != m.Workflow.Nodes[0].ID {
		add("INVALID_ENTRYPOINT", "workflow.entrypoint", "workflow.entrypoint must be the id of the first workflow node")
	}

	sensorByID := map[string]SensorSpec{}
	for i, sensor := range m.Perception.Sensors {
		path := fmt.Sprintf("perception.sensors[%d]", i)
		if sensor.ID == "" {
			add("MISSING_REQUIRED_FIELD", path+".id", "sensor id is required")
		}
		if _, exists := sensorByID[sensor.ID]; sensor.ID != "" && exists {
			add("DUPLICATE_ID", path+".id", "sensor id must be unique")
		}
		sensorByID[sensor.ID] = sensor
		if _, err := v.sensors.Resolve(sensor.Ref); err != nil {
			add("UNKNOWN_SENSOR_REF", path+".ref", err.Error())
		}
		if sensor.Ref == w2a.BuiltinWebhookSensorRef {
			if endpoint, ok := sensor.Config["endpointPath"].(string); !ok || !strings.HasPrefix(endpoint, "/") {
				add("INVALID_SENSOR_CONFIG", path+".config.endpointPath", "webhook sensor endpointPath must start with /")
			}
			validateSecretMap(sensor.Config, path+".config", add)
		}
	}

	for i, trigger := range m.Perception.Triggers {
		path := fmt.Sprintf("perception.triggers[%d]", i)
		sensor, ok := sensorByID[trigger.Sensor]
		if !ok {
			add("UNKNOWN_SENSOR", path+".sensor", "trigger references an unknown sensor")
			continue
		}
		if !contains(sensor.SignalTypes, trigger.SignalType) {
			add("TRIGGER_SIGNAL_TYPE_NOT_ALLOWED", path+".signalType", "trigger signalType must be declared by its sensor")
		}
		if trigger.RouteTo != "" && trigger.RouteTo != m.Workflow.Entrypoint {
			add("UNKNOWN_WORKFLOW_ROUTE", path+".routeTo", "trigger routeTo must match workflow.entrypoint in MVP")
		}
	}

	componentByID := map[string]ComponentSpec{}
	componentDescByID := map[string]registry.ComponentDescriptor{}
	for i, component := range m.Components {
		path := fmt.Sprintf("components[%d]", i)
		if component.ID == "" {
			add("MISSING_REQUIRED_FIELD", path+".id", "component id is required")
		}
		if _, exists := componentByID[component.ID]; component.ID != "" && exists {
			add("DUPLICATE_ID", path+".id", "component id must be unique")
		}
		componentByID[component.ID] = component
		desc, err := v.components.Resolve(component.Ref)
		if err != nil {
			add("UNKNOWN_COMPONENT_REF", path+".ref", err.Error())
			continue
		}
		componentDescByID[component.ID] = desc
		if string(desc.Category) != component.Category {
			add("COMPONENT_CATEGORY_MISMATCH", path+".category", "component category does not match registry descriptor")
		}
		validateComponentRequires(component.ID, desc, path, add)
		if component.Category != string(registry.CategoryProcessor) && component.Category != string(registry.CategoryAction) {
			add("INVALID_COMPONENT_CATEGORY", path+".category", "component category must be processor or action")
		}
		validateSecretMap(component.Config, path+".config", add)
	}

	sourceByID := map[string]KnowledgeSourceSpec{}
	schemaByID := map[string]KnowledgeSchemaSpec{}
	for i, source := range m.Knowledge.Sources {
		path := fmt.Sprintf("knowledge.sources[%d]", i)
		if source.ID == "" {
			add("MISSING_REQUIRED_FIELD", path+".id", "knowledge source id is required")
		}
		if _, exists := sourceByID[source.ID]; source.ID != "" && exists {
			add("DUPLICATE_ID", path+".id", "knowledge source id must be unique")
		}
		sourceByID[source.ID] = source
		if !supportedKnowledgeSourceType(source.Type) {
			add("UNSUPPORTED_KNOWLEDGE_SOURCE_TYPE", path+".type", "knowledge source type must be jsonl, csv, table, or rules")
		}
		validateRelativeManifestPath(source.URI, path+".uri", add)
	}
	for i, schema := range m.Knowledge.Schemas {
		path := fmt.Sprintf("knowledge.schemas[%d]", i)
		if _, exists := schemaByID[schema.ID]; schema.ID != "" && exists {
			add("DUPLICATE_ID", path+".id", "knowledge schema id must be unique")
		}
		schemaByID[schema.ID] = schema
	}
	for i, source := range m.Knowledge.Sources {
		if source.Schema != "" {
			if _, ok := schemaByID[source.Schema]; !ok {
				add("UNKNOWN_KNOWLEDGE_SCHEMA", fmt.Sprintf("knowledge.sources[%d].schema", i), "knowledge source schema must exist")
			}
		}
	}
	for i, gate := range m.Knowledge.QualityGates {
		for _, scope := range gate.Scope {
			if _, ok := schemaByID[scope]; !ok {
				add("UNKNOWN_KNOWLEDGE_SCHEMA", fmt.Sprintf("knowledge.qualityGates[%d].scope", i), "quality gate scope must reference a known schema")
			}
		}
	}

	nodeSpecs := make([]workflow.NodeSpec, len(m.Workflow.Nodes))
	for i, node := range m.Workflow.Nodes {
		nodeSpecs[i] = workflow.NodeSpec{
			ID:                node.ID,
			Component:         node.Component,
			When:              node.When,
			Inputs:            node.Inputs,
			ContinueOnFailure: node.ContinueOnFailure,
		}
	}
	plan, compileIssues := workflow.CompileWorkflow(nodeSpecs)
	for _, issue := range compileIssues {
		add(issue.Code, issue.Path, issue.Message)
	}

	// Type flow validation: check upstream/downstream type compatibility
	validateTypeFlow(m.Workflow.Nodes, componentDescByID, plan, add)

	for i, node := range m.Workflow.Nodes {
		path := fmt.Sprintf("workflow.nodes[%d]", i)
		desc, hasDesc := componentDescByID[node.Component]
		if _, ok := componentByID[node.Component]; !ok {
			add("UNKNOWN_COMPONENT", path+".component", "workflow node references unknown component")
		}
		if node.When != "" {
			if i < len(plan.Nodes) && plan.Nodes[i].WhenCondition == nil {
				add("INVALID_WHEN", path+".when", "when expression is invalid")
			} else if i < len(plan.Nodes) {
				cond := plan.Nodes[i].WhenCondition
				if upstreamIndex, ok := plan.NodeByID[cond.Left.NodeID]; !ok || upstreamIndex >= i {
					add("UNKNOWN_NODE_REFERENCE", path+".when", "when references an unknown or later node")
				} else if plan.Nodes[upstreamIndex].MaySkip {
					add("WORKFLOW_UNSAFE_DEPENDENCY", path+".when", "when cannot reference output from a node that may be skipped")
				}
			}
		}
		if hasDesc && desc.Category == registry.CategoryProcessor && node.ContinueOnFailure {
			add("INVALID_CONTINUE_ON_FAILURE", path+".continueOnFailure", "processor nodes cannot set continueOnFailure")
		}
		validateNodeInputs(node, i, plan, desc, path, add)
	}

	if m.Workflow.OnError.FallbackNode != "" {
		fallback, ok := plan.Node(m.Workflow.OnError.FallbackNode)
		if !ok {
			add("UNKNOWN_FALLBACK_NODE", "workflow.onError.fallbackNode", "fallbackNode must reference workflow.nodes[].id")
		} else {
			for target, ref := range fallback.Inputs {
				root, _, err := workflow.ParseSimplePath(ref)
				if err != nil || root != "inputs" {
					add("FALLBACK_INPUT_UNSAFE", "workflow.nodes."+fallback.ID+".inputs."+target, "fallback node inputs may only reference inputs.field in M1")
				}
			}
		}
	}

	for triggerType, mapping := range m.Workflow.InputMapping {
		allowed := map[string]bool{}
		switch triggerType {
		case "chat":
			allowed["request"] = true
		case "w2a_signal":
			allowed["signal"] = true
		default:
			add("INVALID_INPUT_MAPPING", "workflow.inputMapping."+triggerType, "inputMapping trigger type must be chat or w2a_signal")
			continue
		}
		for target, ref := range mapping {
			if err := workflow.ValidateSimplePath(ref, allowed); err != nil {
				add("INVALID_INPUT_MAPPING", "workflow.inputMapping."+triggerType+"."+target, err.Error())
			}
		}
	}

	for i, binding := range m.Runtime.KnowledgeBindings {
		path := fmt.Sprintf("runtime.knowledgeBindings[%d]", i)
		if _, ok := componentByID[binding.Component]; !ok {
			add("UNKNOWN_COMPONENT", path+".component", "knowledge binding component must exist")
		}
		for _, source := range binding.Sources {
			if _, ok := sourceByID[source]; !ok {
				add("UNKNOWN_KNOWLEDGE_SOURCE", path+".sources", "knowledge binding source must exist")
			}
		}
	}

	envByName := map[string]EnvironmentSpec{}
	for i, env := range m.Delivery.Environments {
		path := fmt.Sprintf("delivery.environments[%d]", i)
		if env.Name == "" {
			add("MISSING_REQUIRED_FIELD", path+".name", "environment name is required")
		}
		if _, exists := envByName[env.Name]; env.Name != "" && exists {
			add("DUPLICATE_ID", path+".name", "environment name must be unique")
		}
		envByName[env.Name] = env
		validateSecretMap(env.Config, path+".config", add)
		for key := range env.Config {
			if !allowedEnvironmentOverride(key) {
				add("UNSUPPORTED_ENVIRONMENT_OVERRIDE", path+".config."+key, "environment config key is not allowed in M1")
			}
		}
	}

	metricSet := map[string]bool{}
	for _, metric := range m.Evaluation.Metrics {
		metricSet[metric] = true
	}
	for i, gate := range m.Evaluation.Gates {
		path := fmt.Sprintf("evaluation.gates[%d]", i)
		if !metricSet[gate.Metric] {
			add("UNKNOWN_EVALUATION_METRIC", path+".metric", "gate metric must be listed in evaluation.metrics")
		}
		if gate.Severity != "" && gate.Severity != "block" && gate.Severity != "warn" {
			add("INVALID_EVALUATION_GATE", path+".severity", "gate severity must be block or warn")
		}
		if gate.Schedule != "" && gate.Schedule != "onRelease" && gate.Schedule != "weekly" {
			add("INVALID_EVALUATION_GATE", path+".schedule", "gate schedule must be onRelease or weekly")
		}
	}

	for i, check := range m.Delivery.ReleaseChecks {
		if !supportedReleaseChecks[check] {
			add("UNKNOWN_RELEASE_CHECK", fmt.Sprintf("delivery.releaseChecks[%d]", i), "release check is not supported")
		}
	}

	return errs
}

func validateNodeInputs(node WorkflowNodeSpec, currentIndex int, plan *workflow.Plan, desc registry.ComponentDescriptor, path string, add func(string, string, string)) {
	for target, ref := range node.Inputs {
		root, _, err := workflow.ParseSimplePath(ref)
		if err != nil {
			add("INVALID_NODE_INPUT", path+".inputs."+target, "node input must reference inputs.field or node_id.field")
			continue
		}
		if root == "inputs" {
			continue
		}
		upstreamIndex, ok := plan.NodeByID[root]
		if !ok || upstreamIndex >= currentIndex {
			add("UNKNOWN_NODE_REFERENCE", path+".inputs."+target, "node input references an unknown or later node")
			continue
		}
		if plan.Nodes[upstreamIndex].MaySkip {
			add("WORKFLOW_UNSAFE_DEPENDENCY", path+".inputs."+target, "node input references output from a node that may be skipped")
		}
	}
	if desc.Ref == "" {
		return
	}
	for required := range desc.InputSchema {
		if strings.HasSuffix(desc.InputSchema[required], "?") {
			continue
		}
		if _, ok := node.Inputs[required]; !ok {
			add("COMPONENT_INPUT_MISSING", path+".inputs."+required, "node inputs must satisfy component input schema")
		}
	}
}

func validateSecretMap(values map[string]any, path string, add func(string, string, string)) {
	for key, value := range values {
		if !shared.IsSensitiveRefKey(key) {
			continue
		}
		s, ok := value.(string)
		if !ok || !shared.IsEnvSecretRef(s) {
			add("INVALID_SECRET_REF", path+"."+key, "sensitive references must use env:VAR_NAME")
		}
	}
}

func validateRelativeManifestPath(uri string, path string, add func(string, string, string)) {
	if strings.TrimSpace(uri) == "" {
		return
	}
	if filepath.IsAbs(uri) || filepath.VolumeName(uri) != "" {
		add("INVALID_KNOWLEDGE_SOURCE_URI", path, "knowledge source uri must be relative to the manifest directory")
		return
	}
	clean := filepath.Clean(uri)
	if clean == "." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || clean == ".." {
		add("INVALID_KNOWLEDGE_SOURCE_URI", path, "knowledge source uri must not escape the manifest directory")
	}
}

func allowedEnvironmentOverride(key string) bool {
	switch key {
	case "modelKeyRef", "defaultModel", "fallbackModel", "maxLatencyMs", "maxCostPerRunUsd", "tracePath", "retainDays":
		return true
	default:
		return false
	}
}

func supportedKnowledgeSourceType(sourceType string) bool {
	switch sourceType {
	case "jsonl", "csv", "table", "rules":
		return true
	default:
		return false
	}
}

func contains(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

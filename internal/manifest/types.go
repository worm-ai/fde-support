package manifest

type SolutionManifest struct {
	APIVersion   string          `yaml:"apiVersion" json:"apiVersion"`
	Kind         string          `yaml:"kind" json:"kind"`
	SolutionType string          `yaml:"solutionType" json:"solutionType"`
	Metadata     MetadataSpec    `yaml:"metadata" json:"metadata"`
	Perception   PerceptionSpec  `yaml:"perception" json:"perception"`
	Knowledge    KnowledgeSpec   `yaml:"knowledge" json:"knowledge"`
	Components   []ComponentSpec `yaml:"components" json:"components"`
	Workflow     WorkflowSpec    `yaml:"workflow" json:"workflow"`
	Runtime      RuntimeSpec     `yaml:"runtime" json:"runtime"`
	Evaluation   EvaluationSpec  `yaml:"evaluation" json:"evaluation"`
	Delivery     DeliverySpec    `yaml:"delivery" json:"delivery"`

	BaseDir string `yaml:"-" json:"-"`
	Path    string `yaml:"-" json:"-"`
}

type MetadataSpec struct {
	Name     string `yaml:"name" json:"name"`
	Version  string `yaml:"version" json:"version"`
	Owner    string `yaml:"owner" json:"owner"`
	Industry string `yaml:"industry" json:"industry"`
}

type PerceptionSpec struct {
	Sensors  []SensorSpec       `yaml:"sensors" json:"sensors"`
	Triggers []TriggerRouteSpec `yaml:"triggers" json:"triggers"`
}

type SensorSpec struct {
	ID          string         `yaml:"id" json:"id"`
	Ref         string         `yaml:"ref" json:"ref"`
	SignalTypes []string       `yaml:"signalTypes" json:"signalTypes"`
	Config      map[string]any `yaml:"config" json:"config"`
}

type TriggerRouteSpec struct {
	ID         string `yaml:"id" json:"id"`
	Sensor     string `yaml:"sensor" json:"sensor"`
	SignalType string `yaml:"signalType" json:"signalType"`
	RouteTo    string `yaml:"routeTo" json:"routeTo"`
}

type KnowledgeSpec struct {
	Sources      []KnowledgeSourceSpec `yaml:"sources" json:"sources"`
	Schemas      []KnowledgeSchemaSpec `yaml:"schemas" json:"schemas"`
	QualityGates []QualityGateSpec     `yaml:"qualityGates" json:"qualityGates"`
}

type KnowledgeSourceSpec struct {
	ID     string `yaml:"id" json:"id"`
	Type   string `yaml:"type" json:"type"`
	URI    string `yaml:"uri" json:"uri"`
	Schema string `yaml:"schema" json:"schema"`
}

type KnowledgeSchemaSpec struct {
	ID     string   `yaml:"id" json:"id"`
	Fields []string `yaml:"fields" json:"fields"`
}

type QualityGateSpec struct {
	Type       string   `yaml:"type" json:"type"`
	Severity   string   `yaml:"severity" json:"severity"`
	Scope      []string `yaml:"scope" json:"scope"`
	MaxAgeDays int      `yaml:"maxAgeDays" json:"maxAgeDays"`
}

type ComponentSpec struct {
	ID       string         `yaml:"id" json:"id"`
	Category string         `yaml:"category" json:"category"`
	Ref      string         `yaml:"ref" json:"ref"`
	Config   map[string]any `yaml:"config" json:"config"`
}

type WorkflowSpec struct {
	Entrypoint   string                       `yaml:"entrypoint" json:"entrypoint"`
	OnError      OnErrorSpec                  `yaml:"onError" json:"onError"`
	InputMapping map[string]map[string]string `yaml:"inputMapping" json:"inputMapping"`
	Nodes        []WorkflowNodeSpec           `yaml:"nodes" json:"nodes"`
}

type OnErrorSpec struct {
	Retry        int    `yaml:"retry" json:"retry"`
	FallbackNode string `yaml:"fallbackNode" json:"fallbackNode"`
}

type WorkflowNodeSpec struct {
	ID                string            `yaml:"id" json:"id"`
	Component         string            `yaml:"component" json:"component"`
	When              string            `yaml:"when" json:"when"`
	Inputs            map[string]string `yaml:"inputs" json:"inputs"`
	ContinueOnFailure bool              `yaml:"continueOnFailure" json:"continueOnFailure"`
}

type RuntimeSpec struct {
	KnowledgeBindings []KnowledgeBindingSpec `yaml:"knowledgeBindings" json:"knowledgeBindings"`
	ModelPolicy       ModelPolicySpec        `yaml:"modelPolicy" json:"modelPolicy"`
	Observability     ObservabilitySpec      `yaml:"observability" json:"observability"`
	Embedding         map[string]any         `yaml:"embedding" json:"embedding"`
}

type KnowledgeBindingSpec struct {
	Component string   `yaml:"component" json:"component"`
	Sources   []string `yaml:"sources" json:"sources"`
}

type ModelPolicySpec struct {
	DefaultModel     string  `yaml:"defaultModel" json:"defaultModel"`
	FallbackModel    string  `yaml:"fallbackModel" json:"fallbackModel"`
	MaxLatencyMs     int     `yaml:"maxLatencyMs" json:"maxLatencyMs"`
	MaxCostPerRunUsd float64 `yaml:"maxCostPerRunUsd" json:"maxCostPerRunUsd"`
}

type ObservabilitySpec struct {
	Trace      string `yaml:"trace" json:"trace"`
	LogInputs  string `yaml:"logInputs" json:"logInputs"`
	LogOutputs bool   `yaml:"logOutputs" json:"logOutputs"`
	RetainDays int    `yaml:"retainDays" json:"retainDays"`
}

type EvaluationSpec struct {
	Datasets []EvaluationDatasetSpec `yaml:"datasets" json:"datasets"`
	Metrics  []string                `yaml:"metrics" json:"metrics"`
	Gates    []EvaluationGateSpec    `yaml:"gates" json:"gates"`
}

type EvaluationDatasetSpec struct {
	ID         string `yaml:"id" json:"id"`
	URI        string `yaml:"uri" json:"uri"`
	CaseFormat string `yaml:"caseFormat" json:"caseFormat"`
}

type EvaluationGateSpec struct {
	Metric   string  `yaml:"metric" json:"metric"`
	Min      float64 `yaml:"min" json:"min"`
	Severity string  `yaml:"severity" json:"severity"`
	Schedule string  `yaml:"schedule" json:"schedule"`
}

type DeliverySpec struct {
	Environments  []EnvironmentSpec `yaml:"environments" json:"environments"`
	Security      SecuritySpec      `yaml:"security" json:"security"`
	ReleaseChecks []string          `yaml:"releaseChecks" json:"releaseChecks"`
}

type EnvironmentSpec struct {
	Name   string         `yaml:"name" json:"name"`
	Type   string         `yaml:"type" json:"type"`
	Config map[string]any `yaml:"config" json:"config"`
}

type SecuritySpec struct {
	PIIDetection           string `yaml:"piiDetection" json:"piiDetection"`
	PromptInjectionDefense string `yaml:"promptInjectionDefense" json:"promptInjectionDefense"`
	RBAC                   string `yaml:"rbac" json:"rbac"`
}

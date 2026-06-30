package registry

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"fde-support/internal/shared"
)

type ComponentCategory string

const (
	CategoryProcessor ComponentCategory = "processor"
	CategoryAction    ComponentCategory = "action"
)

type ComponentDescriptor struct {
	Ref          string
	Category     ComponentCategory
	Factory      string
	ConfigSchema map[string]string
	InputSchema  map[string]string
	OutputSchema map[string]string
	Requires     []string
}

type Component interface {
	ID() string
	Category() ComponentCategory
	Run(ctx context.Context, input map[string]any, runtime RuntimeContext) (map[string]any, error)
}

type RuntimeContext interface {
	Environment() string
	Knowledge() KnowledgeReader
	Request() RuntimeRequestMetadata
	Error() *RuntimeErrorSummary
	Actions() []ActionSummary
	Model() ModelGateway
	HTTP() HTTPCaller
	Logger() Logger
}

type Logger interface {
	Info(traceID string, msg string, fields map[string]any)
	Error(traceID string, msg string, fields map[string]any)
}

type RuntimeRequestMetadata struct {
	TriggerType string
	Sensor      string
	SignalType  string
}

type RuntimeErrorSummary struct {
	FailedNode string `json:"failedNode"`
	Message    string `json:"message"`
	Type       string `json:"type"`
	Attempts   int    `json:"attempts"`
}
type ModelGateway interface {
	Generate(ctx context.Context, req ModelGenerateRequest) (ModelGenerateResponse, error)
}

type ModelGenerateRequest struct {
	Model     string
	Messages  []ModelMessage
	MaxTokens int
}

type ModelMessage struct {
	Role    string
	Content string
}

type ModelGenerateResponse struct {
	Model   string
	Content string
	Usage   ModelUsageSummary
}

// HTTPCaller allows components to make external HTTP requests.
type HTTPCaller interface {
	Call(ctx context.Context, req HTTPCallRequest) (HTTPCallResponse, error)
}

// HTTPCallRequest is a component-facing HTTP request.
type HTTPCallRequest struct {
	URL     string
	Method  string
	Headers map[string]string
	Body    string
}

// HTTPCallResponse is a component-facing HTTP response.
type HTTPCallResponse struct {
	StatusCode int
	Body       string
	Headers    map[string]string
}

type ModelUsageSummary struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	CostUSD          float64
}

type Citation struct {
	Source string `json:"source"`
	Ref    string `json:"ref"`
}

type ActionSummary struct {
	Node   string         `json:"node"`
	Output map[string]any `json:"output"`
}

type KnowledgeReader interface {
	Retrieve(ctx context.Context, query string, topK int) (KnowledgeResult, error)
}

type KnowledgeResult struct {
	Passages  []string         `json:"passages"`
	Citations []Citation       `json:"citations"`
	Raw       []map[string]any `json:"raw,omitempty"`
}

type ComponentRegistry interface {
	Resolve(ref string) (ComponentDescriptor, error)
	Instantiate(id string, ref string, config map[string]any) (Component, error)
}

type BuiltinComponentRegistry struct {
	descriptors map[string]ComponentDescriptor
	factories   map[string]func(id string, cfg map[string]any) Component
	root        string
}

func NewBuiltinComponentRegistry() *BuiltinComponentRegistry {
	return &BuiltinComponentRegistry{
		descriptors: builtinComponentDescriptors(),
		factories:   builtinComponentFactories(),
	}
}

func NewBuiltinComponentRegistryFromRoot(root string) (*BuiltinComponentRegistry, error) {
	r := NewBuiltinComponentRegistry()
	r.root = root
	if err := r.loadFromRoot(root); err != nil {
		return nil, err
	}
	return r, nil
}

type componentFile struct {
	Ref          string            `yaml:"ref"`
	Category     string            `yaml:"category"`
	Factory      string            `yaml:"factory"`
	ConfigSchema map[string]string `yaml:"configSchema"`
	InputSchema  map[string]string `yaml:"inputSchema"`
	OutputSchema map[string]string `yaml:"outputSchema"`
}

func (r *BuiltinComponentRegistry) loadFromRoot(root string) error {
	if root == "" {
		return nil
	}
	registryRoot := filepath.Join(root, "components", "registry")
	if _, err := os.Stat(registryRoot); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	return filepath.WalkDir(registryRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Base(path) != "component.yaml" {
			return nil
		}
		return r.loadComponentFile(path)
	})
}

func (r *BuiltinComponentRegistry) loadComponentFile(path string) error {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var file componentFile
	if err := yaml.Unmarshal(bytes, &file); err != nil {
		return err
	}
	if file.Ref == "" {
		return fmt.Errorf("%s: component ref is required", path)
	}
	desc := ComponentDescriptor{
		Ref:          file.Ref,
		Factory:      file.Factory,
		ConfigSchema: file.ConfigSchema,
		InputSchema:  file.InputSchema,
		OutputSchema: file.OutputSchema,
	}
	if file.Category != "" {
		desc.Category = ComponentCategory(file.Category)
	}
	r.descriptors[file.Ref] = desc
	return nil
}

func builtinComponentDescriptors() map[string]ComponentDescriptor {
	return map[string]ComponentDescriptor{
		"registry.intent.support-router@1.0.0": {
			Ref:          "registry.intent.support-router@1.0.0",
			Category:     CategoryProcessor,
			Factory:      "intent_classifier",
			ConfigSchema: map[string]string{"intents": "array?"},
			InputSchema:  map[string]string{"message": "string"},
			OutputSchema: map[string]string{"intent": "string", "confidence": "number"},
		},
		"registry.intent.beverage-router@1.0.0": {
			Ref:          "registry.intent.beverage-router@1.0.0",
			Category:     CategoryProcessor,
			Factory:      "intent_classifier",
			ConfigSchema: map[string]string{"intents": "array?"},
			InputSchema:  map[string]string{"message": "string"},
			OutputSchema: map[string]string{"intent": "string", "confidence": "number"},
		},
		"registry.intent.severity-beverage@1.0.0": {
			Ref:          "registry.intent.severity-beverage@1.0.0",
			Category:     CategoryProcessor,
			Factory:      "severity_checker",
			ConfigSchema: map[string]string{"criticalKeywords": "array?"},
			InputSchema:  map[string]string{"message": "string", "intent": "string?"},
			OutputSchema: map[string]string{"level": "string"},
		},
		"registry.retriever.local-keyword@1.0.0": {
			Ref:          "registry.retriever.local-keyword@1.0.0",
			Category:     CategoryProcessor,
			Factory:      "keyword_retriever",
			ConfigSchema: map[string]string{"topK": "number?", "requireCitation": "boolean?"},
			InputSchema:  map[string]string{"query": "string"},
			OutputSchema: map[string]string{"passages": "array", "citations": "array"},
		},
		"registry.agent.cited-answer@1.0.0": {
			Ref:          "registry.agent.cited-answer@1.0.0",
			Category:     CategoryProcessor,
			Factory:      "cited_answer",
			ConfigSchema: map[string]string{"style": "string?", "requireGrounding": "boolean?"},
			InputSchema:  map[string]string{"message": "string", "passages": "array", "citations": "array"},
			OutputSchema: map[string]string{"answer": "string", "citations": "array"},
		},
		"registry.agent.cited-answer@1.2.0": {
			Ref:          "registry.agent.cited-answer@1.2.0",
			Category:     CategoryProcessor,
			Factory:      "cited_answer",
			ConfigSchema: map[string]string{"style": "string?", "requireGrounding": "boolean?"},
			InputSchema:  map[string]string{"message": "string", "passages": "array", "citations": "array"},
			OutputSchema: map[string]string{"answer": "string", "citations": "array"},
		},
		"registry.action.human-handoff@1.0.0": {
			Ref:          "registry.action.human-handoff@1.0.0",
			Category:     CategoryAction,
			Factory:      "human_handoff",
			ConfigSchema: map[string]string{"queue": "string"},
			InputSchema:  map[string]string{"message": "string"},
			OutputSchema: map[string]string{"status": "string"},
		},
		"registry.action.mock-human-handoff@1.0.0": {
			Ref:          "registry.action.mock-human-handoff@1.0.0",
			Category:     CategoryAction,
			Factory:      "human_handoff",
			ConfigSchema: map[string]string{"queue": "string"},
			InputSchema:  map[string]string{"message": "string"},
			OutputSchema: map[string]string{"status": "string"},
		},
		"registry.action.mock-create-service-ticket@1.0.0": {
			Ref:          "registry.action.mock-create-service-ticket@1.0.0",
			Category:     CategoryAction,
			Factory:      "mock_create_service_ticket",
			ConfigSchema: map[string]string{"system": "string?", "apiKeyRef": "string?", "simulateFailure": "boolean?"},
			InputSchema:  map[string]string{"message": "string", "level": "string?"},
			OutputSchema: map[string]string{"status": "string", "ticketId": "string?"},
		},
		"registry.processor.llm-extractor@1.0.0": {
			Ref:          "registry.processor.llm-extractor@1.0.0",
			Category:     CategoryProcessor,
			Factory:      "llm_extractor",
			ConfigSchema: map[string]string{"schema": "object?"},
			InputSchema:  map[string]string{"text": "string?"},
			OutputSchema: map[string]string{"status": "string", "extracted": "string?"},
			Requires:     []string{"model.generate"},
		},
		"registry.processor.data-query@1.0.0": {
			Ref:          "registry.processor.data-query@1.0.0",
			Category:     CategoryProcessor,
			Factory:      "data_query",
			ConfigSchema: map[string]string{"source": "string", "query": "string?"},
			InputSchema:  map[string]string{"query": "string?"},
			OutputSchema: map[string]string{"status": "string", "rows": "array", "count": "number"},
			Requires:     []string{"knowledge.query"},
		},
		"registry.processor.rule-evaluator@1.0.0": {
			Ref:          "registry.processor.rule-evaluator@1.0.0",
			Category:     CategoryProcessor,
			Factory:      "rule_evaluator",
			ConfigSchema: map[string]string{"rules": "array"},
			InputSchema:  map[string]string{},
			OutputSchema: map[string]string{"status": "string", "matched": "boolean", "result": "string?"},
		},
		"registry.action.http-caller@1.0.0": {
			Ref:          "registry.action.http-caller@1.0.0",
			Category:     CategoryAction,
			Factory:      "http_caller",
			ConfigSchema: map[string]string{"url": "string", "method": "string?", "headers": "object?", "bodyTemplate": "string?", "timeoutMs": "number?"},
			InputSchema:  map[string]string{},
			OutputSchema: map[string]string{"status": "string", "statusCode": "number?", "body": "object?"},
			Requires:     []string{"http.call"},
		},
	}
}

func builtinComponentFactories() map[string]func(id string, cfg map[string]any) Component {
	factories := map[string]func(id string, cfg map[string]any) Component{
		"registry.intent.support-router@1.0.0":             newIntentClassifier,
		"registry.intent.beverage-router@1.0.0":            newIntentClassifier,
		"registry.intent.severity-beverage@1.0.0":          newSeverityChecker,
		"registry.retriever.local-keyword@1.0.0":           newKeywordRetriever,
		"registry.agent.cited-answer@1.0.0":                newCitedAnswerAgent,
		"registry.agent.cited-answer@1.2.0":                newCitedAnswerAgent,
		"registry.action.human-handoff@1.0.0":              newHumanHandoff,
		"registry.action.mock-human-handoff@1.0.0":         newHumanHandoff,
		"registry.action.mock-create-service-ticket@1.0.0": newMockCreateServiceTicket,
		"registry.processor.llm-extractor@1.0.0":           newLLMExtractor,
		"registry.processor.data-query@1.0.0":              newDataQuery,
		"registry.processor.rule-evaluator@1.0.0":          newRuleEvaluator,
		"registry.action.http-caller@1.0.0":                newHTTPCaller,
	}
	factories["intent_classifier"] = newIntentClassifier
	factories["severity_checker"] = newSeverityChecker
	factories["keyword_retriever"] = newKeywordRetriever
	factories["cited_answer"] = newCitedAnswerAgent
	factories["human_handoff"] = newHumanHandoff
	factories["mock_create_service_ticket"] = newMockCreateServiceTicket
	factories["llm_extractor"] = newLLMExtractor
	factories["data_query"] = newDataQuery
	factories["rule_evaluator"] = newRuleEvaluator
	factories["http_caller"] = newHTTPCaller
	return factories
}

func (r *BuiltinComponentRegistry) Resolve(ref string) (ComponentDescriptor, error) {
	desc, ok := r.descriptors[ref]
	if !ok {
		return ComponentDescriptor{}, fmt.Errorf("unknown component ref %q", ref)
	}
	return desc, nil
}

func (r *BuiltinComponentRegistry) Instantiate(id string, ref string, config map[string]any) (Component, error) {
	desc, err := r.Resolve(ref)
	if err != nil {
		return nil, err
	}
	if config == nil {
		config = map[string]any{}
	}
	if appErr := shared.ValidatePrimitiveMap(desc.ConfigSchema, config, "components."+id+".config"); appErr != nil {
		return nil, appErr
	}
	factoryName := desc.Factory
	if factoryName == "" {
		factoryName = ref
	}
	factory, ok := r.factories[factoryName]
	if !ok {
		factory, ok = r.factories[ref]
		if !ok {
			return nil, fmt.Errorf("component ref %q has no factory", ref)
		}
	}
	return factory(id, config), nil
}

type baseComponent struct {
	id       string
	category ComponentCategory
	config   map[string]any
}

func (c baseComponent) ID() string {
	return c.id
}

func (c baseComponent) Category() ComponentCategory {
	return c.category
}

func intConfig(config map[string]any, key string, fallback int) int {
	value, ok := config[key]
	if !ok {
		return fallback
	}
	f, ok := shared.ToFloat64(value)
	if !ok {
		return fallback
	}
	return int(f)
}

func boolConfig(config map[string]any, key string, fallback bool) bool {
	value, ok := config[key]
	if !ok {
		return fallback
	}
	v, ok := value.(bool)
	if !ok {
		return fallback
	}
	return v
}

func stringSliceConfig(config map[string]any, key string) []string {
	value, ok := config[key]
	if !ok {
		return nil
	}
	switch v := value.(type) {
	case []string:
		return v
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func includesAnyFold(s string, needles []string) bool {
	lower := strings.ToLower(s)
	for _, needle := range needles {
		if strings.Contains(lower, strings.ToLower(needle)) {
			return true
		}
	}
	return false
}

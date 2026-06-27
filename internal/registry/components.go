package registry

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
)

type intentClassifier struct {
	baseComponent
	intents []string
}

func newIntentClassifier(id string, cfg map[string]any) Component {
	return &intentClassifier{
		baseComponent: baseComponent{id: id, category: CategoryProcessor, config: cfg},
		intents:       stringSliceConfig(cfg, "intents"),
	}
}

func (c *intentClassifier) Run(ctx context.Context, input map[string]any, runtime RuntimeContext) (map[string]any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	message, _ := input["message"].(string)
	intent := "troubleshooting"
	confidence := 0.91
	lower := strings.ToLower(message)
	switch {
	case includesAnyFold(lower, []string{"人工", "human", "handoff", "客服", "投诉升级"}):
		intent = "human_handoff"
		confidence = 0.92
	case includesAnyFold(lower, []string{"投诉", "complaint", "呕吐", "中毒", "玻璃", "医疗"}):
		intent = "complaint"
		confidence = 0.88
	case includesAnyFold(lower, []string{"保修", "warranty", "退换", "policy"}):
		intent = "warranty"
		confidence = 0.86
	case strings.TrimSpace(message) == "":
		intent = "human_handoff"
		confidence = 0.2
	}
	if len(c.intents) > 0 && !contains(c.intents, intent) {
		intent = c.intents[0]
		confidence = 0.7
	}
	return map[string]any{"intent": intent, "confidence": confidence}, nil
}

type severityChecker struct {
	baseComponent
	keywords []string
}

func newSeverityChecker(id string, cfg map[string]any) Component {
	keywords := stringSliceConfig(cfg, "criticalKeywords")
	if len(keywords) == 0 {
		keywords = []string{"呕吐", "医疗", "中毒", "玻璃", "critical", "injury"}
	}
	return &severityChecker{baseComponent: baseComponent{id: id, category: CategoryProcessor, config: cfg}, keywords: keywords}
}

func (c *severityChecker) Run(ctx context.Context, input map[string]any, runtime RuntimeContext) (map[string]any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	message, _ := input["message"].(string)
	intent, _ := input["intent"].(string)
	level := "normal"
	if intent == "complaint" && includesAnyFold(message, c.keywords) {
		level = "critical"
	}
	return map[string]any{"level": level}, nil
}

type keywordRetriever struct {
	baseComponent
	topK int
}

func newKeywordRetriever(id string, cfg map[string]any) Component {
	return &keywordRetriever{
		baseComponent: baseComponent{id: id, category: CategoryProcessor, config: cfg},
		topK:          intConfig(cfg, "topK", 5),
	}
}

func (c *keywordRetriever) Run(ctx context.Context, input map[string]any, runtime RuntimeContext) (map[string]any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	query, _ := input["query"].(string)
	result, err := runtime.Knowledge().Retrieve(ctx, query, c.topK)
	if err != nil {
		return nil, err
	}
	citations := make([]any, 0, len(result.Citations))
	for _, citation := range result.Citations {
		citations = append(citations, map[string]any{"source": citation.Source, "ref": citation.Ref})
	}
	passages := make([]any, 0, len(result.Passages))
	for _, passage := range result.Passages {
		passages = append(passages, passage)
	}
	return map[string]any{"passages": passages, "citations": citations}, nil
}

type citedAnswerAgent struct {
	baseComponent
	requireGrounding bool
}

func newCitedAnswerAgent(id string, cfg map[string]any) Component {
	return &citedAnswerAgent{
		baseComponent:    baseComponent{id: id, category: CategoryProcessor, config: cfg},
		requireGrounding: boolConfig(cfg, "requireGrounding", true),
	}
}

func (c *citedAnswerAgent) Run(ctx context.Context, input map[string]any, runtime RuntimeContext) (map[string]any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	passages := anySlice(input["passages"])
	citations := anySlice(input["citations"])
	message, _ := input["message"].(string)
	if len(passages) == 0 {
		if c.requireGrounding {
			return map[string]any{
				"answer":    "当前知识库为空或未检索到相关知识，请联系人工客服。",
				"citations": []any{},
			}, nil
		}
		return map[string]any{"answer": "未找到相关知识。", "citations": []any{}}, nil
	}
	answer := synthesizeAnswer(message, passages, runtime.Actions())
	return map[string]any{"answer": answer, "citations": citations}, nil
}

type humanHandoff struct {
	baseComponent
	queue string
}

func newHumanHandoff(id string, cfg map[string]any) Component {
	queue, _ := cfg["queue"].(string)
	return &humanHandoff{baseComponent: baseComponent{id: id, category: CategoryAction, config: cfg}, queue: queue}
}

func (c *humanHandoff) Run(ctx context.Context, input map[string]any, runtime RuntimeContext) (map[string]any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	out := map[string]any{
		"status": "created",
		"queue":  c.queue,
	}
	if message, ok := input["message"].(string); ok {
		out["message"] = message
	}
	if runtime.Error() != nil {
		out["reason"] = map[string]any{
			"failedNode": runtime.Error().FailedNode,
			"type":       runtime.Error().Type,
			"message":    runtime.Error().Message,
			"attempts":   runtime.Error().Attempts,
		}
	}
	return out, nil
}

var ticketCounter atomic.Int64

type mockCreateServiceTicket struct {
	baseComponent
	simulateFailure bool
}

func newMockCreateServiceTicket(id string, cfg map[string]any) Component {
	return &mockCreateServiceTicket{
		baseComponent:   baseComponent{id: id, category: CategoryAction, config: cfg},
		simulateFailure: boolConfig(cfg, "simulateFailure", false),
	}
}

func (c *mockCreateServiceTicket) Run(ctx context.Context, input map[string]any, runtime RuntimeContext) (map[string]any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if c.simulateFailure {
		return map[string]any{
			"status": "failed",
			"error":  map[string]any{"code": "MOCK_TICKET_SYSTEM_UNAVAILABLE", "message": "mock ticket system is unavailable"},
		}, nil
	}
	id := ticketCounter.Add(1) + 20000
	return map[string]any{
		"status":   "created",
		"ticketId": fmt.Sprintf("mock-T-%d", id),
		"system":   c.config["system"],
	}, nil
}

func contains(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

func anySlice(value any) []any {
	switch v := value.(type) {
	case []any:
		return v
	case []string:
		out := make([]any, 0, len(v))
		for _, item := range v {
			out = append(out, item)
		}
		return out
	default:
		return nil
	}
}

func synthesizeAnswer(message string, passages []any, actions []ActionSummary) string {
	var createdTicket string
	for _, action := range actions {
		if id, ok := action.Output["ticketId"].(string); ok && id != "" {
			createdTicket = id
		}
	}
	first := strings.TrimSpace(fmt.Sprint(passages[0]))
	if first == "" {
		first = "请根据产品手册处理该问题。"
	}
	if createdTicket != "" {
		return fmt.Sprintf("%s 已为您创建工单 %s，专员将联系您。", first, createdTicket)
	}
	return first
}

package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"fde-support/internal/shared"
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

// --- M2 Phase 2 builtin components ---

// llmExtractor extracts structured data from text using the model gateway.
type llmExtractor struct {
	baseComponent
	schema map[string]string
}

func newLLMExtractor(id string, cfg map[string]any) Component {
	schema := map[string]string{}
	if raw, ok := cfg["schema"].(map[string]any); ok {
		for k, v := range raw {
			if s, ok := v.(string); ok {
				schema[k] = s
			}
		}
	}
	return &llmExtractor{
		baseComponent: baseComponent{id: id, category: CategoryProcessor, config: cfg},
		schema:        schema,
	}
}

func (c *llmExtractor) Run(ctx context.Context, input map[string]any, runtime RuntimeContext) (map[string]any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	text, _ := input["text"].(string)
	if text == "" {
		text, _ = input["message"].(string)
	}
	model := runtime.Model()
	if model == nil {
		return nil, fmt.Errorf("model gateway not available")
	}
	resp, err := model.Generate(ctx, ModelGenerateRequest{
		Messages: []ModelMessage{{Role: "user", Content: text}},
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"extracted": resp.Content}, nil
}

// dataQuery queries tabular data sources using SQL-like syntax.
type dataQuery struct {
	baseComponent
	source string
	query  string
}

func newDataQuery(id string, cfg map[string]any) Component {
	source, _ := cfg["source"].(string)
	query, _ := cfg["query"].(string)
	return &dataQuery{
		baseComponent: baseComponent{id: id, category: CategoryProcessor, config: cfg},
		source:        source,
		query:         query,
	}
}

func (c *dataQuery) Run(ctx context.Context, input map[string]any, runtime RuntimeContext) (map[string]any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	query := c.query
	if query == "" {
		query, _ = input["query"].(string)
	}
	result, err := runtime.Knowledge().Retrieve(ctx, query, 10)
	if err != nil {
		return nil, err
	}
	citations := make([]any, 0, len(result.Citations))
	for _, citation := range result.Citations {
		citations = append(citations, map[string]any{"source": citation.Source, "ref": citation.Ref})
	}
	return map[string]any{"rows": result.Raw, "count": len(result.Raw), "citations": citations}, nil
}

// ruleEvaluator evaluates a list of rules against the input.
type ruleEvaluator struct {
	baseComponent
	rules []ruleSpec
}

type ruleSpec struct {
	ID       string
	Field    string
	Operator string
	Value    any
	Result   string
	Priority int
}

func newRuleEvaluator(id string, cfg map[string]any) Component {
	var rules []ruleSpec
	if raw, ok := cfg["rules"].([]any); ok {
		for _, item := range raw {
			if m, ok := item.(map[string]any); ok {
				r := ruleSpec{}
				if v, ok := m["id"].(string); ok {
					r.ID = v
				}
				if v, ok := m["field"].(string); ok {
					r.Field = v
				}
				if v, ok := m["operator"].(string); ok {
					r.Operator = v
				}
				r.Value = m["value"]
				if v, ok := m["result"].(string); ok {
					r.Result = v
				}
				if v, ok := m["priority"]; ok {
					f, _ := shared.ToFloat64(v)
					r.Priority = int(f)
				}
				rules = append(rules, r)
			}
		}
	}
	return &ruleEvaluator{
		baseComponent: baseComponent{id: id, category: CategoryProcessor, config: cfg},
		rules:         rules,
	}
}

func (c *ruleEvaluator) Run(ctx context.Context, input map[string]any, runtime RuntimeContext) (map[string]any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	matched := []map[string]any{}
	for _, rule := range c.rules {
		fieldValue, ok := input[rule.Field]
		if !ok {
			continue
		}
		match := false
		switch rule.Operator {
		case "eq", "==", "equals":
			match = fmt.Sprint(fieldValue) == fmt.Sprint(rule.Value)
		case "neq", "!=":
			match = fmt.Sprint(fieldValue) != fmt.Sprint(rule.Value)
		case "contains":
			match = strings.Contains(strings.ToLower(fmt.Sprint(fieldValue)), strings.ToLower(fmt.Sprint(rule.Value)))
		case "gt", ">":
			fv, _ := shared.ToFloat64(fieldValue)
			rv, _ := shared.ToFloat64(rule.Value)
			match = fv > rv
		case "lt", "<":
			fv, _ := shared.ToFloat64(fieldValue)
			rv, _ := shared.ToFloat64(rule.Value)
			match = fv < rv
		default:
			match = fmt.Sprint(fieldValue) == fmt.Sprint(rule.Value)
		}
		if match {
			matched = append(matched, map[string]any{"rule": rule.ID, "result": rule.Result, "priority": rule.Priority})
		}
	}
	if len(matched) == 0 {
		return map[string]any{"matched": false}, nil
	}
	best := matched[0]
	for _, m := range matched[1:] {
		if p, _ := m["priority"].(int); p > best["priority"].(int) {
			best = m
		}
	}
	return map[string]any{"matched": true, "rule": best["rule"], "result": best["result"], "matches": matched}, nil
}

// httpCaller calls external HTTP APIs.
type httpCaller struct {
	baseComponent
	urlTemplate    string
	method         string
	headers        map[string]string
	bodyTemplate   string
	timeoutMs      int
	continueOnFail bool
}

func newHTTPCaller(id string, cfg map[string]any) Component {
	url, _ := cfg["url"].(string)
	method, _ := cfg["method"].(string)
	if method == "" {
		method = "POST"
	}
	timeoutMs := intConfig(cfg, "timeoutMs", 5000)
	continueOnFail := boolConfig(cfg, "continueOnFailure", false)
	headers := map[string]string{}
	if raw, ok := cfg["headers"].(map[string]any); ok {
		for k, v := range raw {
			if s, ok := v.(string); ok {
				headers[k] = s
			}
		}
	}
	bodyTemplate, _ := cfg["bodyTemplate"].(string)
	return &httpCaller{
		baseComponent:  baseComponent{id: id, category: CategoryAction, config: cfg},
		urlTemplate:    url,
		method:         method,
		headers:        headers,
		bodyTemplate:   bodyTemplate,
		timeoutMs:      timeoutMs,
		continueOnFail: continueOnFail,
	}
}

func (c *httpCaller) Run(ctx context.Context, input map[string]any, runtime RuntimeContext) (map[string]any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	timeout := time.Duration(c.timeoutMs) * time.Millisecond
	if c.timeoutMs <= 0 {
		timeout = 5 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	url := c.urlTemplate
	for k, v := range input {
		url = strings.ReplaceAll(url, "{{"+k+"}}", fmt.Sprint(v))
	}
	body := c.bodyTemplate
	for k, v := range input {
		body = strings.ReplaceAll(body, "{{"+k+"}}", fmt.Sprint(v))
	}

	caller := runtime.HTTP()
	if caller == nil {
		return nil, fmt.Errorf("http.call capability is not available")
	}
	resp, err := caller.Call(ctx, HTTPCallRequest{
		URL:     url,
		Method:  c.method,
		Headers: c.headers,
		Body:    body,
	})
	if err != nil {
		return map[string]any{"status": "failed", "error": map[string]any{"code": "HTTP_REQUEST_FAILED", "message": err.Error()}}, nil
	}

	var parsed any
	if err := json.Unmarshal([]byte(resp.Body), &parsed); err != nil {
		parsed = resp.Body
	}
	return map[string]any{"status": "ok", "statusCode": float64(resp.StatusCode), "body": parsed}, nil
}

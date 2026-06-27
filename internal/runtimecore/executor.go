package runtimecore

import (
	"context"
	"fmt"
	"time"

	"fde-support/internal/environment"
	"fde-support/internal/manifest"
	"fde-support/internal/registry"
	"fde-support/internal/shared"
	"fde-support/internal/trace"
	"fde-support/internal/workflow"
)

type Executor struct {
	manifest    *manifest.SolutionManifest
	plan        *workflow.Plan
	env         environment.ResolvedEnvironment
	registry    registry.ComponentRegistry
	knowledge   registry.KnowledgeReader
	traceWriter *trace.FileTraceWriter
	components  map[string]registry.Component
	descriptors map[string]registry.ComponentDescriptor
	specs       map[string]manifest.ComponentSpec
}

func NewExecutor(m *manifest.SolutionManifest, env environment.ResolvedEnvironment, reg registry.ComponentRegistry, knowledge registry.KnowledgeReader, traceWriter *trace.FileTraceWriter) (*Executor, error) {
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
	plan, issues := workflow.CompileWorkflow(nodeSpecs)
	if len(issues) > 0 {
		return nil, fmt.Errorf("workflow compilation failed: %v", issues)
	}
	ex := &Executor{
		manifest:    m,
		plan:        plan,
		env:         env,
		registry:    reg,
		knowledge:   knowledge,
		traceWriter: traceWriter,
		components:  map[string]registry.Component{},
		descriptors: map[string]registry.ComponentDescriptor{},
		specs:       map[string]manifest.ComponentSpec{},
	}
	for _, spec := range m.Components {
		component, err := reg.Instantiate(spec.ID, spec.Ref, spec.Config)
		if err != nil {
			return nil, err
		}
		desc, err := reg.Resolve(spec.Ref)
		if err != nil {
			return nil, err
		}
		ex.components[spec.ID] = component
		ex.descriptors[spec.ID] = desc
		ex.specs[spec.ID] = spec
	}
	return ex, nil
}

func (e *Executor) Execute(ctx context.Context, req RuntimeRequest) (map[string]any, *trace.TraceRecord, error) {
	start := time.Now()
	inputs, mapErr := e.ApplyInputMapping(req)
	record, err := e.traceWriter.Start(ctx, trace.TraceRecord{
		Solution:    e.manifest.Metadata.Name,
		Version:     e.manifest.Metadata.Version,
		Environment: e.env.EnvironmentName,
		Trigger: trace.TriggerSpec{
			Type:       req.Trigger.Type,
			Sensor:     req.Trigger.Sensor,
			SignalType: req.Trigger.SignalType,
		},
		Input: inputs,
	})
	if err != nil {
		return nil, nil, err
	}
	if mapErr != nil {
		errSummary := toRuntimeError("input_mapping_error", "", mapErr, 0)
		finished, finishErr := e.traceWriter.Finish(ctx, record.TraceID, "failed", errSummary, time.Since(start))
		return nil, finished, coalesceErr(mapErr, finishErr)
	}

	retryCount := e.manifest.Workflow.OnError.Retry
	if retryCount < 0 {
		retryCount = 0
	}

	exec := &workflowExecution{
		executor:   e,
		traceID:    record.TraceID,
		ctx:        ctx,
		inputs:     inputs,
		request:    req,
		retryCount: retryCount,
		outputs:    map[string]map[string]any{},
	}
	if runErr := exec.run(); runErr != nil {
		errSummary := toRuntimeError(runErr.errType, runErr.failedNode, runErr.err, runErr.attempts)
		finished, finishErr := e.traceWriter.Finish(ctx, record.TraceID, "failed", errSummary, time.Since(start))
		return nil, finished, coalesceErr(runErr.err, finishErr)
	}
	response := e.mapResponse(record.TraceID, exec.outputs, exec.actions, exec.firstIntent, exec.lastAnswerNode, exec.lastRetrieverNode)
	finished, finishErr := e.traceWriter.Finish(ctx, record.TraceID, "success", nil, time.Since(start))
	if finishErr != nil {
		return nil, finished, finishErr
	}
	return response, finished, nil
}

func (e *Executor) ApplyInputMapping(req RuntimeRequest) (map[string]any, error) {
	mapping := e.manifest.Workflow.InputMapping[req.Trigger.Type]
	if len(mapping) == 0 {
		root := requestRoot(req)
		return root, nil
	}
	root := requestRoot(req)
	out := map[string]any{}
	for target, path := range mapping {
		value, ok := workflow.ResolvePath(root, path)
		if !ok {
			return out, shared.BadRequest("INPUT_MAPPING_MISSING", "workflow.inputMapping."+req.Trigger.Type+"."+target, "required mapping path is missing: "+path)
		}
		out[target] = value
	}
	return out, nil
}

func (e *Executor) executeNodeWithRetry(ctx context.Context, traceID string, node workflow.CompiledNode, component registry.Component, inputs map[string]any, outputs map[string]map[string]any, req RuntimeRequest, actions []registry.ActionSummary, errSummary *registry.RuntimeErrorSummary, retries int) (map[string]any, int, error) {
	var lastErr error
	for attempt := 1; attempt <= retries+1; attempt++ {
		nodeStart := time.Now()
		nodeInput, err := buildNodeInput(node, inputs, outputs)
		if err != nil {
			lastErr = err
			_ = e.traceWriter.AppendSpan(ctx, traceID, trace.TraceSpan{Node: node.ID, Component: node.Component, Attempt: attempt, LatencyMS: time.Since(nodeStart).Milliseconds(), Error: toRuntimeError("input_mapping_error", node.ID, err, attempt)})
			continue
		}
		if appErr := shared.ValidatePrimitiveMap(e.descriptors[node.Component].InputSchema, nodeInput, "workflow.nodes."+node.ID+".inputs"); appErr != nil {
			lastErr = appErr
			_ = e.traceWriter.AppendSpan(ctx, traceID, trace.TraceSpan{Node: node.ID, Component: node.Component, Attempt: attempt, Input: nodeInput, LatencyMS: time.Since(nodeStart).Milliseconds(), Error: toRuntimeError("input_type_mismatch", node.ID, appErr, attempt)})
			continue
		}
		nodeCtx, cancel := context.WithTimeout(ctx, time.Duration(e.env.MaxLatencyMs)*time.Millisecond)
		output, err := component.Run(nodeCtx, nodeInput, runtimeContext{environment: e.env.EnvironmentName, knowledge: e.knowledge, request: req, errSummary: errSummary, actions: actions})
		cancel()
		if err != nil {
			lastErr = err
			_ = e.traceWriter.AppendSpan(ctx, traceID, trace.TraceSpan{Node: node.ID, Component: node.Component, Attempt: attempt, Input: nodeInput, LatencyMS: time.Since(nodeStart).Milliseconds(), Error: toRuntimeError("component_error", node.ID, err, attempt)})
			continue
		}
		if status, _ := output["status"].(string); status == "error" || status == "failed" {
			componentSpec := e.specs[node.Component]
			hard := componentSpec.Category == string(registry.CategoryProcessor) || status == "error" || !node.ContinueOnFailure
			if hard {
				lastErr = fmt.Errorf("component returned status %q", status)
				_ = e.traceWriter.AppendSpan(ctx, traceID, trace.TraceSpan{Node: node.ID, Component: node.Component, Attempt: attempt, Input: nodeInput, Output: output, LatencyMS: time.Since(nodeStart).Milliseconds(), Error: toRuntimeError("component_failed_status", node.ID, lastErr, attempt)})
				continue
			}
		}
		_ = e.traceWriter.AppendSpan(ctx, traceID, trace.TraceSpan{Node: node.ID, Component: node.Component, Attempt: attempt, Input: nodeInput, Output: output, LatencyMS: time.Since(nodeStart).Milliseconds()})
		return output, attempt, nil
	}
	return nil, retries + 1, lastErr
}

func (e *Executor) executeFallback(ctx context.Context, traceID string, failedNode string, failedErr error, attempts int, inputs map[string]any, outputs map[string]map[string]any, req RuntimeRequest, actions []registry.ActionSummary) (map[string]any, error) {
	fallbackID := e.manifest.Workflow.OnError.FallbackNode
	if fallbackID == "" {
		return nil, failedErr
	}
	fallback, ok := e.plan.Node(fallbackID)
	if !ok {
		return nil, failedErr
	}
	errSummary := toRuntimeError("component_error", failedNode, failedErr, attempts)
	component := e.components[fallback.Component]
	output, _, err := e.executeNodeWithRetry(ctx, traceID, fallback, component, inputs, outputs, req, actions, errSummary, 0)
	return output, err
}

func buildNodeInput(node workflow.CompiledNode, inputs map[string]any, outputs map[string]map[string]any) (map[string]any, error) {
	if len(node.Inputs) == 0 {
		return copyMap(inputs), nil
	}
	root := workflow.NodeInputRoots(inputs, outputs)
	out := map[string]any{}
	for key, path := range node.Inputs {
		value, ok := workflow.ResolvePath(root, path)
		if !ok {
			return nil, shared.BadRequest("NODE_INPUT_MAPPING_MISSING", "workflow.nodes."+node.ID+".inputs."+key, "required node input path is missing: "+path)
		}
		out[key] = value
	}
	return out, nil
}

func (e *Executor) mapResponse(traceID string, outputs map[string]map[string]any, actions []registry.ActionSummary, firstIntent map[string]any, lastAnswerNode string, lastRetrieverNode string) map[string]any {
	response := map[string]any{"traceId": traceID}
	if firstIntent != nil {
		response["intent"] = firstIntent["intent"]
		response["confidence"] = firstIntent["confidence"]
	}
	if lastAnswerNode != "" {
		answerOutput := outputs[lastAnswerNode]
		response["answer"] = answerOutput["answer"]
		if citations, ok := answerOutput["citations"]; ok {
			response["citations"] = citations
		}
	}
	if _, ok := response["citations"]; !ok && lastRetrieverNode != "" {
		response["citations"] = outputs[lastRetrieverNode]["citations"]
	}
	if _, ok := response["answer"]; !ok {
		response["answer"] = "当前知识库为空或未检索到相关知识，请联系人工客服。"
	}
	if _, ok := response["citations"]; !ok {
		response["citations"] = []any{}
	}
	if len(actions) > 0 {
		list := make([]any, 0, len(actions))
		for _, action := range actions {
			list = append(list, map[string]any{"node": action.Node, "output": action.Output})
		}
		response["actions"] = list
	}
	return response
}

func requestRoot(req RuntimeRequest) map[string]any {
	root := map[string]any{
		"trigger": map[string]any{
			"type":       req.Trigger.Type,
			"sensor":     req.Trigger.Sensor,
			"signalType": req.Trigger.SignalType,
		},
	}
	if req.Request != nil {
		root["request"] = req.Request
	}
	if req.Signal != nil {
		root["signal"] = req.Signal
	}
	if req.RawPayload != nil {
		root["raw_payload"] = req.RawPayload
	}
	return root
}

func copyMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func toRuntimeError(errType, failedNode string, err error, attempts int) *registry.RuntimeErrorSummary {
	if err == nil {
		return nil
	}
	return &registry.RuntimeErrorSummary{
		FailedNode: failedNode,
		Message:    err.Error(),
		Type:       errType,
		Attempts:   attempts,
	}
}

func coalesceErr(primary error, secondary error) error {
	if primary != nil {
		return primary
	}
	return secondary
}

type workflowExecution struct {
	executor          *Executor
	traceID           string
	ctx               context.Context
	inputs            map[string]any
	request           RuntimeRequest
	retryCount        int
	outputs           map[string]map[string]any
	actions           []registry.ActionSummary
	firstIntent       map[string]any
	lastAnswerNode    string
	lastRetrieverNode string
}

type workflowRunError struct {
	err        error
	errType    string
	failedNode string
	attempts   int
}

func (e *workflowRunError) Error() string {
	if e == nil || e.err == nil {
		return ""
	}
	return e.err.Error()
}

func (x *workflowExecution) run() *workflowRunError {
	for _, node := range x.executor.plan.Nodes {
		component, err := x.executor.componentFor(node)
		if err != nil {
			return &workflowRunError{err: err, errType: "component_error", failedNode: node.ID}
		}
		if node.WhenCondition != nil {
			ok, err := node.WhenCondition.Evaluate(x.outputs)
			if err != nil {
				return &workflowRunError{err: err, errType: "condition_error", failedNode: node.ID}
			}
			if !ok {
				_ = x.executor.traceWriter.AppendSpan(x.ctx, x.traceID, trace.TraceSpan{Node: node.ID, Component: node.Component, Skipped: true})
				continue
			}
		}
		output, attempts, hardErr := x.executor.executeNodeWithRetry(x.ctx, x.traceID, node, component, x.inputs, x.outputs, x.request, x.actions, nil, x.retryCount)
		if hardErr != nil {
			fallbackOutput, fallbackErr := x.executor.executeFallback(x.ctx, x.traceID, node.ID, hardErr, attempts, x.inputs, x.outputs, x.request, x.actions)
			if fallbackErr != nil {
				return &workflowRunError{err: fallbackErr, errType: "component_error", failedNode: node.ID, attempts: attempts}
			}
			if fallbackOutput != nil {
				fallbackNode := x.executor.manifest.Workflow.OnError.FallbackNode
				x.outputs[fallbackNode] = fallbackOutput
				x.actions = append(x.actions, registry.ActionSummary{Node: fallbackNode, Output: fallbackOutput})
			}
			break
		}
		x.outputs[node.ID] = output
		x.collect(node, output)
	}
	return nil
}

func (x *workflowExecution) collect(node workflow.CompiledNode, output map[string]any) {
	spec := x.executor.specs[node.Component]
	if spec.Category == string(registry.CategoryAction) {
		x.actions = append(x.actions, registry.ActionSummary{Node: node.ID, Output: output})
	}
	if x.firstIntent == nil {
		if _, hasIntent := output["intent"]; hasIntent {
			if _, hasConfidence := output["confidence"]; hasConfidence {
				x.firstIntent = output
			}
		}
	}
	if _, ok := output["passages"]; ok {
		if _, hasCitations := output["citations"]; hasCitations {
			x.lastRetrieverNode = node.ID
		}
	}
	if _, ok := output["answer"]; ok {
		x.lastAnswerNode = node.ID
	}
}

func (e *Executor) componentFor(node workflow.CompiledNode) (registry.Component, error) {
	component := e.components[node.Component]
	if component == nil {
		return nil, shared.Internal("UNKNOWN_COMPONENT", "workflow.nodes."+node.ID+".component", "component is not instantiated")
	}
	return component, nil
}

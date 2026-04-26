package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

type toolRunner struct {
	tools  map[string]kit.Tool
	hooks  *hooks
	config *Config

	window [][]string // sliding window of turn batches for loop detection
}

func newToolRunner(tools map[string]kit.Tool, hooks *hooks, config *Config) *toolRunner {
	return &toolRunner{
		tools:  tools,
		hooks:  hooks,
		config: config,
	}
}

func (r *toolRunner) run(
	ctx context.Context,
	toolCalls []kit.ToolCall,
	yield func(kit.Event, error) bool,
) ([]kit.Message, bool) {
	if err := r.checkLoops(toolCalls); err != nil {
		yield(kit.Event{}, err)

		return nil, false
	}

	// TODO: it is begging for a strategy
	if r.config.ToolExecution == ToolExecutionSequential {
		return r.runSequential(ctx, toolCalls, yield)
	}

	return r.runParallel(ctx, toolCalls, yield)
}

func (r *toolRunner) runSequential(
	ctx context.Context,
	toolCalls []kit.ToolCall,
	yield func(kit.Event, error) bool,
) ([]kit.Message, bool) {
	msgs := make([]kit.Message, 0, len(toolCalls))

	for _, toolCall := range toolCalls {
		result := r.call(ctx, toolCall)
		part := kit.NewToolResultPart(result.Content, result.Call)

		if !yield(kit.NewContentPartStartedEvent(kit.ContentTypeToolResult), nil) {
			return msgs, false
		}

		if !yield(kit.NewContentPartDeltaEvent(kit.ContentTypeToolResult, result.Content), nil) {
			return msgs, false
		}

		if !yield(kit.NewContentPartDoneEvent(part), nil) {
			return msgs, false
		}

		toolMsg := kit.NewToolMessage(result.Content, result.Call)

		if !yield(kit.NewMessageEvent(toolMsg), nil) {
			return msgs, false
		}

		msgs = append(msgs, toolMsg)
	}

	return msgs, true
}

func (r *toolRunner) runParallel(
	ctx context.Context,
	toolCalls []kit.ToolCall,
	yield func(kit.Event, error) bool,
) ([]kit.Message, bool) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	msgs := make([]kit.Message, len(toolCalls))

	type indexedResult struct {
		index  int
		result kit.ToolResult
	}

	results := make(chan indexedResult, len(toolCalls))

	for i, toolCall := range toolCalls {
		go func(index int, call kit.ToolCall) {
			results <- indexedResult{
				index:  index,
				result: r.call(ctx, call),
			}
		}(i, toolCall)
	}

	for range len(toolCalls) {
		var result indexedResult

		select {
		case result = <-results:
		case <-ctx.Done():
			return msgs, false
		}

		part := kit.NewToolResultPart(result.result.Content, result.result.Call)

		if !yield(kit.NewContentPartStartedEvent(kit.ContentTypeToolResult), nil) {
			return msgs, false
		}

		if !yield(kit.NewContentPartDeltaEvent(kit.ContentTypeToolResult, result.result.Content), nil) {
			return msgs, false
		}

		if !yield(kit.NewContentPartDoneEvent(part), nil) {
			return msgs, false
		}

		toolMsg := kit.NewToolMessage(result.result.Content, result.result.Call)

		if !yield(kit.NewMessageEvent(toolMsg), nil) {
			return msgs, false
		}

		msgs[result.index] = toolMsg
	}

	return msgs, true
}

func (r *toolRunner) call(ctx context.Context, toolCall kit.ToolCall) (result kit.ToolResult) {
	defer func() {
		if recovered := recover(); recovered != nil {
			result = kit.NewErrorToolResult(toolCall, fmt.Errorf("recovered: %v", recovered))
		}
	}()

	ctx, toolCall, err := r.hooks.onToolCall(ctx, toolCall)
	if err != nil {
		return kit.NewErrorToolResult(toolCall, err)
	}

	tool, ok := r.tools[toolCall.Name]
	if !ok {
		return kit.NewErrorToolResult(toolCall, fmt.Errorf("tool not found: %s", toolCall.Name))
	}

	content, err := tool.Execute(ctx, toolCall.Arguments)
	if err != nil {
		return kit.NewErrorToolResult(toolCall, err)
	}

	result = kit.ToolResult{Call: toolCall, Content: content}

	_, result, err = r.hooks.onToolResult(ctx, result)
	if err != nil {
		return kit.NewErrorToolResult(toolCall, err)
	}

	return result
}

func (r *toolRunner) checkLoops(calls []kit.ToolCall) error {
	if r.config.ToolLoopMaxRepetitions <= 0 {
		return nil
	}

	batch := make([]string, len(calls))
	for i, call := range calls {
		batch[i] = loopKey(call)
	}

	r.window = append(r.window, batch)
	if len(r.window) > r.config.ToolLoopWindow {
		r.window = r.window[len(r.window)-r.config.ToolLoopWindow:]
	}

	counts := make(map[string]int)
	for _, turn := range r.window {
		for _, k := range turn {
			counts[k]++
		}
	}

	for _, call := range calls {
		if n := counts[loopKey(call)]; n > r.config.ToolLoopMaxRepetitions {
			return fmt.Errorf("%w: %s called %d times within the last %d turns",
				kit.ErrToolLoop, call.Name, n, r.config.ToolLoopWindow)
		}
	}

	return nil
}

func loopKey(call kit.ToolCall) string {
	b, _ := json.Marshal(call.Arguments)

	return call.Name + "-" + string(b)
}

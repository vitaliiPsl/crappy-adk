package agent

import (
	"context"
	"fmt"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

type toolRunner struct {
	tools  map[string]kit.Tool
	hooks  *hooks
	config *Config
}

func (r *toolRunner) run(
	ctx context.Context,
	toolCalls []kit.ToolCall,
	yield func(kit.Event, error) bool,
) ([]kit.Message, bool) {
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

		if !yield(kit.NewToolResultEvent(result), nil) {
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

	msgs := make([]kit.Message, 0, len(toolCalls))

	results := make(chan kit.ToolResult, len(toolCalls))

	for _, toolCall := range toolCalls {
		go func() {
			results <- r.call(ctx, toolCall)
		}()
	}

	for range len(toolCalls) {
		var result kit.ToolResult

		select {
		case result = <-results:
		case <-ctx.Done():
			return msgs, false
		}

		if !yield(kit.NewToolResultEvent(result), nil) {
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

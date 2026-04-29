package agent

import (
	"context"
	"fmt"

	"github.com/vitaliiPsl/crappy-adk/kit"
	"github.com/vitaliiPsl/crappy-adk/x/stream"
)

type toolRunner struct {
	tools  map[string]kit.Tool
	hooks  *hooks
	config *Config
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
	e *stream.Emitter[kit.Event],
) ([]kit.Message, error) {
	// TODO: it is begging for a strategy
	if r.config.ToolExecution == ToolExecutionSequential {
		return r.runSequential(ctx, toolCalls, e)
	}

	return r.runParallel(ctx, toolCalls, e)
}

func (r *toolRunner) runSequential(
	ctx context.Context,
	toolCalls []kit.ToolCall,
	e *stream.Emitter[kit.Event],
) ([]kit.Message, error) {
	msgs := make([]kit.Message, 0, len(toolCalls))

	for _, toolCall := range toolCalls {
		result := r.call(ctx, toolCall)
		part := kit.NewToolResultPart(result.Content, result.Call)

		if err := e.Emit(kit.NewContentPartStartedEvent(kit.ContentTypeToolResult)); err != nil {
			return msgs, err
		}

		if err := e.Emit(kit.NewContentPartDeltaEvent(kit.ContentTypeToolResult, result.Content)); err != nil {
			return msgs, err
		}

		if err := e.Emit(kit.NewContentPartDoneEvent(part)); err != nil {
			return msgs, err
		}

		toolMsg := kit.NewToolMessage(result.Content, result.Call)

		if err := e.Emit(kit.NewMessageEvent(toolMsg)); err != nil {
			return msgs, err
		}

		msgs = append(msgs, toolMsg)
	}

	return msgs, nil
}

func (r *toolRunner) runParallel(
	ctx context.Context,
	toolCalls []kit.ToolCall,
	e *stream.Emitter[kit.Event],
) ([]kit.Message, error) {
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
			return msgs, ctx.Err()
		}

		part := kit.NewToolResultPart(result.result.Content, result.result.Call)

		if err := e.Emit(kit.NewContentPartStartedEvent(kit.ContentTypeToolResult)); err != nil {
			return msgs, err
		}

		if err := e.Emit(kit.NewContentPartDeltaEvent(kit.ContentTypeToolResult, result.result.Content)); err != nil {
			return msgs, err
		}

		if err := e.Emit(kit.NewContentPartDoneEvent(part)); err != nil {
			return msgs, err
		}

		toolMsg := kit.NewToolMessage(result.result.Content, result.result.Call)

		if err := e.Emit(kit.NewMessageEvent(toolMsg)); err != nil {
			return msgs, err
		}

		msgs[result.index] = toolMsg
	}

	return msgs, nil
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

package agent

import (
	"context"
	"fmt"

	"github.com/vitaliiPsl/crappy-adk/kit"
	"github.com/vitaliiPsl/crappy-adk/x/stream"
)

type toolExecutor struct {
	tools    map[string]kit.Tool
	hooks    *hooks
	strategy toolExecutionStrategy
}

func newToolExecutor(tools map[string]kit.Tool, hooks *hooks, strategy toolExecutionStrategy) *toolExecutor {
	return &toolExecutor{tools: tools, hooks: hooks, strategy: strategy}
}

func (e *toolExecutor) run(
	ctx context.Context,
	calls []kit.ToolCall,
	emitter *stream.Emitter[kit.Event],
) ([]kit.Message, error) {
	emit := func(result kit.ToolResult) (kit.Message, error) {
		return e.emitResult(result, emitter)
	}

	return e.strategy.execute(ctx, calls, e.call, emit)
}

func (e *toolExecutor) call(ctx context.Context, toolCall kit.ToolCall) (result kit.ToolResult) {
	defer func() {
		if recovered := recover(); recovered != nil {
			result = kit.NewErrorToolResult(toolCall, fmt.Errorf("recovered: %v", recovered))
		}
	}()

	ctx, toolCall, err := e.hooks.onToolCall(ctx, toolCall)
	if err != nil {
		return kit.NewErrorToolResult(toolCall, err)
	}

	tool, ok := e.tools[toolCall.Name]
	if !ok {
		return kit.NewErrorToolResult(toolCall, fmt.Errorf("tool not found: %s", toolCall.Name))
	}

	content, err := tool.Execute(ctx, toolCall.Arguments)
	if err != nil {
		return kit.NewErrorToolResult(toolCall, err)
	}

	result = kit.ToolResult{Call: toolCall, Content: content}

	_, result, err = e.hooks.onToolResult(ctx, result)
	if err != nil {
		return kit.NewErrorToolResult(toolCall, err)
	}

	return result
}

func (e *toolExecutor) emitResult(result kit.ToolResult, emitter *stream.Emitter[kit.Event]) (kit.Message, error) {
	part := kit.NewToolResultPart(result.Content, result.Call)

	if err := emitter.Emit(kit.NewContentPartStartedEvent(kit.ContentTypeToolResult)); err != nil {
		return kit.Message{}, err
	}

	if err := emitter.Emit(kit.NewContentPartDoneEvent(part)); err != nil {
		return kit.Message{}, err
	}

	msg := kit.NewToolMessage(result.Content, result.Call)

	if err := emitter.Emit(kit.NewMessageEvent(msg)); err != nil {
		return kit.Message{}, err
	}

	return msg, nil
}

// toolExecutionStrategy controls how a batch of tool calls are dispatched.
type toolExecutionStrategy interface {
	execute(
		ctx context.Context,
		calls []kit.ToolCall,
		call func(context.Context, kit.ToolCall) kit.ToolResult,
		emit func(kit.ToolResult) (kit.Message, error),
	) ([]kit.Message, error)
}

type sequentialStrategy struct{}

func (sequentialStrategy) execute(
	ctx context.Context,
	calls []kit.ToolCall,
	call func(context.Context, kit.ToolCall) kit.ToolResult,
	emit func(kit.ToolResult) (kit.Message, error),
) ([]kit.Message, error) {
	msgs := make([]kit.Message, 0, len(calls))

	for _, c := range calls {
		msg, err := emit(call(ctx, c))
		if err != nil {
			return msgs, err
		}

		msgs = append(msgs, msg)
	}

	return msgs, nil
}

type parallelStrategy struct{}

type indexedResult struct {
	index  int
	result kit.ToolResult
}

func (parallelStrategy) execute(
	ctx context.Context,
	calls []kit.ToolCall,
	call func(context.Context, kit.ToolCall) kit.ToolResult,
	emit func(kit.ToolResult) (kit.Message, error),
) ([]kit.Message, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	msgs := make([]kit.Message, len(calls))
	results := make(chan indexedResult, len(calls))

	for i, c := range calls {
		go func(index int, c kit.ToolCall) {
			results <- indexedResult{index: index, result: call(ctx, c)}
		}(i, c)
	}

	for range len(calls) {
		var ir indexedResult

		select {
		case ir = <-results:
		case <-ctx.Done():
			return msgs, ctx.Err()
		}

		msg, err := emit(ir.result)
		if err != nil {
			return msgs, err
		}

		msgs[ir.index] = msg
	}

	return msgs, nil
}

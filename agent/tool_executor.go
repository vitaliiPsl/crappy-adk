package agent

import (
	"context"
	"fmt"

	"github.com/vitaliiPsl/crappy-adk/kit"
	"github.com/vitaliiPsl/crappy-adk/x/stream"
)

type toolExecutor struct {
	registry *toolRegistry
	hooks    *hooks
}

func newToolExecutor(registry *toolRegistry, hooks *hooks) *toolExecutor {
	return &toolExecutor{registry: registry, hooks: hooks}
}

func (e *toolExecutor) run(
	ctx context.Context,
	calls []kit.ToolCall,
	emitter *stream.Emitter[kit.Event],
) ([]kit.Message, error) {
	msgs := make([]kit.Message, 0, len(calls))

	for _, call := range calls {
		if err := ctx.Err(); err != nil {
			return msgs, err
		}

		result := e.call(ctx, call)

		msg, err := e.emitResult(result, emitter)
		if err != nil {
			return msgs, err
		}

		msgs = append(msgs, msg)
	}

	return msgs, nil
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

	tool, ok := e.registry.get(toolCall.Name)
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

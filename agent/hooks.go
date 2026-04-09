package agent

import (
	"context"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

type hooks struct {
	modelRequest  []kit.OnModelRequest
	modelResponse []kit.OnModelResponse
	toolCall      []kit.OnToolCall
	toolResult    []kit.OnToolResult
	runStart      []kit.OnRunStart
	runEnd        []kit.OnRunEnd
	turnStart     []kit.OnTurnStart
	turnEnd       []kit.OnTurnEnd
}

func (h *hooks) onModelRequest(ctx context.Context, req kit.ModelRequest) (context.Context, kit.ModelRequest, error) {
	for _, fn := range h.modelRequest {
		var err error

		ctx, req, err = fn(ctx, req)
		if err != nil {
			return ctx, kit.ModelRequest{}, err
		}
	}

	return ctx, req, nil
}

func (h *hooks) onModelResponse(ctx context.Context, resp kit.ModelResponse) (context.Context, kit.ModelResponse, error) {
	for _, fn := range h.modelResponse {
		var err error

		ctx, resp, err = fn(ctx, resp)
		if err != nil {
			return ctx, kit.ModelResponse{}, err
		}
	}

	return ctx, resp, nil
}

func (h *hooks) onToolCall(ctx context.Context, call kit.ToolCall) (context.Context, kit.ToolCall, error) {
	for _, fn := range h.toolCall {
		var err error

		ctx, call, err = fn(ctx, call)
		if err != nil {
			return ctx, call, err
		}
	}

	return ctx, call, nil
}

func (h *hooks) onToolResult(ctx context.Context, result kit.ToolResult) (context.Context, kit.ToolResult, error) {
	for _, fn := range h.toolResult {
		var err error

		ctx, result, err = fn(ctx, result)
		if err != nil {
			return ctx, result, err
		}
	}

	return ctx, result, nil
}

func (h *hooks) onRunStart(ctx context.Context, messages []kit.Message) (context.Context, []kit.Message, error) {
	for _, fn := range h.runStart {
		var err error

		ctx, messages, err = fn(ctx, messages)
		if err != nil {
			return ctx, nil, err
		}
	}

	return ctx, messages, nil
}

func (h *hooks) onRunEnd(ctx context.Context, result kit.Result, runErr error) (context.Context, error) {
	for _, fn := range h.runEnd {
		var err error

		ctx, err = fn(ctx, result, runErr)
		if err != nil {
			return ctx, err
		}
	}

	return ctx, nil
}

func (h *hooks) onTurnStart(ctx context.Context, messages []kit.Message) (context.Context, []kit.Message, error) {
	for _, fn := range h.turnStart {
		var err error

		ctx, messages, err = fn(ctx, messages)
		if err != nil {
			return ctx, nil, err
		}
	}

	return ctx, messages, nil
}

func (h *hooks) onTurnEnd(ctx context.Context, messages []kit.Message) (context.Context, error) {
	for _, fn := range h.turnEnd {
		var err error

		ctx, err = fn(ctx, messages)
		if err != nil {
			return ctx, err
		}
	}

	return ctx, nil
}

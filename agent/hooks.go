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
			return ctx, kit.ToolCall{}, err
		}
	}

	return ctx, call, nil
}

func (h *hooks) onToolResult(ctx context.Context, result kit.ToolResult) (context.Context, kit.ToolResult, error) {
	for _, fn := range h.toolResult {
		var err error

		ctx, result, err = fn(ctx, result)
		if err != nil {
			return ctx, kit.ToolResult{}, err
		}
	}

	return ctx, result, nil
}

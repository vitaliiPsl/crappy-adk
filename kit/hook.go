package kit

import "context"

// OnModelRequest is called before each model request. Returning an error cancels the request.
type OnModelRequest func(ctx context.Context, req ModelRequest) error

// OnModelResponse is called after each model response. Returning an error stops the agent.
type OnModelResponse func(ctx context.Context, resp ModelResponse) error

// OnToolCall is called before a tool is executed. Returning an error cancels the tool call.
type OnToolCall func(ctx context.Context, call ToolCall) error

// OnToolResult is called after a tool finishes executing.
type OnToolResult func(ctx context.Context, call ToolCall, result string, err error) error

type hooks struct {
	modelRequest  []OnModelRequest
	modelResponse []OnModelResponse
	toolCall      []OnToolCall
	toolResult    []OnToolResult
}

func (h *hooks) onModelRequest(ctx context.Context, req ModelRequest) error {
	for _, fn := range h.modelRequest {
		if err := fn(ctx, req); err != nil {
			return err
		}
	}

	return nil
}

func (h *hooks) onModelResponse(ctx context.Context, resp ModelResponse) error {
	for _, fn := range h.modelResponse {
		if err := fn(ctx, resp); err != nil {
			return err
		}
	}

	return nil
}

func (h *hooks) onToolCall(ctx context.Context, call ToolCall) error {
	for _, fn := range h.toolCall {
		if err := fn(ctx, call); err != nil {
			return err
		}
	}

	return nil
}

func (h *hooks) onToolResult(ctx context.Context, call ToolCall, result string, err error) error {
	for _, fn := range h.toolResult {
		if err := fn(ctx, call, result, err); err != nil {
			return err
		}
	}

	return nil
}

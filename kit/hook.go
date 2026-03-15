package kit

import "context"

// OnModelRequest is called before each model request.
// The returned context and [ModelRequest] replace the originals for the model call.
// Returning an error cancels the request.
type OnModelRequest func(ctx context.Context, req ModelRequest) (context.Context, ModelRequest, error)

// OnModelResponse is called after each model response.
// The returned context and [ModelResponse] replace the originals for the agent loop.
// Returning an error stops the agent.
type OnModelResponse func(ctx context.Context, resp ModelResponse) (context.Context, ModelResponse, error)

// OnToolCall is called before a tool is executed.
// The returned context and [ToolCall] replace the originals for the tool execution.
// Returning an error cancels the tool call.
type OnToolCall func(ctx context.Context, call ToolCall) (context.Context, ToolCall, error)

// OnToolResult is called after a tool finishes executing.
// The returned context and string replace the originals for the agent loop.
// Returning an error stops the agent.
type OnToolResult func(ctx context.Context, call ToolCall, result string, err error) (context.Context, string, error)

type hooks struct {
	modelRequest  []OnModelRequest
	modelResponse []OnModelResponse
	toolCall      []OnToolCall
	toolResult    []OnToolResult
}

func (h *hooks) onModelRequest(ctx context.Context, req ModelRequest) (context.Context, ModelRequest, error) {
	for _, fn := range h.modelRequest {
		var err error

		ctx, req, err = fn(ctx, req)
		if err != nil {
			return ctx, ModelRequest{}, err
		}
	}

	return ctx, req, nil
}

func (h *hooks) onModelResponse(ctx context.Context, resp ModelResponse) (context.Context, ModelResponse, error) {
	for _, fn := range h.modelResponse {
		var err error

		ctx, resp, err = fn(ctx, resp)
		if err != nil {
			return ctx, ModelResponse{}, err
		}
	}

	return ctx, resp, nil
}

func (h *hooks) onToolCall(ctx context.Context, call ToolCall) (context.Context, ToolCall, error) {
	for _, fn := range h.toolCall {
		var err error

		ctx, call, err = fn(ctx, call)
		if err != nil {
			return ctx, ToolCall{}, err
		}
	}

	return ctx, call, nil
}

func (h *hooks) onToolResult(ctx context.Context, call ToolCall, result string, err error) (context.Context, string, error) {
	for _, fn := range h.toolResult {
		var hookErr error

		ctx, result, hookErr = fn(ctx, call, result, err)
		if hookErr != nil {
			return ctx, "", hookErr
		}
	}

	return ctx, result, nil
}

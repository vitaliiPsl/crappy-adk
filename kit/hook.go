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
// The returned context and [ToolResult] replace the originals for the agent loop.
// Returning an error replaces the tool result with the error message.
type OnToolResult func(ctx context.Context, result ToolResult) (context.Context, ToolResult, error)

// OnRunStart is called once before the ReAct loop begins.
// The returned context and messages replace the originals for the run.
// Returning an error cancels the run.
type OnRunStart func(ctx context.Context, messages []Message) (context.Context, []Message, error)

// OnRunEnd is called once after the ReAct loop completes.
// err is non-nil if the run failed. Returning an error overrides the original run error.
type OnRunEnd func(ctx context.Context, response Response, err error) (context.Context, error)

// OnTurnStart is called at the beginning of each ReAct loop iteration, before the model is called.
// The returned context and messages replace the originals for this turn.
// Returning an error stops the agent.
type OnTurnStart func(ctx context.Context, messages []Message) (context.Context, []Message, error)

// OnTurnEnd is called at the end of each ReAct loop iteration, after all tool calls complete.
// messages is the full conversation history at that point.
// Returning an error stops the agent.
type OnTurnEnd func(ctx context.Context, messages []Message) (context.Context, error)

type hooks struct {
	modelRequest  []OnModelRequest
	modelResponse []OnModelResponse
	toolCall      []OnToolCall
	toolResult    []OnToolResult
	runStart      []OnRunStart
	runEnd        []OnRunEnd
	turnStart     []OnTurnStart
	turnEnd       []OnTurnEnd
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

func (h *hooks) onToolResult(ctx context.Context, result ToolResult) (context.Context, ToolResult, error) {
	for _, fn := range h.toolResult {
		var err error

		ctx, result, err = fn(ctx, result)
		if err != nil {
			return ctx, ToolResult{}, err
		}
	}

	return ctx, result, nil
}

func (h *hooks) onRunStart(ctx context.Context, messages []Message) (context.Context, []Message, error) {
	for _, fn := range h.runStart {
		var err error

		ctx, messages, err = fn(ctx, messages)
		if err != nil {
			return ctx, nil, err
		}
	}

	return ctx, messages, nil
}

func (h *hooks) onRunEnd(ctx context.Context, response Response, runErr error) (context.Context, error) {
	for _, fn := range h.runEnd {
		var err error

		ctx, err = fn(ctx, response, runErr)
		if err != nil {
			return ctx, err
		}
	}

	return ctx, nil
}

func (h *hooks) onTurnStart(ctx context.Context, messages []Message) (context.Context, []Message, error) {
	for _, fn := range h.turnStart {
		var err error

		ctx, messages, err = fn(ctx, messages)
		if err != nil {
			return ctx, nil, err
		}
	}

	return ctx, messages, nil
}

func (h *hooks) onTurnEnd(ctx context.Context, messages []Message) (context.Context, error) {
	for _, fn := range h.turnEnd {
		var err error

		ctx, err = fn(ctx, messages)
		if err != nil {
			return ctx, err
		}
	}

	return ctx, nil
}

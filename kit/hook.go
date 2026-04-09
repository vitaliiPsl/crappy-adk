package kit

import "context"

// OnModelRequest is called before each model request.
// The returned context and [ModelRequest] replace the originals for the model call.
// Returning an error cancels the request.
type OnModelRequest func(ctx context.Context, req ModelRequest) (context.Context, ModelRequest, error)

// OnModelResponse is called after each model result.
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
type OnRunEnd func(ctx context.Context, result Result, err error) (context.Context, error)

// OnTurnStart is called at the beginning of each ReAct loop iteration, before the model is called.
// The returned context and messages replace the originals for this turn.
// Returning an error stops the agent.
type OnTurnStart func(ctx context.Context, messages []Message) (context.Context, []Message, error)

// OnTurnEnd is called at the end of each ReAct loop iteration, after all tool calls complete.
// messages is the full conversation history at that point.
// Returning an error stops the agent.
type OnTurnEnd func(ctx context.Context, messages []Message) (context.Context, error)

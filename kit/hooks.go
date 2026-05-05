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
// Returning an error stops the agent.
type OnToolResult func(ctx context.Context, result ToolResult) (context.Context, ToolResult, error)

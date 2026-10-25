package kit

// OnTurnStart is called before each model call.
// Returning an error stops the agent.
type OnTurnStart func(rc *RunContext) error

// OnTurnEnd is called after a loop iteration completes — either after tool
// results have been appended or after a terminal model response.
// Returning an error stops the agent.
type OnTurnEnd func(rc *RunContext) error

// OnModelRequest transforms a fresh request; errors stop the run.
// Must be safe to repeat after context changes or model errors.
type OnModelRequest func(rc *RunContext, req ModelRequest) (ModelRequest, error)

// OnModelRequestReady runs once per turn, after construction.
// Returning true rebuilds the request before the next ready hook.
type OnModelRequestReady func(rc *RunContext, req ModelRequest) (rebuild bool, err error)

// OnModelResponse is called after each model response.
// Returned [ModelResponse] replaces the original for the agent loop.
// Returning an error stops the agent.
type OnModelResponse func(rc *RunContext, resp ModelResponse) (ModelResponse, error)

// OnModelError returns nil to rebuild and retry once, skipping ready hooks.
// Errors pass to the next error hook; the last error stops the run.
// Retrying may replay output already streamed by the failed call.
type OnModelError func(rc *RunContext, req ModelRequest, err error) error

// OnToolCall is called before a tool is executed.
// Returned [ToolCall] replaces the original for the tool execution.
// Returned error is sent to the model as a tool result.
type OnToolCall func(rc *RunContext, call ToolCall) (ToolCall, error)

// OnToolResult is called after a tool finishes executing.
// Returned [ToolResult] replaces the original for the agent loop.
// Returned error is sent to the model as a tool result.
type OnToolResult func(rc *RunContext, result ToolResult) (ToolResult, error)

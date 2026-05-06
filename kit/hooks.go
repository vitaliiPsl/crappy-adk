package kit

// OnTurnStart is called before each model call, after [RunContext.Turn] has been
// incremented. Hooks may inspect or mutate [RunContext]; this is where compaction
// strategies modify [RunContext.Messages]. Returning an error stops the agent.
type OnTurnStart func(rc *RunContext) error

// OnTurnEnd is called after a turn completes — either after tool results have
// been appended (mid-loop turn) or after a terminal model response.
// Returning an error stops the agent.
type OnTurnEnd func(rc *RunContext) error

// OnModelRequest is called before each model request.
// The hook may mutate [RunContext] (including [RunContext.Context]) and the
// returned [ModelRequest] replaces the original for the model call.
// Returning an error cancels the request.
type OnModelRequest func(rc *RunContext, req ModelRequest) (ModelRequest, error)

// OnModelResponse is called after each model response.
// The hook may mutate [RunContext] and the returned [ModelResponse] replaces
// the original for the agent loop. Returning an error stops the agent.
type OnModelResponse func(rc *RunContext, resp ModelResponse) (ModelResponse, error)

// OnToolCall is called before a tool is executed.
// The hook may mutate [RunContext] and the returned [ToolCall] replaces the
// original for the tool execution. Returning an error cancels the tool call.
type OnToolCall func(rc *RunContext, call ToolCall) (ToolCall, error)

// OnToolResult is called after a tool finishes executing.
// The hook may mutate [RunContext] and the returned [ToolResult] replaces the
// original for the agent loop. Returning an error stops the agent.
type OnToolResult func(rc *RunContext, result ToolResult) (ToolResult, error)

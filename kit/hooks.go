package kit

// OnTurnStart is called before each model call.
// Returning an error stops the agent.
type OnTurnStart func(rc *RunContext) error

// OnTurnEnd is called after a loop iteration completes — either after tool
// results have been appended or after a terminal model response.
// Returning an error stops the agent.
type OnTurnEnd func(rc *RunContext) error

// OnModelRequest is called before each model request.
// Returned [ModelRequest] replaces the original for the model call.
// Returning an error cancels the request.
type OnModelRequest func(rc *RunContext, req ModelRequest) (ModelRequest, error)

// OnModelResponse is called after each model response.
// Returned [ModelResponse] replaces the original for the agent loop.
// Returning an error stops the agent.
type OnModelResponse func(rc *RunContext, resp ModelResponse) (ModelResponse, error)

// OnModelError is called when a model call fails.
// Returning a [ModelRequest] retries the call once with it; the retry does not
// run [OnModelRequest] hooks, streams its output again if the failed call had
// already streamed some, and its failure is final.
// Returning an error passes it to the next hook; the last error stops the agent.
type OnModelError func(rc *RunContext, req ModelRequest, err error) (ModelRequest, error)

// OnToolCall is called before a tool is executed.
// Returned [ToolCall] replaces the original for the tool execution.
// Returned error is sent to the model as a tool result.
type OnToolCall func(rc *RunContext, call ToolCall) (ToolCall, error)

// OnToolResult is called after a tool finishes executing.
// Returned [ToolResult] replaces the original for the agent loop.
// Returned error is sent to the model as a tool result.
type OnToolResult func(rc *RunContext, result ToolResult) (ToolResult, error)

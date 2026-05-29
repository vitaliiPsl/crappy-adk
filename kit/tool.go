package kit

// Tool is an action the agent can take.
type Tool interface {
	// Definition returns the tool's metadata, used to describe it to the model.
	Definition() ToolDefinition
	// Execute runs the tool with the given arguments and returns its output.
	Execute(rc *RunContext, input map[string]any) (string, error)
}

// ToolDefinition describes a tool to the model.
type ToolDefinition struct {
	// The unique name of the tool.
	Name string `json:"name"`
	// Description explains what the tool does and how to use it.
	Description string `json:"description,omitempty"`
	// Schema defines the expected input structure for the tool.
	Schema map[string]any `json:"schema,omitempty"`
}

// ToolCall represents a request to execute a tool.
type ToolCall struct {
	// Unique identifier for this call, used to match results back to the model.
	ID string `json:"id"`
	// Name of the tool to execute.
	Name string `json:"name"`
	// Arguments parsed to the tool.
	Arguments map[string]any `json:"arguments,omitempty"`
	// Signature is opaque provider metadata required by some models to replay signed tool calls.
	Signature string `json:"signature,omitempty"`
}

// ToolResult represents the output of a tool execution.
type ToolResult struct {
	// Call contains the original tool call details.
	Call ToolCall `json:"call"`
	// Output is the successful output from the tool, if any.
	Output string `json:"output,omitempty"`
	// Error is the error message if the tool execution failed.
	Error string `json:"error,omitempty"`
}

// NewToolCall creates a new [ToolCall] with the given parameters.
func NewToolCall(id, name string, arguments map[string]any) ToolCall {
	return ToolCall{
		ID:        id,
		Name:      name,
		Arguments: arguments,
	}
}

// NewToolResult creates a new [ToolResult] for the given call, output, and error.
func NewToolResult(call ToolCall, output string, err error) ToolResult {
	var errMsg string
	if err != nil {
		errMsg = err.Error()
	}

	return ToolResult{
		Call:   call,
		Output: output,
		Error:  errMsg,
	}
}

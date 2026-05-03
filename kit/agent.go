package kit

import "context"

// Agent runs a ReAct loop against a [Model], executes any requested [Tool] calls,
// and returns the final response.
type Agent interface {
	Run(ctx context.Context, messages []Message) (AgentResponse, error)
}

// AgentResponse is the result of a complete agent run.
type AgentResponse struct {
	// Output contains the first text content of the final model message.
	Output *Text
	// Messages contains the messages generated during the run.
	Messages []Message
	// Usage is the accumulated token usage across all turns.
	Usage Usage
}

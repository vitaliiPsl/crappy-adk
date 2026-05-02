package kit

import "context"

// Agent runs a multi-turn conversation with a model.
type Agent interface {
	Run(ctx context.Context, messages []Message) (AgentResponse, error)
}

// AgentResponse is the result of a complete agent run.
type AgentResponse struct {
	// Message is the final message produced by the model.
	Message Message
	// Usage is the accumulated token usage across all turns.
	Usage Usage
}

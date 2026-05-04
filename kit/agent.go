package kit

import "context"

// Agent runs a ReAct loop against a [Model], executes any requested [Tool] calls,
// and returns the final response.
type Agent interface {
	Run(ctx context.Context, messages []Message) (AgentResponse, error)
}

// AgentConfig holds agent-level configuration applied to every model call in a run.
type AgentConfig struct {
	// Instructions provides the system prompt for the agent.
	Instructions string

	// Temperature controls randomness. Higher values produce more varied output.
	Temperature *float32

	// MaxOutputTokens limits the number of tokens the model can generate per turn.
	MaxOutputTokens *int32

	// Thinking controls extended thinking. Defaults to ThinkingDisabled.
	Thinking *ThinkingLevel
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

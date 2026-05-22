package kit

import "context"

// Agent runs a ReAct loop against a [Model], executes any requested [Tool] calls,
// and returns the final response.
type Agent interface {
	Run(ctx context.Context, input Message) (AgentResponse, error)
	Stream(ctx context.Context, input Message) *Stream[AgentEvent, AgentResponse]
}

// AgentConfig is the configuration for an [Agent].
type AgentConfig struct {
	// Instructions describe the agent's behavior and role.
	Instructions string

	// Temperature controls response randomness. Higher values produce more varied output.
	Temperature *float32

	// MaxOutputTokens limits the length of each response.
	MaxOutputTokens *int32

	// Thinking controls the level of extended reasoning. Defaults to [ThinkingDisabled].
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
	// LastUsage is the usage from the most recent model call.
	LastUsage Usage
}

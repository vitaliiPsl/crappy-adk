package kit

import (
	"context"
	"encoding/json"
	"strings"
)

// Agent runs a ReAct loop against a [Model], executes any requested [Tool] calls,
// and returns the final response.
type Agent interface {
	Context(ctx context.Context) (AgentContext, error)
	Run(ctx context.Context, input ...Message) (AgentResponse, error)
	Stream(ctx context.Context, input ...Message) *Stream[AgentEvent, AgentResponse]
}

// History holds the conversation the model sees. The agent reads Messages
// before every model request, so it must be safe to repeat and may reflect
// changes made since the previous call, such as compaction.
type History interface {
	Messages(ctx context.Context) ([]Message, error)
	Append(ctx context.Context, messages ...Message) error
}

// AgentConfig is the configuration for an [Agent].
type AgentConfig struct {
	// Instructions describe the agent's behavior and role. They render before
	// sourced instructions.
	Instructions string
	// Temperature controls response randomness. Higher values produce more varied output.
	Temperature *float32
	// MaxOutputTokens limits the length of each response.
	MaxOutputTokens *int32
	// Thinking controls the level of extended reasoning. Defaults to [ThinkingDisabled].
	Thinking *ThinkingLevel
	// OutputSchema constrains final responses to a JSON schema.
	OutputSchema *OutputSchema
}

// AgentContext is what a model request is built from.
type AgentContext struct {
	Instructions []Instruction `json:"instructions,omitempty"`
	Messages     []Message     `json:"messages,omitempty"`
	Tools        []Tool        `json:"-"`
}

// InstructionsText renders instruction sections in order.
func (c AgentContext) InstructionsText() string {
	sections := make([]string, len(c.Instructions))
	for i, instruction := range c.Instructions {
		sections[i] = instruction.Text
		if instruction.Source != "" {
			sections[i] = "# " + instruction.Source + "\n\n" + instruction.Text
		}
	}

	return strings.Join(sections, "\n\n")
}

// Instruction is a section of agent guidance with an optional source heading.
type Instruction struct {
	Source string `json:"source,omitempty"`
	Text   string `json:"text"`
}

// NewInstructions creates sections from text under source, trimming
// whitespace and skipping empty text.
func NewInstructions(source string, text ...string) []Instruction {
	source = strings.TrimSpace(source)
	instructions := make([]Instruction, 0, len(text))

	for _, value := range text {
		if value = strings.TrimSpace(value); value != "" {
			instructions = append(instructions, Instruction{Source: source, Text: value})
		}
	}

	return instructions
}

// AgentResponse is the result of a complete agent run.
type AgentResponse struct {
	// Output contains the first text content of the final model message.
	Output *Text
	// StructuredOutput contains validated JSON when an output schema is configured.
	StructuredOutput json.RawMessage
	// Messages contains the messages generated during the run.
	Messages []Message
	// Usage is the accumulated token usage across all turns.
	Usage Usage
	// LastUsage is the usage from the most recent model call.
	LastUsage Usage
	// FinishReason indicates why the final model call stopped generating.
	FinishReason FinishReason
}

package kit

import (
	"context"

	"github.com/google/jsonschema-go/jsonschema"

	"github.com/vitaliiPsl/crappy-adk/x/stream"
)

// Agent runs a ReAct loop against a [Model], executes any requested tool calls,
// and returns the model's final answer.
type Agent interface {
	// Name returns the agent's identifier, used in catalogs and logs.
	Name() string

	// Description explains what this agent does and when to use it.
	Description() string

	// Run executes the ReAct loop and returns the accumulated [Result] once the
	// agent reaches a final answer.
	Run(ctx context.Context, messages []Message) (Result, error)

	// Stream executes the ReAct loop and returns a [stream.Stream] that emits
	// incremental [Event] values as the agent works. Call Result on the stream
	// after iteration to retrieve the accumulated [Result].
	Stream(ctx context.Context, messages []Message) (*stream.Stream[Event, Result], error)
}

// AgentConfig holds the static configuration for an [Agent].
type AgentConfig struct {
	// Name is the agent's identifier, used in catalogs and logs.
	Name string

	// Description explains what this agent does and when to use it.
	// Used by parent agents to decide which subagent to delegate to.
	Description string

	// SystemPrompt is a static system prompt used as-is on every run.
	SystemPrompt string

	// Temperature controls randomness. Nil uses the model default.
	Temperature *float32

	// TopP limits sampling to the smallest set of tokens whose cumulative
	// probability meets this threshold. Nil uses the model default.
	TopP *float32

	// MaxOutputTokens limits the number of tokens the model can generate.
	// Nil uses the model default.
	MaxOutputTokens *int32

	// ResponseSchema constrains the final assistant answer to JSON matching this schema.
	ResponseSchema *jsonschema.Schema

	// Thinking controls extended thinking. Empty disables it.
	Thinking ThinkingLevel

	// ToolExecution controls whether tool calls run in parallel or sequentially.
	// Defaults to ToolExecutionParallel.
	ToolExecution ToolExecutionMode

	// CompactionThreshold is the fraction of the context window that triggers
	// compaction.
	CompactionThreshold float64
}

// ToolExecutionMode controls how multiple tool calls in a single turn are executed.
type ToolExecutionMode string

const (
	// ToolExecutionParallel executes tool calls concurrently. This is the default.
	ToolExecutionParallel ToolExecutionMode = "parallel"
	// ToolExecutionSequential executes tool calls one at a time, in order.
	ToolExecutionSequential ToolExecutionMode = "sequential"
)

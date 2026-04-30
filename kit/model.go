package kit

import (
	"context"
	"time"

	"github.com/google/jsonschema-go/jsonschema"

	"github.com/vitaliiPsl/crappy-adk/x/stream"
)

// ModelMiddleware wraps a [Model] to intercept and modify its behaviour.
type ModelMiddleware func(Model) Model

// Model is a single AI model capable of generating responses.
type Model interface {
	// Config returns the static configuration for this model.
	Config() ModelConfig

	// Generate sends a request to the model and returns its response.
	Generate(ctx context.Context, req ModelRequest) (ModelResponse, error)

	// GenerateStream sends a request to the model and streams the response as
	// a sequence of chunks. The stream also exposes the final ModelResponse
	// once iteration completes.
	GenerateStream(ctx context.Context, req ModelRequest) (*stream.Stream[Event, ModelResponse], error)
}

// ModelConfig holds static metadata for a model.
type ModelConfig struct {
	// ID is the model identifier used when calling the provider API.
	ID string `json:"id" yaml:"id"`

	// Provider is the name of the provider that owns this model.
	Provider string `json:"provider,omitempty" yaml:"provider,omitempty"`

	// ContextWindow is the total token budget (input + output).
	ContextWindow int `json:"context_window,omitempty" yaml:"context_window,omitempty"`

	// InputLimit is the maximum number of input tokens the model accepts.
	InputLimit int `json:"input_limit,omitempty" yaml:"input_limit,omitempty"`

	// OutputLimit is the maximum number of tokens the model can generate.
	OutputLimit int `json:"output_limit,omitempty" yaml:"output_limit,omitempty"`

	// Cost holds the pricing information for this model.
	Cost ModelCost `json:"cost" yaml:"cost,omitempty"`

	// Capabilities describes what the model can do.
	Capabilities ModelCapabilities `json:"capabilities" yaml:"capabilities,omitempty"`

	// KnowledgeCutoff is the date after which the model has no training data.
	KnowledgeCutoff time.Time `json:"knowledge_cutoff,omitzero" yaml:"knowledge_cutoff,omitempty"`

	// ReleaseDate is when the model became publicly available.
	ReleaseDate time.Time `json:"release_date,omitzero" yaml:"release_date,omitempty"`
}

// ModelCost holds the pricing information for a model in USD per million tokens.
type ModelCost struct {
	// Input is the cost per million input tokens.
	Input float64 `json:"input,omitempty" yaml:"input,omitempty"`

	// Output is the cost per million output tokens.
	Output float64 `json:"output,omitempty" yaml:"output,omitempty"`

	// CacheRead is the cost per million cache-read tokens. Zero if unsupported.
	CacheRead float64 `json:"cache_read,omitempty" yaml:"cache_read,omitempty"`

	// CacheWrite is the cost per million cache-write tokens. Zero if unsupported.
	CacheWrite float64 `json:"cache_write,omitempty" yaml:"cache_write,omitempty"`
}

// ModelCapabilities describes the input modalities and features a model supports.
type ModelCapabilities struct {
	// Input modalities
	Text  bool `json:"text,omitempty" yaml:"text,omitempty"`
	Image bool `json:"image,omitempty" yaml:"image,omitempty"`
	Audio bool `json:"audio,omitempty" yaml:"audio,omitempty"`
	Video bool `json:"video,omitempty" yaml:"video,omitempty"`
	PDF   bool `json:"pdf,omitempty" yaml:"pdf,omitempty"`

	// Features
	Tools     bool `json:"tools,omitempty" yaml:"tools,omitempty"`
	Streaming bool `json:"streaming,omitempty" yaml:"streaming,omitempty"`
	Reasoning bool `json:"reasoning,omitempty" yaml:"reasoning,omitempty"` // extended thinking / o1-style reasoning
	Caching   bool `json:"caching,omitempty" yaml:"caching,omitempty"`
	Batch     bool `json:"batch,omitempty" yaml:"batch,omitempty"`
}

// ModelRequest is the input to a model generation call.
type ModelRequest struct {
	// Instruction is the system prompt passed to the model.
	Instruction string

	// Messages is the conversation history.
	Messages []Message

	// Tools is the set of available tools.
	Tools []ToolDefinition

	// ResponseSchema constrains the final assistant message to JSON matching this
	// schema. Providers may enforce the schema natively; the returned structured
	// output is always validated locally before it is exposed to callers.
	ResponseSchema *jsonschema.Schema

	// Config controls generation parameters. Zero value uses model defaults.
	Config GenerationConfig
}

// ModelResponse is the output of a model generation call.
type ModelResponse struct {
	// Message is the assistant message produced by the model.
	Message Message

	// StructuredOutput is the validated JSON payload produced when
	// [ModelRequest.ResponseSchema] is set.
	StructuredOutput *StructuredOutput

	// FinishReason indicates why the model stopped generating.
	FinishReason FinishReason

	// Usage reports token consumption for this call.
	Usage Usage
}

// GenerationConfig controls how the model generates its response.
// All fields are optional; unset fields use the model's defaults.
type GenerationConfig struct {
	// Temperature controls randomness. Higher values produce more varied output.
	Temperature *float32

	// TopP limits sampling to the smallest set of tokens whose cumulative
	// probability meets this threshold.
	TopP *float32

	// MaxOutputTokens limits the number of tokens the model can generate.
	MaxOutputTokens *int32

	// Thinking controls extended thinking. Defaults to ThinkingDisabled.
	Thinking ThinkingLevel
}

// ThinkingLevel controls how much reasoning effort the model applies.
type ThinkingLevel string

const (
	// ThinkingDisabled turns off extended thinking. This is the default.
	ThinkingDisabled ThinkingLevel = ""
	// ThinkingLevelLow applies minimal reasoning, faster and cheaper.
	ThinkingLevelLow ThinkingLevel = "low"
	// ThinkingLevelMedium applies moderate reasoning.
	ThinkingLevelMedium ThinkingLevel = "medium"
	// ThinkingLevelHigh applies thorough reasoning, slower and more expensive.
	ThinkingLevelHigh ThinkingLevel = "high"
)

// FinishReason indicates why the model stopped generating.
type FinishReason string

const (
	// FinishReasonStop means the model reached a natural stopping point.
	FinishReasonStop FinishReason = "stop"
	// FinishReasonMaxTokens means the output token limit was reached.
	FinishReasonMaxTokens FinishReason = "max_tokens"
	// FinishReasonToolCall means the model is requesting one or more tool calls.
	FinishReasonToolCall FinishReason = "tool_call"
	// FinishReasonSafety means the response was blocked by a safety filter.
	FinishReasonSafety FinishReason = "safety"
	// FinishReasonUnknown means the stop reason was not recognised.
	FinishReasonUnknown FinishReason = "unknown"
)

// Usage reports the number of tokens consumed by a model call.
type Usage struct {
	// InputTokens is the number of tokens in the request.
	InputTokens int32

	// OutputTokens is the number of tokens generated in the response.
	OutputTokens int32

	// CacheReadTokens is the number of input tokens read from the cache.
	// Zero if caching was not used or not supported.
	CacheReadTokens int32

	// CacheWriteTokens is the number of input tokens written to the cache.
	// Zero if no new cache entry was created or not supported.
	CacheWriteTokens int32

	// ReasoningTokens is the number of tokens used for internal reasoning.
	// Zero if the model does not support extended thinking.
	ReasoningTokens int32
}

// Add accumulates the token counts from other into u.
func (u *Usage) Add(other Usage) {
	u.InputTokens += other.InputTokens
	u.OutputTokens += other.OutputTokens
	u.CacheReadTokens += other.CacheReadTokens
	u.CacheWriteTokens += other.CacheWriteTokens
	u.ReasoningTokens += other.ReasoningTokens
}

// Cost calculates the total USD cost of this usage given the model's pricing.
// CacheReadTokens are subtracted from InputTokens before applying the input rate
// since providers include them in the InputTokens total.
func (u Usage) Cost(c ModelCost) float64 {
	const perMillion = 1_000_000.0

	netInput := float64(u.InputTokens-u.CacheReadTokens) / perMillion
	cacheRead := float64(u.CacheReadTokens) / perMillion
	cacheWrite := float64(u.CacheWriteTokens) / perMillion
	output := float64(u.OutputTokens) / perMillion

	return netInput*c.Input + cacheRead*c.CacheRead + cacheWrite*c.CacheWrite + output*c.Output
}

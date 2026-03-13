package kit

import (
	"context"
	"iter"
)

// Provider is a factory for creating models from a specific AI provider.
type Provider interface {
	// Model returns the model with the given ID, or an error if it is unknown.
	Model(id string) (Model, error)

	// Models returns all models supported by this provider.
	Models() ([]ModelConfig, error)
}

// Model is a single AI model capable of generating responses.
type Model interface {
	// Config returns the static configuration for this model.
	Config() ModelConfig

	// Generate sends a request to the model and returns its response.
	Generate(ctx context.Context, req ModelRequest) (ModelResponse, error)

	// GenerateStream sends a request to the model and streams the response as
	// a sequence of chunks. The stream also exposes the final ModelResponse
	// once iteration completes.
	GenerateStream(ctx context.Context, req ModelRequest) (*ModelStream, error)
}

// ModelConfig holds static metadata for a model.
type ModelConfig struct {
	// ID is the model identifier used when calling the provider API.
	ID string

	// Provider is the name of the provider that owns this model.
	Provider string

	// ContextWindow is the maximum number of input tokens the model accepts.
	ContextWindow int64

	// MaxOutputTokens is the maximum number of tokens the model can generate.
	MaxOutputTokens int64

	// InputPricePerM is the cost in USD per million input tokens.
	InputPricePerM float64

	// OutputPricePerM is the cost in USD per million output tokens.
	OutputPricePerM float64
}

// ModelRequest is the input to a model generation call.
type ModelRequest struct {
	// Instruction is the system prompt passed to the model.
	Instruction string

	// Messages is the conversation history.
	Messages []Message

	// Tools is the set of available tools.
	Tools []ToolDefinition

	// Config controls generation parameters. Zero value uses model defaults.
	Config GenerationConfig
}

// ModelResponse is the output of a model generation call.
type ModelResponse struct {
	// Message is the assistant message produced by the model.
	Message Message

	// FinishReason indicates why the model stopped generating.
	FinishReason FinishReason

	// Usage reports token consumption for this call.
	Usage Usage
}

// ModelStream is the result of a streaming generation call. Callers consume
// delta chunks via Iter and retrieve the assembled result via Response.
type ModelStream struct {
	iter     iter.Seq2[ModelChunk, error]
	response ModelResponse
	done     bool
}

// NewModelStream constructs a ModelStream from fn. fn is invoked lazily on
// first iteration; it should yield ModelChunk deltas and return the complete
// ModelResponse when done.
func NewModelStream(fn func(yield func(ModelChunk, error) bool) ModelResponse) *ModelStream {
	s := &ModelStream{}
	s.iter = func(yield func(ModelChunk, error) bool) {
		s.response = fn(yield)
		s.done = true
	}

	return s
}

// Iter returns an iterator over the incremental chunks of the stream.
func (s *ModelStream) Iter() iter.Seq2[ModelChunk, error] {
	return s.iter
}

// Response returns the complete ModelResponse. If the stream has not been
// fully consumed, it drains the remaining chunks first.
func (s *ModelStream) Response() ModelResponse {
	if !s.done {
		for range s.iter {
			_ = "" // drain remaining chunks
		}
	}

	return s.response
}

// ChunkType indicates the kind of content carried by a ModelChunk.
type ChunkType string

const (
	// ChunkTypeText is an incremental piece of the model's text response.
	ChunkTypeText ChunkType = "text"
	// ChunkTypeThinking is an incremental piece of the model's reasoning.
	ChunkTypeThinking ChunkType = "thinking"
	// ChunkTypeToolCall is a completed tool call requested by the model.
	ChunkTypeToolCall ChunkType = "tool_call"
)

// ModelChunk is a single incremental piece of a streamed model response.
type ModelChunk struct {
	// Type indicates what kind of content this chunk carries.
	Type     ChunkType
	Text     string
	Thinking string
	ToolCall *ToolCall
}

// NewTextChunk creates a ModelChunk carrying an incremental text delta.
func NewTextChunk(text string) ModelChunk {
	return ModelChunk{Type: ChunkTypeText, Text: text}
}

// NewThinkingChunk creates a ModelChunk carrying an incremental thinking delta.
func NewThinkingChunk(thinking string) ModelChunk {
	return ModelChunk{Type: ChunkTypeThinking, Thinking: thinking}
}

// NewToolCallChunk creates a ModelChunk carrying a completed tool call.
func NewToolCallChunk(tc ToolCall) ModelChunk {
	return ModelChunk{Type: ChunkTypeToolCall, ToolCall: &tc}
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
}

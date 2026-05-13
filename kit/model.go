package kit

import "context"

// Model represents a language model that can generate responses based on a conversation history and system prompt.
type Model interface {
	Config() ModelConfig
	Generate(ctx context.Context, request ModelRequest) (ModelResponse, error)
	Stream(ctx context.Context, request ModelRequest) *Stream[ModelEvent, ModelResponse]
}

// ModelRequest is the input to a model call.
type ModelRequest struct {
	// Instructions provides instructions to the model.
	Instructions string
	// Messages is the conversation history.
	Messages []Message
	// Tools is a list of tools the model can call during generation.
	Tools []Tool
	// Config controls how the model generates its response.
	Config GenerationConfig
}

// ModelResponse is the output of a model call.
type ModelResponse struct {
	// Message is the assistant message produced by the model.
	Message Message
	// FinishReason indicates why the model stopped generating.
	FinishReason FinishReason
	// Usage contains token usage statistics for this model call.
	Usage Usage
}

// GenerationConfig controls how the model generates its response.
type GenerationConfig struct {
	// Temperature controls randomness. Higher values produce more varied output.
	Temperature *float32
	// MaxOutputTokens limits the number of tokens the model can generate.
	MaxOutputTokens *int32
	// Thinking controls extended thinking. Defaults to ThinkingDisabled.
	Thinking *ThinkingLevel
}

// ThinkingLevel controls how much reasoning effort the model applies.
type ThinkingLevel string

const (
	// ThinkingDisabled turns off extended thinking. This is the default.
	ThinkingDisabled ThinkingLevel = "disabled"
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

// Usage holds the number of tokens consumed by a model call.
type Usage struct {
	// InputTokens is the number of tokens in the request.
	InputTokens int64
	// OutputTokens is the number of tokens generated in the response.
	OutputTokens int64
	// CacheReadTokens is the number of input tokens read from the cache.
	// Zero if caching was not used or not supported.
	CacheReadTokens int64
	// CacheWriteTokens is the number of input tokens written to the cache.
	// Zero if no new cache entry was created or not supported.
	CacheWriteTokens int64
	// ReasoningTokens is the number of tokens used for internal reasoning.
	// Zero if the model does not support extended thinking.
	ReasoningTokens int64
}

// Add accumulates the token counts from other into u.
func (u *Usage) Add(other Usage) {
	u.InputTokens += other.InputTokens
	u.OutputTokens += other.OutputTokens
	u.CacheReadTokens += other.CacheReadTokens
	u.CacheWriteTokens += other.CacheWriteTokens
	u.ReasoningTokens += other.ReasoningTokens
}

// ModelConfig holds static metadata for a model.
type ModelConfig struct {
	// ID is the model identifier used when calling the provider API.
	ID string
	// Provider is the name of the provider that owns this model.
	Provider string
	// ContextWindow is the total token budget (input + output).
	ContextWindow int
	// InputLimit is the maximum number of input tokens the model accepts.
	InputLimit int
	// OutputLimit is the maximum number of tokens the model can generate.
	OutputLimit int
	// Cost holds the pricing information for this model.
	Cost ModelCost
	// Capabilities describes what the model can do.
	Capabilities ModelCapabilities
	// KnowledgeCutoff is the date after which the model has no training data.
	KnowledgeCutoff string
	// ReleaseDate is when the model became publicly available.
	ReleaseDate string
}

// ModelCost holds the pricing information for a model in USD per million tokens.
type ModelCost struct {
	// Input is the cost per million input tokens.
	Input float64
	// Output is the cost per million output tokens.
	Output float64
	// CacheRead is the cost per million cache-read tokens. Zero if unsupported.
	CacheRead float64
	// CacheWrite is the cost per million cache-write tokens. Zero if unsupported.
	CacheWrite float64
}

// ModelCapabilities describes the input modalities and features a model supports.
type ModelCapabilities struct {
	// Input modalities
	Text  bool
	Image bool
	Audio bool
	Video bool
	PDF   bool

	// Features
	Tools     bool
	Streaming bool
	Reasoning bool
	Caching   bool
	Batch     bool
}

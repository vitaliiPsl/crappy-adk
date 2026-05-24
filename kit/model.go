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
	Instructions string `json:"instructions,omitempty"`
	// Messages is the conversation history.
	Messages []Message `json:"messages,omitempty"`
	// Tools is a list of tools the model can call during generation.
	Tools []Tool `json:"-"`
	// Config controls how the model generates its response.
	Config GenerationConfig `json:"config"`
}

// ModelResponse is the output of a model call.
type ModelResponse struct {
	// Message is the assistant message produced by the model.
	Message Message `json:"message"`
	// FinishReason indicates why the model stopped generating.
	FinishReason FinishReason `json:"finish_reason,omitempty"`
	// Usage contains token usage statistics for this model call.
	Usage Usage `json:"usage"`
}

// GenerationConfig controls how the model generates its response.
type GenerationConfig struct {
	// Temperature controls randomness. Higher values produce more varied output.
	Temperature *float32 `json:"temperature,omitempty"`
	// MaxOutputTokens limits the number of tokens the model can generate.
	MaxOutputTokens *int32 `json:"max_output_tokens,omitempty"`
	// Thinking controls extended thinking. Defaults to ThinkingDisabled.
	Thinking *ThinkingLevel `json:"thinking,omitempty"`
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
	InputTokens int64 `json:"input_tokens,omitempty"`
	// OutputTokens is the number of tokens generated in the response.
	OutputTokens int64 `json:"output_tokens,omitempty"`
	// CacheReadTokens is the number of input tokens read from the cache.
	// Zero if caching was not used or not supported.
	CacheReadTokens int64 `json:"cache_read_tokens,omitempty"`
	// CacheWriteTokens is the number of input tokens written to the cache.
	// Zero if no new cache entry was created or not supported.
	CacheWriteTokens int64 `json:"cache_write_tokens,omitempty"`
	// ReasoningTokens is the number of tokens used for internal reasoning.
	// Zero if the model does not support extended thinking.
	ReasoningTokens int64 `json:"reasoning_tokens,omitempty"`
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
	ID string `json:"id,omitempty" yaml:"id,omitempty"`
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
	KnowledgeCutoff string `json:"knowledge_cutoff,omitempty" yaml:"knowledge_cutoff,omitempty"`
	// ReleaseDate is when the model became publicly available.
	ReleaseDate string `json:"release_date,omitempty" yaml:"release_date,omitempty"`
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
	Reasoning bool `json:"reasoning,omitempty" yaml:"reasoning,omitempty"`
	Caching   bool `json:"caching,omitempty" yaml:"caching,omitempty"`
	Batch     bool `json:"batch,omitempty" yaml:"batch,omitempty"`
}

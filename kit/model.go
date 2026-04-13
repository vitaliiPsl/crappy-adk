package kit

import (
	"context"
	"time"
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
	GenerateStream(ctx context.Context, req ModelRequest) (*Stream[ModelEvent, ModelResponse], error)
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
	KnowledgeCutoff time.Time

	// ReleaseDate is when the model became publicly available.
	ReleaseDate time.Time
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
	Reasoning bool // extended thinking / o1-style reasoning
	Caching   bool
	Batch     bool
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

type ModelEventType string

const (
	ModelEventThinkingStarted    ModelEventType = "thinking_started"
	ModelEventThinkingDelta      ModelEventType = "thinking_delta"
	ModelEventThinkingDone       ModelEventType = "thinking_done"
	ModelEventContentPartStarted ModelEventType = "content_part_started"
	ModelEventContentPartDelta   ModelEventType = "content_part_delta"
	ModelEventContentPartDone    ModelEventType = "content_part_done"
	ModelEventToolCallStarted    ModelEventType = "tool_call_started"
	ModelEventToolCallDone       ModelEventType = "tool_call_done"
)

type ModelEvent struct {
	Type ModelEventType

	Thinking string

	ContentPartType ContentType
	ContentPart     *ContentPart
	Text            string

	ToolCall *ToolCall
}

func NewModelThinkingStartedEvent() ModelEvent {
	return ModelEvent{Type: ModelEventThinkingStarted}
}

func NewModelThinkingDeltaEvent(text string) ModelEvent {
	return ModelEvent{Type: ModelEventThinkingDelta, Text: text}
}

func NewModelThinkingDoneEvent(thinking string) ModelEvent {
	return ModelEvent{Type: ModelEventThinkingDone, Thinking: thinking}
}

func NewModelContentPartStartedEvent(partType ContentType) ModelEvent {
	return ModelEvent{
		Type:            ModelEventContentPartStarted,
		ContentPartType: partType,
	}
}

func NewModelContentPartDeltaEvent(partType ContentType, text string) ModelEvent {
	return ModelEvent{
		Type:            ModelEventContentPartDelta,
		ContentPartType: partType,
		Text:            text,
	}
}

func NewModelContentPartDoneEvent(part ContentPart) ModelEvent {
	return ModelEvent{
		Type:            ModelEventContentPartDone,
		ContentPartType: part.Type,
		ContentPart:     &part,
	}
}

func NewModelToolCallStartedEvent(tc ToolCall) ModelEvent {
	return ModelEvent{Type: ModelEventToolCallStarted, ToolCall: &tc}
}

func NewModelToolCallDoneEvent(tc ToolCall) ModelEvent {
	return ModelEvent{Type: ModelEventToolCallDone, ToolCall: &tc}
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

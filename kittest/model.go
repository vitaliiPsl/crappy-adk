package kittest

import (
	"context"
	"testing"

	"github.com/vitaliiPsl/crappy-adk/kit"
	"github.com/vitaliiPsl/crappy-adk/x/stream"
)

// ChunkResult represents a single yield from a model stream: either a
// successful chunk or an error.
type ChunkResult struct {
	Event kit.Event
	Err   error
}

// ModelTurn describes a single model response in terms the test cares about.
type ModelTurn struct {
	Text             string
	Thinking         string
	ToolCalls        []kit.ToolCall
	StructuredOutput *kit.StructuredOutput
	Usage            kit.Usage
	Error            error

	// Stream, when set, overrides the auto-generated chunks for [Model.GenerateStream].
	// Each [ChunkResult] is yielded in order, allowing tests to inject stream-level errors.
	Stream []ChunkResult
}

func (turn ModelTurn) modelResponse() kit.ModelResponse {
	var parts []kit.ContentPart
	if turn.Thinking != "" {
		parts = append(parts, kit.NewThinkingPart(turn.Thinking, ""))
	}

	if turn.Text != "" {
		parts = append(parts, kit.NewTextPart(turn.Text))
	}

	for _, tc := range turn.ToolCalls {
		parts = append(parts, kit.NewToolCallPart(tc))
	}

	return kit.ModelResponse{
		Message:          kit.NewAssistantMessage(parts...),
		StructuredOutput: turn.StructuredOutput,
		FinishReason:     turn.finishReason(),
		Usage:            turn.Usage,
	}
}

func (turn ModelTurn) events() []kit.Event {
	var events []kit.Event

	if turn.Thinking != "" {
		events = append(events,
			kit.NewContentPartStartedEvent(kit.ContentTypeThinking),
			kit.NewContentPartDeltaEvent(kit.ContentTypeThinking, turn.Thinking),
			kit.NewContentPartDoneEvent(kit.NewThinkingPart(turn.Thinking, "")),
		)
	}

	if turn.Text != "" {
		part := kit.NewTextPart(turn.Text)
		events = append(events,
			kit.NewContentPartStartedEvent(kit.ContentTypeText),
			kit.NewContentPartDeltaEvent(kit.ContentTypeText, turn.Text),
			kit.NewContentPartDoneEvent(part),
		)
	}

	for _, toolCall := range turn.ToolCalls {
		part := kit.NewToolCallPart(toolCall)
		events = append(events,
			kit.NewContentPartStartedEvent(kit.ContentTypeToolCall),
			kit.NewContentPartDoneEvent(part),
		)
	}

	return events
}

func (turn ModelTurn) finishReason() kit.FinishReason {
	if len(turn.ToolCalls) > 0 {
		return kit.FinishReasonToolCall
	}

	return kit.FinishReasonStop
}

// Model is a programmable test double for [kit.Model]. Callers describe a
// sequence of [ModelTurn] values. Each Generate or GenerateStream call pops the
// next one. If the queue is exhausted the test fails.
type Model struct {
	t      *testing.T
	config kit.ModelConfig
	turns  []ModelTurn
	calls  []kit.ModelRequest
	idx    int
}

// NewModel creates a [Model] that will play through the given turns in order.
func NewModel(t *testing.T, turns ...ModelTurn) *Model {
	return &Model{
		t:     t,
		turns: turns,
		config: kit.ModelConfig{
			ID:            "test-model",
			Provider:      "test",
			ContextWindow: 128_000,
			InputLimit:    128_000,
			OutputLimit:   16_000,
		},
	}
}

// Config returns the static model configuration.
func (model *Model) Config() kit.ModelConfig {
	return model.config
}

// Generate returns the next queued turn as a complete response.
func (model *Model) Generate(_ context.Context, req kit.ModelRequest) (kit.ModelResponse, error) {
	turn := model.next(req)
	if turn.Error != nil {
		return kit.ModelResponse{}, turn.Error
	}

	return turn.modelResponse(), nil
}

// GenerateStream returns a stream that yields the next turn's chunks, then
// exposes the assembled response. When [Turn.Stream] is set, those chunk
// results are yielded.
func (model *Model) GenerateStream(_ context.Context, req kit.ModelRequest) (*stream.Stream[kit.Event, kit.ModelResponse], error) {
	turn := model.next(req)
	if turn.Error != nil {
		return nil, turn.Error
	}

	resp := turn.modelResponse()

	if turn.Stream != nil {
		results := turn.Stream

		return stream.New(func(e *stream.Emitter[kit.Event]) (kit.ModelResponse, error) {
			for _, result := range results {
				if result.Err != nil {
					return resp, result.Err
				}

				if err := e.Emit(result.Event); err != nil {
					return resp, nil
				}
			}

			return resp, nil
		}), nil
	}

	events := turn.events()

	return stream.New(func(e *stream.Emitter[kit.Event]) (kit.ModelResponse, error) {
		for _, event := range events {
			if err := e.Emit(event); err != nil {
				return resp, nil
			}
		}

		return resp, nil
	}), nil
}

func (model *Model) next(req kit.ModelRequest) ModelTurn {
	model.t.Helper()

	idx := model.idx
	if idx >= len(model.turns) {
		model.t.Fatalf("kittest.Model: no more queued turns (call %d)", idx+1)
	}

	model.idx++
	model.calls = append(model.calls, cloneModelRequest(req))

	return model.turns[idx]
}

// Assertion helpers

// CallCount returns the number of times the model was called.
func (model *Model) CallCount() int {
	return len(model.calls)
}

// CallAt returns the request from the call at the given index.
func (model *Model) CallAt(index int) kit.ModelRequest {
	model.t.Helper()

	if index >= len(model.calls) {
		model.t.Fatalf("kittest.Model: call index %d out of range (got %d calls)", index, len(model.calls))
	}

	return cloneModelRequest(model.calls[index])
}

// AssertCallCount fails the test if the model was not called exactly n times.
func (model *Model) AssertCallCount(t *testing.T, expected int) {
	t.Helper()

	if len(model.calls) != expected {
		t.Errorf("model call count = %d, want %d", len(model.calls), expected)
	}
}

// AssertToolCalled fails the test if none of the model's requests included
// a tool with the given name.
func (model *Model) AssertToolCalled(t *testing.T, name string) {
	t.Helper()

	for _, req := range model.calls {
		for _, tool := range req.Tools {
			if tool.Name == name {
				return
			}
		}
	}

	t.Errorf("model was never called with tool %q", name)
}

package kit

import "iter"

// Response is the result of a full agent run (one or more LLM turns).
type Response struct {
	// Messages are all new messages produced during this run.
	Messages []Message

	// Usage is the token consumption accumulated across all turns.
	Usage Usage
}

// LastMessage returns the final assistant message, or a zero Message if
// the response is empty.
func (r Response) LastMessage() Message {
	for i := len(r.Messages) - 1; i >= 0; i-- {
		if r.Messages[i].Role == MessageRoleAssistant {
			return r.Messages[i]
		}
	}

	return Message{}
}

// Stream is returned by [Agent.Stream]. It holds a lazy iterator; execution
// begins when the caller first ranges over Iter.
type Stream struct {
	iter     iter.Seq2[Event, error]
	response Response
	err      error
	done     bool
}

// NewStream constructs a Stream from fn. fn is invoked lazily on first
// iteration; it should yield events and return the accumulated Response.
func NewStream(fn func(yield func(Event, error) bool) Response) *Stream {
	s := &Stream{}
	s.iter = func(yield func(Event, error) bool) {
		s.response = fn(yield)
	}

	return s
}

// Iter returns an iterator over the events produced by the agent.
func (s *Stream) Iter() iter.Seq2[Event, error] {
	return func(yield func(Event, error) bool) {
		defer func() { s.done = true }()

		for event, err := range s.iter {
			if err != nil {
				s.err = err
			}

			if !yield(event, err) {
				return
			}
		}
	}
}

// Result returns the assembled Response and any error that occurred during
// the run. If Iter has not been exhausted, it drains the remaining events first.
func (s *Stream) Result() (Response, error) {
	if !s.done {
		for range s.Iter() { //nolint:revive // drain remaining events
		}
	}

	return s.response, s.err
}

// EventType classifies events emitted during a streamed agent run.
type EventType string

const (
	EventThinkingDelta  EventType = "thinking_delta"
	EventTextDelta      EventType = "text_delta"
	EventToolCall       EventType = "tool_call"
	EventToolResult     EventType = "tool_result"
	EventContextSummary EventType = "context_summary"
	EventMessage        EventType = "message"
)

// Event is a single item emitted by [Stream.Iter].
type Event struct {
	Type       EventType
	Text       string
	ToolCall   ToolCall
	ToolResult ToolResult
	Summary    string
	Message    Message
}

// NewTextDeltaEvent returns a text delta event with the given text.
func NewTextDeltaEvent(text string) Event {
	return Event{Type: EventTextDelta, Text: text}
}

// NewThinkingDeltaEvent returns a thinking delta event with the given text.
func NewThinkingDeltaEvent(text string) Event {
	return Event{Type: EventThinkingDelta, Text: text}
}

// NewToolCallEvent returns a tool call event for the given tool call.
func NewToolCallEvent(tc ToolCall) Event {
	return Event{Type: EventToolCall, ToolCall: tc}
}

// NewToolResultEvent returns a tool result event for the given tool result.
func NewToolResultEvent(tr ToolResult) Event {
	return Event{Type: EventToolResult, ToolResult: tr}
}

// NewContextSummaryEvent returns an event indicating that the conversation
// history was compacted. The summary contains the condensed conversation text.
func NewContextSummaryEvent(summary string) Event {
	return Event{Type: EventContextSummary, Summary: summary}
}

// NewMessageEvent returns an event carrying a complete assembled message.
func NewMessageEvent(msg Message) Event {
	return Event{Type: EventMessage, Message: msg}
}

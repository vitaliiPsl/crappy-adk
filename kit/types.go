package kit

// Result is the result of a full agent run (one or more LLM turns).
type Result struct {
	// Messages are all new messages produced during this run.
	Messages []Message

	// Usage is the token consumption accumulated across all model calls.
	Usage Usage

	// LastUsage is the token consumption of the final model call.
	LastUsage Usage
}

// LastMessage returns the final assistant message, or a zero Message if
// the result is empty.
func (r Result) LastMessage() Message {
	for i := len(r.Messages) - 1; i >= 0; i-- {
		if r.Messages[i].Role == MessageRoleAssistant {
			return r.Messages[i]
		}
	}

	return Message{}
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

package kit

// EventType identifies the lifecycle stage represented by a stream event.
type EventType string

const (
	EventContentStarted EventType = "content_started"
	EventContentDelta   EventType = "content_delta"
	EventContentDone    EventType = "content_done"
	EventMessage        EventType = "message"
)

// ModelEvent is emitted by a model stream while a model message is produced.
type ModelEvent struct {
	Type EventType

	Content *Content
}

// AgentEvent is emitted by an agent stream while an agent run progresses.
type AgentEvent struct {
	Type EventType

	Message    *Message
	Content    *Content
	ToolResult *ToolResult
}

// NewModelContentStartedEvent creates a model event for the start of content.
func NewModelContentStartedEvent(content Content) ModelEvent {
	return ModelEvent{
		Type:    EventContentStarted,
		Content: &content,
	}
}

// NewModelContentDeltaEvent creates a model event for a content delta.
func NewModelContentDeltaEvent(content Content) ModelEvent {
	return ModelEvent{
		Type:    EventContentDelta,
		Content: &content,
	}
}

// NewModelContentDoneEvent creates a model event for completed content.
func NewModelContentDoneEvent(content Content) ModelEvent {
	return ModelEvent{
		Type:    EventContentDone,
		Content: &content,
	}
}

// AgentEventFromModel converts a model content event into an agent event.
// Returns false if the event type is not a content event or has no content.
func AgentEventFromModel(event ModelEvent) (AgentEvent, bool) {
	if event.Content == nil {
		return AgentEvent{}, false
	}

	switch event.Type {
	case EventContentStarted:
		return NewAgentContentStartedEvent(*event.Content), true
	case EventContentDelta:
		return NewAgentContentDeltaEvent(*event.Content), true
	case EventContentDone:
		return NewAgentContentDoneEvent(*event.Content), true
	default:
		return AgentEvent{}, false
	}
}

// NewAgentContentStartedEvent creates an agent event for the start of content.
func NewAgentContentStartedEvent(content Content) AgentEvent {
	return AgentEvent{
		Type:    EventContentStarted,
		Content: &content,
	}
}

// NewAgentContentDeltaEvent creates an agent event for a content delta.
func NewAgentContentDeltaEvent(content Content) AgentEvent {
	return AgentEvent{
		Type:    EventContentDelta,
		Content: &content,
	}
}

// NewAgentContentDoneEvent creates an agent event for completed content.
func NewAgentContentDoneEvent(content Content) AgentEvent {
	return AgentEvent{
		Type:    EventContentDone,
		Content: &content,
	}
}

// NewAgentMessageEvent creates an agent event for a completed message.
func NewAgentMessageEvent(message Message) AgentEvent {
	return AgentEvent{
		Type:    EventMessage,
		Message: &message,
	}
}

// NewAgentToolResultEvent creates an agent event for a completed tool result.
func NewAgentToolResultEvent(result ToolResult) AgentEvent {
	content := NewToolResultContent(result)

	return AgentEvent{
		Type:       EventContentDone,
		Content:    &content,
		ToolResult: &result,
	}
}

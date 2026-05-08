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
	Message *Message
}

// AgentEvent is emitted by an agent stream while an agent run progresses.
type AgentEvent struct {
	Type EventType

	Model      *ModelEvent
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

// NewModelMessageEvent creates a model event for a completed model message.
func NewModelMessageEvent(message Message) ModelEvent {
	return ModelEvent{
		Type:    EventMessage,
		Message: &message,
	}
}

// NewAgentModelEvent promotes a model event into an agent event.
func NewAgentModelEvent(event ModelEvent) AgentEvent {
	eventCopy := event

	return AgentEvent{
		Type:  event.Type,
		Model: &eventCopy,
	}
}

// NewAgentMessageEvent creates an agent event for a completed agent-owned message.
func NewAgentMessageEvent(message Message) AgentEvent {
	return AgentEvent{
		Type:    EventMessage,
		Message: &message,
	}
}

// NewAgentContentDoneEvent creates an agent event for completed agent-owned content.
func NewAgentContentDoneEvent(content Content) AgentEvent {
	return AgentEvent{
		Type:    EventContentDone,
		Content: &content,
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

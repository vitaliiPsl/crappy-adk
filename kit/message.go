package kit

// MessageRole identifies who sent a message.
type MessageRole string

const (
	MessageRoleUser      MessageRole = "user"
	MessageRoleTool      MessageRole = "tool"
	MessageRoleAssistant MessageRole = "assistant"
)

// Message is a single entry in the conversation history.
type Message struct {
	// Who sent this message.
	Role MessageRole

	// The text content of the message. TODO: extend this to support more than just text.
	Content string

	// Name of the tool that produced this message. Set on tool messages.
	ToolName string
	// ID of the tool call this message is a result for. Set on tool messages.
	ToolCallID string

	// Tool calls requested by the model. Set on assistant messages.
	ToolCalls []ToolCall
}

type ToolCall struct {
	// Unique identifier for this call, used to match results back to the model.
	ID string
	// Name of the tool to execute.
	Name string
	// Arguments parsed from the model's response.
	Arguments map[string]any
}

// NewUserMessage creates a user message with the given content.
func NewUserMessage(content string) Message {
	return Message{
		Role:    MessageRoleUser,
		Content: content,
	}
}

// NewAssistantMessage creates a complete assistant message.
func NewAssistantMessage(content string, toolCalls []ToolCall) Message {
	return Message{
		Role:      MessageRoleAssistant,
		Content:   content,
		ToolCalls: toolCalls,
	}
}

// NewToolMessage creates a tool result message for the given tool call.
func NewToolMessage(content string, toolCall ToolCall) Message {
	return Message{
		Role:       MessageRoleTool,
		Content:    content,
		ToolCallID: toolCall.ID,
	}
}

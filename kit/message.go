package kit

// Role represents the role of a message sender, either user, tool, or model.
type Role string

const (
	RoleUser  Role = "user"
	RoleTool  Role = "tool"
	RoleModel Role = "model"
)

// Message is a single entry in the conversation history.
type Message struct {
	// Role indicates who sent the message.
	Role Role
	// Content is the text of the message.
	Content []Content
}

// NewUserMessage creates a new user message with the given content.
func NewUserMessage(content []Content) Message {
	return Message{
		Role:    RoleUser,
		Content: content,
	}
}

// NewModelMessage creates a new model message with the given content.
func NewModelMessage(content []Content) Message {
	return Message{
		Role:    RoleModel,
		Content: content,
	}
}

// NewToolMessage creates a new tool message with the given content.
func NewToolMessage(content []Content) Message {
	return Message{
		Role:    RoleTool,
		Content: content,
	}
}

// ContentType identifies the kind of content in a [Content].
type ContentType string

const (
	ContentTypeText       ContentType = "text"
	ContentTypeThinking   ContentType = "thinking"
	ContentTypeToolCall   ContentType = "tool_call"
	ContentTypeToolResult ContentType = "tool_result"
)

// Content is a single typed piece of message content.
type Content struct {
	// Type indicates what kind of content this part carries
	Type ContentType

	Text     *Text
	Thinking *Thinking

	ToolCall   *ToolCall
	ToolResult *ToolResult
}

// Text represents a text content.
type Text struct {
	Text string
}

// Thinking represents a thought or internal reasoning process of the model.
type Thinking struct {
	// ID is a unique identifier for this thought.
	ID string
	// Text is the content of the thought, which may include reasoning, plans, or other internal dialogue.
	Text string
	// Signature is a cryptographic signature of the thought, used to verify its authenticity and integrity.
	Signature string
}

// NewTextContent creates a new [ContentTypeText] content with the given text.
func NewTextContent(text string) Content {
	return Content{
		Type: ContentTypeText,
		Text: &Text{Text: text},
	}
}

// NewThinkingContent creates a new [ContentTypeThinking] content with the given text.
func NewThinkingContent(id, text, signature string) Content {
	return Content{
		Type:     ContentTypeThinking,
		Thinking: &Thinking{ID: id, Text: text, Signature: signature},
	}
}

// NewToolCallContent creates a new [ContentTypeToolCall] content with the given parameters.
func NewToolCallContent(call ToolCall) Content {
	return Content{
		Type:     ContentTypeToolCall,
		ToolCall: &call,
	}
}

// NewToolResultContent creates a new [ToolResult] content for the given call, output, and error.
func NewToolResultContent(result ToolResult) Content {
	return Content{
		Type:       ContentTypeToolResult,
		ToolResult: &result,
	}
}

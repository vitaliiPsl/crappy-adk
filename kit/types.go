package kit

import "strings"

// MessageRole identifies who sent a message.
type MessageRole string

const (
	MessageRoleUser      MessageRole = "user"
	MessageRoleTool      MessageRole = "tool"
	MessageRoleAssistant MessageRole = "assistant"
)

// ContentType identifies the kind of content in a [ContentPart].
type ContentType string

const (
	// ContentTypeText is plain text content.
	ContentTypeText ContentType = "text"
	// ContentTypeImage is an image (JPEG, PNG, GIF, WebP).
	ContentTypeImage ContentType = "image"
	// ContentTypeDocument is a document such as a PDF.
	ContentTypeDocument ContentType = "document"
)

// ContentPart is a single typed piece of message content.
// A message may contain multiple parts of different types.
type ContentPart struct {
	// Type indicates what kind of content this part carries.
	Type ContentType `json:"type"`

	// Text holds the text value for ContentTypeText parts.
	Text string `json:"text,omitempty"`

	// MediaType is the MIME type for binary or URL-based parts
	// (e.g. "image/png", "application/pdf").
	MediaType string `json:"media_type,omitempty"`

	// Data holds raw bytes for inline binary content.
	// Mutually exclusive with URL.
	Data []byte `json:"data,omitempty"`

	// URL is a remote reference for URL-based content.
	// Mutually exclusive with Data.
	URL string `json:"url,omitempty"`
}

// NewTextPart creates a text content part.
func NewTextPart(text string) ContentPart {
	return ContentPart{Type: ContentTypeText, Text: text}
}

// NewImageURLPart creates an image content part from a remote URL.
func NewImageURLPart(url string) ContentPart {
	return ContentPart{Type: ContentTypeImage, URL: url}
}

// NewImageDataPart creates an image content part from raw bytes.
// mediaType must be a valid image MIME type (e.g. "image/png").
func NewImageDataPart(data []byte, mediaType string) ContentPart {
	return ContentPart{Type: ContentTypeImage, Data: data, MediaType: mediaType}
}

// NewDocumentURLPart creates a document content part from a remote URL.
func NewDocumentURLPart(url string) ContentPart {
	return ContentPart{Type: ContentTypeDocument, URL: url}
}

// NewDocumentDataPart creates a document content part from raw bytes.
// mediaType should be "application/pdf" or similar.
func NewDocumentDataPart(data []byte, mediaType string) ContentPart {
	return ContentPart{Type: ContentTypeDocument, Data: data, MediaType: mediaType}
}

// Message is a single entry in the conversation history.
type Message struct {
	// Who sent this message.
	Role MessageRole `json:"role"`

	// Content holds the parts that make up this message.
	// User messages may contain text, images, documents, etc.
	// Assistant and tool messages contain text only.
	Content []ContentPart `json:"content,omitempty"`

	// Thinking is the reasoning produced by the model when extended thinking is
	// enabled. Must be preserved in conversation history for thinking-enabled models.
	Thinking string `json:"thinking,omitempty"`

	// Name of the tool that produced this message. Set on tool messages.
	ToolName string `json:"tool_name,omitempty"`
	// ID of the tool call this message is a result for. Set on tool messages.
	ToolCallID string `json:"tool_call_id,omitempty"`

	// Tool calls requested by the model. Set on assistant messages.
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`

	// IsSummary marks this message as a compaction summary. When building
	// input for the next run, clients can use the last summary message as
	// the starting point — everything before it has been compacted away.
	IsSummary bool `json:"is_summary,omitempty"`
}

// Text returns the concatenation of all text parts in the message.
// This is a convenience accessor for callers that only need the text content.
func (m Message) Text() string {
	var out strings.Builder
	for _, p := range m.Content {
		if p.Type == ContentTypeText {
			out.WriteString(p.Text)
		}
	}

	return out.String()
}

type ToolCall struct {
	// Unique identifier for this call, used to match results back to the model.
	ID string `json:"id"`
	// Name of the tool to execute.
	Name string `json:"name"`
	// Arguments parsed from the model's response.
	Arguments map[string]any `json:"arguments,omitempty"`
	// Metadata holds provider-specific data that must be preserved across turns
	// (e.g. Gemini's thought_signature for thinking models).
	Metadata map[string]any `json:"metadata,omitempty"`
}

// NewUserMessage creates a user message with the given content parts.
func NewUserMessage(parts ...ContentPart) Message {
	return Message{
		Role:    MessageRoleUser,
		Content: parts,
	}
}

// NewAssistantMessage creates a complete assistant message.
func NewAssistantMessage(content, thinking string, toolCalls []ToolCall) Message {
	var parts []ContentPart
	if content != "" {
		parts = []ContentPart{NewTextPart(content)}
	}

	return Message{
		Role:      MessageRoleAssistant,
		Content:   parts,
		Thinking:  thinking,
		ToolCalls: toolCalls,
	}
}

// NewSummaryMessage creates a user message that carries a compaction summary.
// IsSummary is set to true so clients can identify the compaction boundary.
func NewSummaryMessage(summary string) Message {
	return Message{
		Role:      MessageRoleUser,
		Content:   []ContentPart{NewTextPart(summary)},
		IsSummary: true,
	}
}

// NewToolMessage creates a tool result message for the given tool call.
func NewToolMessage(content string, toolCall ToolCall) Message {
	return Message{
		Role:       MessageRoleTool,
		Content:    []ContentPart{NewTextPart(content)},
		ToolName:   toolCall.Name,
		ToolCallID: toolCall.ID,
	}
}

// Result is the result of a full agent run (one or more LLM turns).
type Result struct {
	// Output is the final output of the agent.
	Output ContentPart

	// Messages are all new messages produced during this run.
	Messages []Message

	// Usage is the token consumption accumulated across all model calls.
	Usage Usage

	// LastUsage is the token consumption of the final model call.
	LastUsage Usage
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

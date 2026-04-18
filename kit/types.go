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

	// ContentTypeThinking is an extended-thinking / reasoning block. The Text
	// field holds the reasoning text; Signature holds the provider-issued
	// cryptographic signature that must be passed back on subsequent turns
	// (e.g. Anthropic thinking signatures) to preserve the model's reasoning
	// chain during multi-turn tool use.
	ContentTypeThinking ContentType = "thinking"
	// ContentTypeRedactedThinking is an opaque reasoning block the provider
	// chose not to reveal. Data holds the provider-issued bytes that must be
	// echoed back verbatim on subsequent turns.
	ContentTypeRedactedThinking ContentType = "redacted_thinking"

	// ContentTypeToolCall is a model-requested tool call embedded in message
	// content so providers can preserve exact ordering and provider metadata.
	ContentTypeToolCall ContentType = "tool_call"
	// ContentTypeToolResult is the content form of a tool response.
	ContentTypeToolResult ContentType = "tool_result"
)

// ContentPart is a single typed piece of message content.
// A message may contain multiple parts of different types.
type ContentPart struct {
	// Type indicates what kind of content this part carries.
	Type ContentType `json:"type"`

	// ID is the provider-issued identifier for this content part when one is
	// available. Some providers require this exact ID to be preserved and sent
	// back on subsequent turns.
	ID string `json:"id,omitempty"`

	// Text holds the text value for ContentTypeText parts.
	Text string `json:"text,omitempty"`

	// Name identifies a tool for tool_call/tool_result parts.
	Name string `json:"name,omitempty"`

	// Arguments holds parsed tool-call arguments for tool_call parts.
	Arguments map[string]any `json:"arguments,omitempty"`

	// MediaType is the MIME type for binary or URL-based parts
	// (e.g. "image/png", "application/pdf").
	MediaType string `json:"media_type,omitempty"`

	// Data holds raw bytes for inline binary content.
	// Mutually exclusive with URL.
	Data []byte `json:"data,omitempty"`

	// URL is a remote reference for URL-based content.
	// Mutually exclusive with Data.
	URL string `json:"url,omitempty"`

	// Signature is provider-issued opaque carry-over data for this content part.
	// Must be preserved verbatim and returned to the provider on subsequent
	// turns when the provider exposes such a value.
	Signature string `json:"signature,omitempty"`
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

// NewThinkingPart creates a thinking content part. signature should be the
// provider-issued signature when one is available; pass "" if the provider
// does not supply one.
func NewThinkingPart(text, signature string) ContentPart {
	return ContentPart{Type: ContentTypeThinking, Text: text, Signature: signature}
}

// NewToolCallPart creates a tool-call content part from a tool call.
func NewToolCallPart(call ToolCall) ContentPart {
	return ContentPart{
		Type:      ContentTypeToolCall,
		ID:        call.ID,
		Name:      call.Name,
		Arguments: call.Arguments,
	}
}

// NewToolResultPart creates a tool-result content part from a tool call.
func NewToolResultPart(content string, toolCall ToolCall) ContentPart {
	return ContentPart{
		Type: ContentTypeToolResult,
		ID:   toolCall.ID,
		Name: toolCall.Name,
		Text: content,
	}
}

// NewRedactedThinkingPart creates an opaque thinking content part carrying
// provider-supplied encrypted data that must be echoed back verbatim.
func NewRedactedThinkingPart(data []byte) ContentPart {
	return ContentPart{Type: ContentTypeRedactedThinking, Data: data}
}

// Message is a single entry in the conversation history.
type Message struct {
	// Who sent this message.
	Role MessageRole `json:"role"`

	// Content holds the parts that make up this message.
	// User messages may contain text, images, documents, etc.
	// Assistant messages may additionally contain thinking parts that carry
	// reasoning text and a provider-issued signature that must be preserved
	// across turns for thinking-enabled models.
	Content []ContentPart `json:"content,omitempty"`

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
		if p.Type == ContentTypeText || p.Type == ContentTypeToolResult {
			out.WriteString(p.Text)
		}
	}

	return out.String()
}

// Thinking returns the concatenation of all thinking part texts in the message.
// Signatures and redacted-thinking parts are not included.
func (m Message) Thinking() string {
	var out strings.Builder
	for _, p := range m.Content {
		if p.Type == ContentTypeThinking {
			out.WriteString(p.Text)
		}
	}

	return out.String()
}

// ToolCalls returns tool calls embedded in message content.
func (m Message) ToolCalls() []ToolCall {
	var calls []ToolCall
	for _, p := range m.Content {
		if p.Type != ContentTypeToolCall {
			continue
		}

		calls = append(calls, ToolCall{
			ID:        p.ID,
			Name:      p.Name,
			Arguments: p.Arguments,
		})
	}

	return calls
}

// ToolResult returns the first embedded tool-result part, if present.
func (m Message) ToolResult() (ContentPart, bool) {
	for _, p := range m.Content {
		if p.Type == ContentTypeToolResult {
			return p, true
		}
	}

	return ContentPart{}, false
}

// NewUserMessage creates a user message with the given content parts.
func NewUserMessage(parts ...ContentPart) Message {
	return Message{
		Role:    MessageRoleUser,
		Content: parts,
	}
}

// NewAssistantMessage creates a complete assistant message. When thinking is
// non-empty a thinking part with no signature is prepended; providers that
// preserve signatures should build the Message directly with the signed part.
func NewAssistantMessage(content, thinking string, toolCalls []ToolCall) Message {
	var parts []ContentPart
	if thinking != "" {
		parts = append(parts, NewThinkingPart(thinking, ""))
	}

	if content != "" {
		parts = append(parts, NewTextPart(content))
	}

	for _, tc := range toolCalls {
		parts = append(parts, NewToolCallPart(tc))
	}

	return Message{
		Role:    MessageRoleAssistant,
		Content: parts,
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
		Role:    MessageRoleTool,
		Content: []ContentPart{NewToolResultPart(content, toolCall)},
	}
}

// Result is the result of a full agent run (one or more LLM turns).
type Result struct {
	// Output is the final output of the agent.
	Output ContentPart

	// StructuredOutput is the validated machine-readable final answer when a
	// response schema is configured on the agent.
	StructuredOutput *StructuredOutput

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
	EventContentPartStarted EventType = "content_part_started"
	EventContentPartDelta   EventType = "content_part_delta"
	EventContentPartDone    EventType = "content_part_done"
	EventCompactionDone     EventType = "compaction_done"
	EventMessage            EventType = "message"
)

// Event is a single item emitted by [Stream.Iter].
type Event struct {
	Type EventType

	ContentPartType ContentType
	ContentPart     *ContentPart
	Text            string
	Summary         string
	Message         Message
}

func NewContentPartStartedEvent(partType ContentType) Event {
	return Event{
		Type:            EventContentPartStarted,
		ContentPartType: partType,
	}
}

func NewContentPartDeltaEvent(partType ContentType, text string) Event {
	return Event{
		Type:            EventContentPartDelta,
		ContentPartType: partType,
		Text:            text,
	}
}

func NewContentPartDoneEvent(part ContentPart) Event {
	return Event{
		Type:            EventContentPartDone,
		ContentPartType: part.Type,
		ContentPart:     &part,
	}
}

func NewCompactionDoneEvent(summary string) Event {
	return Event{Type: EventCompactionDone, Summary: summary}
}

func NewMessageEvent(msg Message) Event {
	return Event{Type: EventMessage, Message: msg}
}

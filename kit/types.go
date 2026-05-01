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
	// ContentTypeSummary is a compaction summary carried as normal text to
	// providers, but identifiable by clients as a transcript boundary.
	ContentTypeSummary ContentType = "summary"

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

	Summary          *SummaryPart          `json:"summary,omitempty"`
	Thinking         *ThinkingPart         `json:"thinking,omitempty"`
	RedactedThinking *RedactedThinkingPart `json:"redacted_thinking,omitempty"`
	ToolCall         *ToolCallPart         `json:"tool_call,omitempty"`
	ToolResult       *ToolResultPart       `json:"tool_result,omitempty"`

	Text     *TextPart `json:"text,omitempty"`
	Image    *BlobPart `json:"image,omitempty"`
	Document *BlobPart `json:"document,omitempty"`
}

type SummaryPart struct {
	Text string `json:"text,omitempty"`
}

type ThinkingPart struct {
	ID        string `json:"id,omitempty"`
	Text      string `json:"text,omitempty"`
	Signature string `json:"signature,omitempty"`
}

type RedactedThinkingPart struct {
	Data []byte `json:"data,omitempty"`
}

type ToolCallPart struct {
	ID        string         `json:"id,omitempty"`
	Name      string         `json:"name,omitempty"`
	Arguments map[string]any `json:"arguments,omitempty"`
	Signature string         `json:"signature,omitempty"`
}

type ToolResultPart struct {
	ID    string `json:"id,omitempty"`
	Name  string `json:"name,omitempty"`
	Text  string `json:"text,omitempty"`
	Error string `json:"error,omitempty"`
}

type TextPart struct {
	Text string `json:"text,omitempty"`
}

type BlobPart struct {
	MediaType string `json:"media_type,omitempty"`
	Data      []byte `json:"data,omitempty"`
	URL       string `json:"url,omitempty"`
}

// NewTextPart creates a text content part.
func NewTextPart(text string) ContentPart {
	return ContentPart{Type: ContentTypeText, Text: &TextPart{Text: text}}
}

func NewSummaryPart(summary string) ContentPart {
	return ContentPart{Type: ContentTypeSummary, Summary: &SummaryPart{Text: summary}}
}

// NewImageURLPart creates an image content part from a remote URL.
func NewImageURLPart(url string) ContentPart {
	return ContentPart{Type: ContentTypeImage, Image: &BlobPart{URL: url}}
}

// NewImageDataPart creates an image content part from raw bytes.
// mediaType must be a valid image MIME type (e.g. "image/png").
func NewImageDataPart(data []byte, mediaType string) ContentPart {
	return ContentPart{Type: ContentTypeImage, Image: &BlobPart{Data: data, MediaType: mediaType}}
}

// NewDocumentURLPart creates a document content part from a remote URL.
func NewDocumentURLPart(url string) ContentPart {
	return ContentPart{Type: ContentTypeDocument, Document: &BlobPart{URL: url}}
}

// NewDocumentDataPart creates a document content part from raw bytes.
// mediaType should be "application/pdf" or similar.
func NewDocumentDataPart(data []byte, mediaType string) ContentPart {
	return ContentPart{Type: ContentTypeDocument, Document: &BlobPart{Data: data, MediaType: mediaType}}
}

// NewThinkingPart creates a thinking content part. signature should be the
// provider-issued signature when one is available; pass "" if the provider
// does not supply one.
func NewThinkingPart(text, signature string) ContentPart {
	return ContentPart{Type: ContentTypeThinking, Thinking: &ThinkingPart{Text: text, Signature: signature}}
}

// NewToolCallPart creates a tool-call content part from a tool call.
func NewToolCallPart(call ToolCall) ContentPart {
	return ContentPart{
		Type: ContentTypeToolCall,
		ToolCall: &ToolCallPart{
			ID:        call.ID,
			Name:      call.Name,
			Arguments: call.Arguments,
		},
	}
}

// NewToolResultPart creates a tool-result content part from a tool call.
func NewToolResultPart(content string, toolCall ToolCall) ContentPart {
	return ContentPart{
		Type: ContentTypeToolResult,
		ToolResult: &ToolResultPart{
			ID:   toolCall.ID,
			Name: toolCall.Name,
			Text: content,
		},
	}
}

// NewRedactedThinkingPart creates an opaque thinking content part carrying
// provider-supplied encrypted data that must be echoed back verbatim.
func NewRedactedThinkingPart(data []byte) ContentPart {
	return ContentPart{Type: ContentTypeRedactedThinking, RedactedThinking: &RedactedThinkingPart{Data: data}}
}

func (p ContentPart) TextValue() string {
	switch p.Type {
	case ContentTypeText:
		if p.Text != nil {
			return p.Text.Text
		}
	case ContentTypeSummary:
		if p.Summary != nil {
			return p.Summary.Text
		}
	case ContentTypeThinking:
		if p.Thinking != nil {
			return p.Thinking.Text
		}
	case ContentTypeToolResult:
		if p.ToolResult != nil {
			return p.ToolResult.Text
		}
	}

	return ""
}

func (p ContentPart) ToolCallValue() (ToolCall, bool) {
	if p.Type != ContentTypeToolCall || p.ToolCall == nil {
		return ToolCall{}, false
	}

	return ToolCall{
		ID:        p.ToolCall.ID,
		Name:      p.ToolCall.Name,
		Arguments: p.ToolCall.Arguments,
	}, true
}

func (p ContentPart) ToolResultValue() (ToolResultPart, bool) {
	if p.Type != ContentTypeToolResult || p.ToolResult == nil {
		return ToolResultPart{}, false
	}

	return *p.ToolResult, true
}

func (p ContentPart) BlobValue() (BlobPart, bool) {
	switch p.Type {
	case ContentTypeImage:
		if p.Image != nil {
			return *p.Image, true
		}
	case ContentTypeDocument:
		if p.Document != nil {
			return *p.Document, true
		}
	}

	return BlobPart{}, false
}

func (p ContentPart) ThinkingValue() (ThinkingPart, bool) {
	if p.Type != ContentTypeThinking || p.Thinking == nil {
		return ThinkingPart{}, false
	}

	return *p.Thinking, true
}

func (p ContentPart) RedactedThinkingValue() (RedactedThinkingPart, bool) {
	if p.Type != ContentTypeRedactedThinking || p.RedactedThinking == nil {
		return RedactedThinkingPart{}, false
	}

	return *p.RedactedThinking, true
}

// Message is a single entry in the conversation history.
type Message struct {
	// Who sent this message.
	Role MessageRole `json:"role"`

	// Content holds the parts that make up this message.
	Content []ContentPart `json:"content,omitempty"`
}

// Text returns the concatenation of all text parts in the message.
// This is a convenience accessor for callers that only need the text content.
func (m Message) Text() string {
	var out strings.Builder
	for _, p := range m.Content {
		if p.Type == ContentTypeText || p.Type == ContentTypeSummary || p.Type == ContentTypeToolResult {
			out.WriteString(p.TextValue())
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
			out.WriteString(p.TextValue())
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

		call, ok := p.ToolCallValue()
		if ok {
			calls = append(calls, call)
		}
	}

	return calls
}

// ToolResult returns the first embedded tool-result part, if present.
func (m Message) ToolResult() (ToolResultPart, bool) {
	for _, p := range m.Content {
		if result, ok := p.ToolResultValue(); ok {
			return result, true
		}
	}

	return ToolResultPart{}, false
}

// Summary returns the first summary part, if present.
func (m Message) Summary() (SummaryPart, bool) {
	for _, p := range m.Content {
		if p.Type == ContentTypeSummary {
			return *p.Summary, true
		}
	}

	return SummaryPart{}, false
}

// IsSummary returns true if the message contains a summary part.
func (m Message) IsSummary() bool {
	for _, p := range m.Content {
		if p.Type == ContentTypeSummary {
			return true
		}
	}

	return false
}

// Output returns the first user-facing content part in the message.
// Internal reasoning and tool-call parts are skipped.
func (m Message) Output() ContentPart {
	for _, p := range m.Content {
		switch p.Type {
		case ContentTypeThinking, ContentTypeRedactedThinking, ContentTypeToolCall:
			continue
		default:
			return p
		}
	}

	if len(m.Content) == 0 {
		return ContentPart{}
	}

	return m.Content[0]
}

// NewUserMessage creates a user message with the given content parts.
func NewUserMessage(parts ...ContentPart) Message {
	return Message{
		Role:    MessageRoleUser,
		Content: parts,
	}
}

// NewAssistantMessage creates an assistant message with the given content parts.
func NewAssistantMessage(parts ...ContentPart) Message {
	return Message{
		Role:    MessageRoleAssistant,
		Content: parts,
	}
}

// NewToolMessage creates a tool result message for the given tool call.
func NewToolMessage(content string, toolCall ToolCall) Message {
	return Message{
		Role:    MessageRoleTool,
		Content: []ContentPart{NewToolResultPart(content, toolCall)},
	}
}

// NewSummaryMessage creates a user message that carries a compaction summary.
func NewSummaryMessage(summary string) Message {
	return Message{
		Role:    MessageRoleUser,
		Content: []ContentPart{NewSummaryPart(summary)},
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

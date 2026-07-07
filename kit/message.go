package kit

import (
	"fmt"
	"strings"
)

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
	Role Role `json:"role"`
	// Content is the text of the message.
	Content []Content `json:"content,omitempty"`
}

// NewUserMessage creates a new user message with the given content.
func NewUserMessage(content ...Content) Message {
	return Message{
		Role:    RoleUser,
		Content: content,
	}
}

// NewModelMessage creates a new model message with the given content.
func NewModelMessage(content ...Content) Message {
	return Message{
		Role:    RoleModel,
		Content: content,
	}
}

// NewToolMessage creates a new tool message with the given content.
func NewToolMessage(content ...Content) Message {
	return Message{
		Role:    RoleTool,
		Content: content,
	}
}

// TextContent returns the first text block from the message content, or nil if none.
func (m Message) TextContent() *Text {
	for _, c := range m.Content {
		if c.Type == ContentTypeText && c.Text != nil {
			return c.Text
		}
	}

	return nil
}

// ThinkingContent returns the first thinking block from the message content, or nil.
func (m Message) ThinkingContent() *Thinking {
	for _, c := range m.Content {
		if c.Type == ContentTypeThinking && c.Thinking != nil {
			return c.Thinking
		}
	}

	return nil
}

// ToolCalls returns all tool call blocks from the message content.
func (m Message) ToolCalls() []ToolCall {
	var out []ToolCall
	for _, c := range m.Content {
		if c.Type == ContentTypeToolCall && c.ToolCall != nil {
			out = append(out, *c.ToolCall)
		}
	}

	return out
}

// ToolResults returns all tool result blocks from the message content.
func (m Message) ToolResults() []ToolResult {
	var out []ToolResult
	for _, c := range m.Content {
		if c.Type == ContentTypeToolResult && c.ToolResult != nil {
			out = append(out, *c.ToolResult)
		}
	}

	return out
}

// ContentType identifies the kind of content in a [Content].
type ContentType string

const (
	ContentTypeText       ContentType = "text"
	ContentTypeThinking   ContentType = "thinking"
	ContentTypeImage      ContentType = "image"
	ContentTypeAudio      ContentType = "audio"
	ContentTypeResource   ContentType = "resource"
	ContentTypeToolCall   ContentType = "tool_call"
	ContentTypeToolResult ContentType = "tool_result"
	ContentTypeSummary    ContentType = "summary"
)

// Content is a single typed piece of message content.
type Content struct {
	// Type indicates what kind of content this part carries
	Type ContentType `json:"type"`

	Text     *Text     `json:"text,omitempty"`
	Thinking *Thinking `json:"thinking,omitempty"`

	Image    *Image    `json:"image,omitempty"`
	Audio    *Audio    `json:"audio,omitempty"`
	Resource *Resource `json:"resource,omitempty"`

	ToolCall   *ToolCall   `json:"tool_call,omitempty"`
	ToolResult *ToolResult `json:"tool_result,omitempty"`

	Summary *Summary `json:"summary,omitempty"`
}

// Text represents a text content.
type Text struct {
	Text string `json:"text,omitempty"`
}

// Image represents image content. Data is raw bytes and is JSON-encoded as base64.
type Image struct {
	MIMEType string `json:"mime_type,omitempty"`
	Data     []byte `json:"data,omitempty"`
	URI      string `json:"uri,omitempty"`
}

// Audio represents audio content. Data is raw bytes and is JSON-encoded as base64.
type Audio struct {
	MIMEType string `json:"mime_type,omitempty"`
	Data     []byte `json:"data,omitempty"`
	URI      string `json:"uri,omitempty"`
}

// Resource represents a referenced or embedded resource.
type Resource struct {
	URI         string `json:"uri,omitempty"`
	Name        string `json:"name,omitempty"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	MIMEType    string `json:"mime_type,omitempty"`
	Text        string `json:"text,omitempty"`
	Blob        []byte `json:"blob,omitempty"`
}

// Summary represents a summary that stands in for a range of older
// conversation messages.
type Summary struct {
	// Text is the natural-language summary of the replaced messages.
	Text string `json:"text,omitempty"`
}

// Thinking represents a thought or internal reasoning process of the model.
type Thinking struct {
	// ID is a unique identifier for this thought.
	ID string `json:"id,omitempty"`
	// Text is the content of the thought, which may include reasoning, plans, or other internal dialogue.
	Text string `json:"text,omitempty"`
	// Signature is a cryptographic signature of the thought, used to verify its authenticity and integrity.
	Signature string `json:"signature,omitempty"`
}

// NewTextContent creates a new [ContentTypeText] content with the given text.
func NewTextContent(text string) Content {
	return Content{
		Type: ContentTypeText,
		Text: &Text{Text: text},
	}
}

// NewImageContent creates a new [ContentTypeImage] content with inline bytes.
func NewImageContent(mimeType string, data []byte) Content {
	return Content{
		Type:  ContentTypeImage,
		Image: &Image{MIMEType: mimeType, Data: append([]byte(nil), data...)},
	}
}

// NewImageURLContent creates a new [ContentTypeImage] content with a URI.
func NewImageURLContent(mimeType, uri string) Content {
	return Content{
		Type:  ContentTypeImage,
		Image: &Image{MIMEType: mimeType, URI: uri},
	}
}

// NewAudioContent creates a new [ContentTypeAudio] content with inline bytes.
func NewAudioContent(mimeType string, data []byte) Content {
	return Content{
		Type:  ContentTypeAudio,
		Audio: &Audio{MIMEType: mimeType, Data: append([]byte(nil), data...)},
	}
}

// NewAudioURLContent creates a new [ContentTypeAudio] content with a URI.
func NewAudioURLContent(mimeType, uri string) Content {
	return Content{
		Type:  ContentTypeAudio,
		Audio: &Audio{MIMEType: mimeType, URI: uri},
	}
}

// NewResourceContent creates a new [ContentTypeResource] content.
func NewResourceContent(resource Resource) Content {
	resource.Blob = append([]byte(nil), resource.Blob...)

	return Content{
		Type:     ContentTypeResource,
		Resource: &resource,
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

// NewSummaryContent creates a new [ContentTypeSummary] content with the given text.
func NewSummaryContent(text string) Content {
	return Content{
		Type:    ContentTypeSummary,
		Summary: &Summary{Text: text},
	}
}

// ContentsTextFallback joins text representations of content blocks.
func ContentsTextFallback(content []Content) string {
	parts := make([]string, 0, len(content))
	for _, item := range content {
		text, ok := ContentTextFallback(item)
		if !ok || text == "" {
			continue
		}

		parts = append(parts, text)
	}

	return strings.Join(parts, "\n")
}

// ContentTextFallback returns a text representation of a content block when a
// provider or subsystem cannot handle its richer form directly.
func ContentTextFallback(content Content) (string, bool) {
	switch content.Type {
	case ContentTypeText:
		if content.Text == nil {
			return "", false
		}

		return content.Text.Text, true
	case ContentTypeSummary:
		if content.Summary == nil {
			return "", false
		}

		return content.Summary.Text, true
	case ContentTypeImage:
		if content.Image == nil {
			return "", false
		}

		return mediaFallback("image", content.Image.MIMEType, content.Image.URI, len(content.Image.Data)), true
	case ContentTypeAudio:
		if content.Audio == nil {
			return "", false
		}

		return mediaFallback("audio", content.Audio.MIMEType, content.Audio.URI, len(content.Audio.Data)), true
	case ContentTypeResource:
		if content.Resource == nil {
			return "", false
		}

		return resourceFallback(content.Resource), true
	default:
		return "", false
	}
}

func mediaFallback(kind, mimeType, uri string, size int) string {
	switch {
	case uri != "" && mimeType != "":
		return fmt.Sprintf("[%s: %s, %s]", kind, uri, mimeType)
	case uri != "":
		return fmt.Sprintf("[%s: %s]", kind, uri)
	case mimeType != "":
		return fmt.Sprintf("[%s: %s, %d bytes]", kind, mimeType, size)
	default:
		return fmt.Sprintf("[%s: %d bytes]", kind, size)
	}
}

func resourceFallback(resource *Resource) string {
	if resource.Text != "" {
		return resource.Text
	}

	switch {
	case resource.URI != "" && resource.MIMEType != "":
		return fmt.Sprintf("[resource: %s, %s, %d bytes]", resource.URI, resource.MIMEType, len(resource.Blob))
	case resource.URI != "":
		return fmt.Sprintf("[resource: %s, %d bytes]", resource.URI, len(resource.Blob))
	case resource.MIMEType != "":
		return fmt.Sprintf("[resource: %s, %d bytes]", resource.MIMEType, len(resource.Blob))
	default:
		return fmt.Sprintf("[resource: %d bytes]", len(resource.Blob))
	}
}

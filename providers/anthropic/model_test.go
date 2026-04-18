package anthropic

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	anthropicapi "github.com/anthropics/anthropic-sdk-go"
	"github.com/google/jsonschema-go/jsonschema"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

func TestNew_AllowsUnknownModelIDs(t *testing.T) {
	model, err := New("", "claude-compatible")
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	cfg := model.Config()
	if cfg.ID != "claude-compatible" {
		t.Fatalf("config.ID = %q, want %q", cfg.ID, "claude-compatible")
	}

	if cfg.Provider != ProviderID {
		t.Fatalf("config.Provider = %q, want %q", cfg.Provider, ProviderID)
	}
}

func TestNew_PreservesKnownModelMetadata(t *testing.T) {
	model, err := New("", "claude-haiku-4-5")
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	cfg := model.Config()
	if cfg.OutputLimit == 0 {
		t.Fatal("expected known model metadata to be preserved")
	}

	if !cfg.Capabilities.Tools {
		t.Fatal("expected known model capabilities to be preserved")
	}
}

func TestConvertContentPart_Text(t *testing.T) {
	part, ok := convertUserContentPart(kit.NewTextPart("hello"))
	if !ok {
		t.Fatal("convertContentPart returned ok=false")
	}

	text := part.OfText
	if text == nil {
		t.Fatal("expected text block")
	}

	if text.Text != "hello" {
		t.Fatalf("text = %q, want %q", text.Text, "hello")
	}
}

func TestConvertContentPart_ImageData(t *testing.T) {
	part, ok := convertUserContentPart(kit.NewImageDataPart([]byte("png-bytes"), "image/png"))
	if !ok {
		t.Fatal("convertContentPart returned ok=false")
	}

	image := part.OfImage
	if image == nil {
		t.Fatal("expected image block")
	}

	source := image.Source.OfBase64
	if source == nil {
		t.Fatal("expected base64 image source")
	}

	want := base64.StdEncoding.EncodeToString([]byte("png-bytes"))
	if source.Data != want {
		t.Fatalf("image data = %q, want %q", source.Data, want)
	}
}

func TestConvertContentPart_ImageURL(t *testing.T) {
	part, ok := convertUserContentPart(kit.NewImageURLPart("https://example.com/image.png"))
	if !ok {
		t.Fatal("convertContentPart returned ok=false")
	}

	image := part.OfImage
	if image == nil {
		t.Fatal("expected image block")
	}

	source := image.Source.OfURL
	if source == nil {
		t.Fatal("expected URL image source")
	}

	if source.URL != "https://example.com/image.png" {
		t.Fatalf("url = %q, want %q", source.URL, "https://example.com/image.png")
	}
}

func TestConvertContentPart_DocumentData(t *testing.T) {
	part, ok := convertUserContentPart(kit.NewDocumentDataPart([]byte("%PDF-1.7"), "application/pdf"))
	if !ok {
		t.Fatal("convertContentPart returned ok=false")
	}

	document := part.OfDocument
	if document == nil {
		t.Fatal("expected document block")
	}

	source := document.Source.OfBase64
	if source == nil {
		t.Fatal("expected base64 document source")
	}

	want := base64.StdEncoding.EncodeToString([]byte("%PDF-1.7"))
	if source.Data != want {
		t.Fatalf("document data = %q, want %q", source.Data, want)
	}
}

func TestConvertContentPart_DocumentURL(t *testing.T) {
	part, ok := convertUserContentPart(kit.NewDocumentURLPart("https://example.com/files/spec.pdf"))
	if !ok {
		t.Fatal("convertContentPart returned ok=false")
	}

	document := part.OfDocument
	if document == nil {
		t.Fatal("expected document block")
	}

	source := document.Source.OfURL
	if source == nil {
		t.Fatal("expected URL document source")
	}

	if source.URL != "https://example.com/files/spec.pdf" {
		t.Fatalf("url = %q, want %q", source.URL, "https://example.com/files/spec.pdf")
	}
}

func TestConvertAssistantMessage_PreservesThinkingAndToolCalls(t *testing.T) {
	msg := convertAssistantMessage(kit.Message{
		Role: kit.MessageRoleAssistant,
		Content: []kit.ContentPart{
			kit.NewThinkingPart("internal reasoning", ""),
			kit.NewTextPart("done"),
			kit.NewToolCallPart(kit.ToolCall{
				ID:        "call_1",
				Name:      "search",
				Arguments: map[string]any{"query": "crappy"},
			}),
		},
	})

	if msg.Role != anthropicapi.MessageParamRoleAssistant {
		t.Fatalf("role = %q, want %q", msg.Role, anthropicapi.MessageParamRoleAssistant)
	}

	if got := len(msg.Content); got != 3 {
		t.Fatalf("len(content) = %d, want 3", got)
	}

	if msg.Content[0].OfThinking == nil {
		t.Fatal("expected first content block to be thinking")
	}

	if msg.Content[1].OfText == nil || msg.Content[1].OfText.Text != "done" {
		t.Fatal("expected second content block to be assistant text")
	}

	if msg.Content[2].OfToolUse == nil {
		t.Fatal("expected third content block to be tool call")
	}

	if msg.Content[2].OfToolUse.Name != "search" {
		t.Fatalf("tool name = %q, want %q", msg.Content[2].OfToolUse.Name, "search")
	}
}

func TestConvertResponse_PreservesThinkingToolCallsAndUsage(t *testing.T) {
	raw := []byte(`{
		"stop_reason":"tool_use",
		"content":[
			{"type":"thinking","signature":"sig","thinking":"chain of thought"},
			{"type":"text","text":"final text","citations":[]},
			{"type":"tool_use","id":"call_1","name":"read_file","input":{"path":"README.md"}}
		],
		"usage":{
			"input_tokens":11,
			"output_tokens":7,
			"cache_read_input_tokens":3,
			"cache_creation_input_tokens":2
		}
	}`)

	var message anthropicapi.Message

	err := json.Unmarshal(raw, &message)
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	resp := convertResponse(&message)

	if resp.Message.Text() != "final text" {
		t.Fatalf("text = %q, want %q", resp.Message.Text(), "final text")
	}

	if resp.Message.Thinking() != "chain of thought" {
		t.Fatalf("thinking = %q, want %q", resp.Message.Thinking(), "chain of thought")
	}

	if resp.FinishReason != kit.FinishReasonToolCall {
		t.Fatalf("finish reason = %q, want %q", resp.FinishReason, kit.FinishReasonToolCall)
	}

	if got := len(resp.Message.ToolCalls()); got != 1 {
		t.Fatalf("len(tool_calls) = %d, want 1", got)
	}

	if resp.Message.ToolCalls()[0].Name != "read_file" {
		t.Fatalf("tool name = %q, want %q", resp.Message.ToolCalls()[0].Name, "read_file")
	}

	if resp.Usage.InputTokens != 11 || resp.Usage.OutputTokens != 7 || resp.Usage.CacheReadTokens != 3 || resp.Usage.CacheWriteTokens != 2 {
		t.Fatalf("usage = %+v", resp.Usage)
	}
}

func TestBuildParams_IncludesStructuredOutputSchema(t *testing.T) {
	params, err := buildParams(kit.ModelRequest{
		ResponseSchema: &jsonschema.Schema{
			Type:     "object",
			Required: []string{"answer"},
			Properties: map[string]*jsonschema.Schema{
				"answer": {Type: "string"},
			},
		},
	}, kit.ModelConfig{ID: "claude-sonnet-4-5"})
	if err != nil {
		t.Fatalf("buildParams returned error: %v", err)
	}

	if params.OutputConfig.Format.Schema == nil {
		t.Fatal("expected output schema to be set")
	}

	if got := params.OutputConfig.Format.Schema["type"]; got != "object" {
		t.Fatalf("schema type = %v, want %q", got, "object")
	}
}

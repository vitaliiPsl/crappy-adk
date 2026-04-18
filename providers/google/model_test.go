package google

import (
	"encoding/json"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"google.golang.org/genai"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

func TestNew_AllowsUnknownModelIDs(t *testing.T) {
	model, err := New("test-key", "gemini-compatible")
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	cfg := model.Config()
	if cfg.ID != "gemini-compatible" {
		t.Fatalf("config.ID = %q, want %q", cfg.ID, "gemini-compatible")
	}

	if cfg.Provider != ProviderID {
		t.Fatalf("config.Provider = %q, want %q", cfg.Provider, ProviderID)
	}
}

func TestNew_PreservesKnownModelMetadata(t *testing.T) {
	model, err := NewWithConfig("test-key", kit.ModelConfig{
		ID:          "gemini-2.5-flash",
		Provider:    ProviderID,
		OutputLimit: 65_536,
		Capabilities: kit.ModelCapabilities{
			Tools: true,
		},
	})
	if err != nil {
		t.Fatalf("NewWithConfig: %v", err)
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
	part := convertUserContentPart(kit.NewTextPart("hello"))
	if part == nil {
		t.Fatal("expected text part")
	}

	if part.Text != "hello" {
		t.Fatalf("text = %q, want %q", part.Text, "hello")
	}
}

func TestConvertContentPart_ImageData(t *testing.T) {
	part := convertUserContentPart(kit.NewImageDataPart([]byte("png-bytes"), "image/png"))
	if part == nil {
		t.Fatal("expected image part")
	}

	if part.InlineData == nil {
		t.Fatal("expected inline data")
	}

	if string(part.InlineData.Data) != "png-bytes" {
		t.Fatalf("data = %q, want %q", string(part.InlineData.Data), "png-bytes")
	}

	if part.InlineData.MIMEType != "image/png" {
		t.Fatalf("mime type = %q, want %q", part.InlineData.MIMEType, "image/png")
	}
}

func TestConvertContentPart_ImageURL(t *testing.T) {
	part := convertUserContentPart(kit.ContentPart{
		Type:      kit.ContentTypeImage,
		URL:       "https://example.com/image.png",
		MediaType: "image/png",
	})
	if part == nil {
		t.Fatal("expected image URL part")
	}

	if part.FileData == nil {
		t.Fatal("expected file data")
	}

	if part.FileData.FileURI != "https://example.com/image.png" {
		t.Fatalf("file uri = %q, want %q", part.FileData.FileURI, "https://example.com/image.png")
	}

	if part.FileData.MIMEType != "image/png" {
		t.Fatalf("mime type = %q, want %q", part.FileData.MIMEType, "image/png")
	}
}

func TestConvertContentPart_DocumentData(t *testing.T) {
	part := convertUserContentPart(kit.NewDocumentDataPart([]byte("%PDF-1.7"), "application/pdf"))
	if part == nil {
		t.Fatal("expected document part")
	}

	if part.InlineData == nil {
		t.Fatal("expected inline data")
	}

	if string(part.InlineData.Data) != "%PDF-1.7" {
		t.Fatalf("data = %q, want %q", string(part.InlineData.Data), "%PDF-1.7")
	}

	if part.InlineData.MIMEType != "application/pdf" {
		t.Fatalf("mime type = %q, want %q", part.InlineData.MIMEType, "application/pdf")
	}
}

func TestConvertContentPart_DocumentURL(t *testing.T) {
	part := convertUserContentPart(kit.ContentPart{
		Type:      kit.ContentTypeDocument,
		URL:       "https://example.com/files/spec.pdf",
		MediaType: "application/pdf",
	})
	if part == nil {
		t.Fatal("expected document URL part")
	}

	if part.FileData == nil {
		t.Fatal("expected file data")
	}

	if part.FileData.FileURI != "https://example.com/files/spec.pdf" {
		t.Fatalf("file uri = %q, want %q", part.FileData.FileURI, "https://example.com/files/spec.pdf")
	}

	if part.FileData.MIMEType != "application/pdf" {
		t.Fatalf("mime type = %q, want %q", part.FileData.MIMEType, "application/pdf")
	}
}

func TestConvertAssistantMessage_PreservesToolMetadata(t *testing.T) {
	msg := convertAssistantMessage(kit.Message{
		Role: kit.MessageRoleAssistant,
		Content: []kit.ContentPart{
			kit.NewThinkingPart("chain", encodeSignature([]byte("sig-part"))),
			{
				Type: kit.ContentTypeText,
				Text: "done",
			},
			{
				Type:      kit.ContentTypeToolCall,
				ID:        "call_1",
				Name:      "search",
				Arguments: map[string]any{"query": "crappy"},
				Signature: encodeSignature([]byte("sig")),
			},
		},
	})

	if msg.Role != genai.RoleModel {
		t.Fatalf("role = %q, want %q", msg.Role, genai.RoleModel)
	}

	if got := len(msg.Parts); got != 3 {
		t.Fatalf("len(parts) = %d, want 3", got)
	}

	if !msg.Parts[0].Thought || msg.Parts[0].Text != "chain" {
		t.Fatalf("thinking part = %+v", msg.Parts[0])
	}

	if string(msg.Parts[0].ThoughtSignature) != "sig-part" {
		t.Fatalf("thinking signature = %q, want %q", string(msg.Parts[0].ThoughtSignature), "sig-part")
	}

	if msg.Parts[1].Text != "done" {
		t.Fatalf("text = %q, want %q", msg.Parts[1].Text, "done")
	}

	call := msg.Parts[2].FunctionCall
	if call == nil {
		t.Fatal("expected function call part")
	}

	if call.ID != "call_1" {
		t.Fatalf("tool id = %q, want %q", call.ID, "call_1")
	}

	if call.Name != "search" {
		t.Fatalf("tool name = %q, want %q", call.Name, "search")
	}

	if string(msg.Parts[2].ThoughtSignature) != "sig" {
		t.Fatalf("thought signature = %q, want %q", string(msg.Parts[2].ThoughtSignature), "sig")
	}
}

func TestConvertResponse_PreservesThinkingToolCallsAndUsage(t *testing.T) {
	resp := convertResponse(&genai.GenerateContentResponse{
		Candidates: []*genai.Candidate{{
			FinishReason: genai.FinishReasonStop,
			Content: &genai.Content{
				Parts: []*genai.Part{
					{Text: "chain", Thought: true, ThoughtSignature: []byte("part-sig")},
					{Text: "final"},
					{
						InlineData: &genai.Blob{Data: []byte("img"), MIMEType: "image/png"},
					},
					{
						FunctionCall: &genai.FunctionCall{
							ID:   "call_9",
							Name: "read_file",
							Args: map[string]any{"path": "README.md"},
						},
						ThoughtSignature: []byte("sig"),
					},
				},
			},
		}},
		UsageMetadata: &genai.GenerateContentResponseUsageMetadata{
			PromptTokenCount:        9,
			CandidatesTokenCount:    4,
			CachedContentTokenCount: 2,
			ThoughtsTokenCount:      6,
		},
	})

	if resp.Message.Text() != "final" {
		t.Fatalf("text = %q, want %q", resp.Message.Text(), "final")
	}

	if resp.Message.Thinking() != "chain" {
		t.Fatalf("thinking = %q, want %q", resp.Message.Thinking(), "chain")
	}

	if resp.FinishReason != kit.FinishReasonToolCall {
		t.Fatalf("finish reason = %q, want %q", resp.FinishReason, kit.FinishReasonToolCall)
	}

	if got := len(resp.Message.Content); got != 4 {
		t.Fatalf("len(content) = %d, want 4", got)
	}

	if resp.Message.Content[3].Type != kit.ContentTypeToolCall {
		t.Fatalf("content[3].type = %q, want %q", resp.Message.Content[3].Type, kit.ContentTypeToolCall)
	}

	if resp.Message.Content[0].Type != kit.ContentTypeThinking {
		t.Fatalf("content[0].type = %q, want %q", resp.Message.Content[0].Type, kit.ContentTypeThinking)
	}

	if want := encodeSignature([]byte("part-sig")); resp.Message.Content[0].Signature != want {
		t.Fatalf("content[0].signature = %q, want %q", resp.Message.Content[0].Signature, want)
	}

	if resp.Message.Content[1].Type != kit.ContentTypeText {
		t.Fatalf("content[1].type = %q, want %q", resp.Message.Content[1].Type, kit.ContentTypeText)
	}

	if resp.Message.Content[1].Text != "final" {
		t.Fatalf("content[1].text = %q, want %q", resp.Message.Content[1].Text, "final")
	}

	if resp.Message.Content[2].Type != kit.ContentTypeImage {
		t.Fatalf("content[2].type = %q, want %q", resp.Message.Content[2].Type, kit.ContentTypeImage)
	}

	if string(resp.Message.Content[2].Data) != "img" {
		t.Fatalf("content[2].data = %q, want %q", string(resp.Message.Content[2].Data), "img")
	}

	toolCalls := resp.Message.ToolCalls()
	if got := len(toolCalls); got != 1 {
		t.Fatalf("len(tool_calls) = %d, want 1", got)
	}

	if toolCalls[0].ID != "call_9" {
		t.Fatalf("tool id = %q, want %q", toolCalls[0].ID, "call_9")
	}

	var toolPart kit.ContentPart
	for _, p := range resp.Message.Content {
		if p.Type == kit.ContentTypeToolCall {
			toolPart = p

			break
		}
	}

	if want := encodeSignature([]byte("sig")); toolPart.Signature != want {
		t.Fatalf("thought signature = %q, want %q", toolPart.Signature, want)
	}

	if resp.Usage.InputTokens != 9 || resp.Usage.OutputTokens != 4 || resp.Usage.CacheReadTokens != 2 || resp.Usage.ReasoningTokens != 6 {
		t.Fatalf("usage = %+v", resp.Usage)
	}
}

func TestBuildConfig_IncludesStructuredOutputSchema(t *testing.T) {
	config, err := buildConfig(kit.ModelRequest{
		ResponseSchema: &jsonschema.Schema{
			Type:     "object",
			Required: []string{"answer"},
			Properties: map[string]*jsonschema.Schema{
				"answer": {Type: "string"},
			},
		},
	})
	if err != nil {
		t.Fatalf("buildConfig returned error: %v", err)
	}

	if config.ResponseMIMEType != "application/json" {
		t.Fatalf("response mime type = %q, want %q", config.ResponseMIMEType, "application/json")
	}

	if config.ResponseJsonSchema == nil {
		t.Fatal("expected response json schema to be set")
	}

	raw, err := json.Marshal(config.ResponseJsonSchema)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	got := string(raw)
	if got != `{"type":"object","properties":{"answer":{"type":"string"}},"required":["answer"]}` &&
		got != `{"properties":{"answer":{"type":"string"}},"required":["answer"],"type":"object"}` {
		t.Fatalf("response json schema = %s", got)
	}
}

func TestConvertFinishReason_NoToolCallsUsesProviderReason(t *testing.T) {
	got := convertFinishReason(genai.FinishReasonSafety, nil)
	if got != kit.FinishReasonSafety {
		t.Fatalf("finish reason = %q, want %q", got, kit.FinishReasonSafety)
	}
}

func TestAppendThinkingDelta_MergesAndPreservesLatestSignature(t *testing.T) {
	parts := appendThinkingDelta(nil, "chain ", nil)
	parts = appendThinkingDelta(parts, "continued", []byte("sig"))

	if got := len(parts); got != 1 {
		t.Fatalf("len(parts) = %d, want 1", got)
	}

	if parts[0].Type != kit.ContentTypeThinking {
		t.Fatalf("parts[0].type = %q, want %q", parts[0].Type, kit.ContentTypeThinking)
	}

	if parts[0].Text != "chain continued" {
		t.Fatalf("parts[0].text = %q, want %q", parts[0].Text, "chain continued")
	}

	if want := encodeSignature([]byte("sig")); parts[0].Signature != want {
		t.Fatalf("parts[0].signature = %q, want %q", parts[0].Signature, want)
	}
}

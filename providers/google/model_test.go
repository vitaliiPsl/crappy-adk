package google

import (
	"testing"

	"google.golang.org/genai"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

func TestConvertContentPart_Text(t *testing.T) {
	part := convertContentPart(kit.NewTextPart("hello"))
	if part == nil {
		t.Fatal("expected text part")
	}

	if part.Text != "hello" {
		t.Fatalf("text = %q, want %q", part.Text, "hello")
	}
}

func TestConvertContentPart_ImageData(t *testing.T) {
	part := convertContentPart(kit.NewImageDataPart([]byte("png-bytes"), "image/png"))
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
	part := convertContentPart(kit.ContentPart{
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
	part := convertContentPart(kit.NewDocumentDataPart([]byte("%PDF-1.7"), "application/pdf"))
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
	part := convertContentPart(kit.ContentPart{
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
		Role:     kit.MessageRoleAssistant,
		Thinking: "chain",
		Content: []kit.ContentPart{{
			Type: kit.ContentTypeText,
			Text: "done",
		}},
		ToolCalls: []kit.ToolCall{{
			ID:        "call_1",
			Name:      "search",
			Arguments: map[string]any{"query": "crappy"},
			Metadata:  map[string]any{"thought_signature": []byte("sig")},
		}},
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
					{Text: "chain", Thought: true},
					{Text: "final"},
					{
						InlineData:       &genai.Blob{Data: []byte("img"), MIMEType: "image/png"},
						ThoughtSignature: []byte("part-sig"),
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

	if resp.Message.Thinking != "chain" {
		t.Fatalf("thinking = %q, want %q", resp.Message.Thinking, "chain")
	}

	if resp.FinishReason != kit.FinishReasonToolCall {
		t.Fatalf("finish reason = %q, want %q", resp.FinishReason, kit.FinishReasonToolCall)
	}

	if got := len(resp.Message.Content); got != 2 {
		t.Fatalf("len(content) = %d, want 2", got)
	}

	if resp.Message.Content[1].Type != kit.ContentTypeImage {
		t.Fatalf("content[1].type = %q, want %q", resp.Message.Content[1].Type, kit.ContentTypeImage)
	}

	if string(resp.Message.Content[1].Data) != "img" {
		t.Fatalf("content[1].data = %q, want %q", string(resp.Message.Content[1].Data), "img")
	}

	if got := len(resp.Message.ToolCalls); got != 1 {
		t.Fatalf("len(tool_calls) = %d, want 1", got)
	}

	if resp.Message.ToolCalls[0].ID != "call_9" {
		t.Fatalf("tool id = %q, want %q", resp.Message.ToolCalls[0].ID, "call_9")
	}

	if string(resp.Message.ToolCalls[0].Metadata["thought_signature"].([]byte)) != "sig" {
		t.Fatalf("thought signature = %q, want %q", string(resp.Message.ToolCalls[0].Metadata["thought_signature"].([]byte)), "sig")
	}

	if resp.Usage.InputTokens != 9 || resp.Usage.OutputTokens != 4 || resp.Usage.CacheReadTokens != 2 || resp.Usage.ReasoningTokens != 6 {
		t.Fatalf("usage = %+v", resp.Usage)
	}
}

func TestConvertFinishReason_NoToolCallsUsesProviderReason(t *testing.T) {
	got := convertFinishReason(genai.FinishReasonSafety, nil)
	if got != kit.FinishReasonSafety {
		t.Fatalf("finish reason = %q, want %q", got, kit.FinishReasonSafety)
	}
}

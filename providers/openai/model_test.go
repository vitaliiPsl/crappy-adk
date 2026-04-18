package openai

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/openai/openai-go/v3/responses"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

func TestNew_AllowsUnknownModelIDs(t *testing.T) {
	model, err := New("", "qwen3:8b")
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	cfg := model.Config()
	if cfg.ID != "qwen3:8b" {
		t.Fatalf("config.ID = %q, want %q", cfg.ID, "qwen3:8b")
	}

	if cfg.Provider != ProviderID {
		t.Fatalf("config.Provider = %q, want %q", cfg.Provider, ProviderID)
	}
}

func TestNew_PreservesKnownModelMetadata(t *testing.T) {
	model, err := New("", "gpt-5.4-mini")
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

	if part.OfInputText == nil {
		t.Fatal("expected input_text payload")
	}

	if part.OfInputText.Text != "hello" {
		t.Fatalf("text = %q, want %q", part.OfInputText.Text, "hello")
	}
}

func TestConvertContentPart_ImageData(t *testing.T) {
	part, ok := convertUserContentPart(kit.NewImageDataPart([]byte("png-bytes"), "image/png"))
	if !ok {
		t.Fatal("convertContentPart returned ok=false")
	}

	if part.OfInputImage == nil {
		t.Fatal("expected input_image payload")
	}

	want := "data:image/png;base64," + base64.StdEncoding.EncodeToString([]byte("png-bytes"))
	if got := part.OfInputImage.ImageURL.Value; got != want {
		t.Fatalf("image_url = %q, want %q", got, want)
	}
}

func TestConvertContentPart_ImageURL(t *testing.T) {
	part, ok := convertUserContentPart(kit.NewImageURLPart("https://example.com/image.png"))
	if !ok {
		t.Fatal("convertContentPart returned ok=false")
	}

	if part.OfInputImage == nil {
		t.Fatal("expected input_image payload")
	}

	if got := part.OfInputImage.ImageURL.Value; got != "https://example.com/image.png" {
		t.Fatalf("image_url = %q, want %q", got, "https://example.com/image.png")
	}
}

func TestConvertContentPart_DocumentData(t *testing.T) {
	part, ok := convertUserContentPart(kit.NewDocumentDataPart([]byte("%PDF-1.7"), "application/pdf"))
	if !ok {
		t.Fatal("convertContentPart returned ok=false")
	}

	if part.OfInputFile == nil {
		t.Fatal("expected input_file payload")
	}

	wantData := base64.StdEncoding.EncodeToString([]byte("%PDF-1.7"))
	if got := part.OfInputFile.FileData.Value; got != wantData {
		t.Fatalf("file_data = %q, want %q", got, wantData)
	}

	if got := part.OfInputFile.Filename.Value; got != "document.pdf" {
		t.Fatalf("filename = %q, want %q", got, "document.pdf")
	}
}

func TestConvertContentPart_DocumentURL(t *testing.T) {
	part, ok := convertUserContentPart(kit.NewDocumentURLPart("https://example.com/files/spec.pdf"))
	if !ok {
		t.Fatal("convertContentPart returned ok=false")
	}

	if part.OfInputFile == nil {
		t.Fatal("expected input_file payload")
	}

	if got := part.OfInputFile.FileURL.Value; got != "https://example.com/files/spec.pdf" {
		t.Fatalf("file_url = %q", got)
	}

	if got := part.OfInputFile.Filename.Value; got != "spec.pdf" {
		t.Fatalf("filename = %q, want %q", got, "spec.pdf")
	}
}

func TestConvertContentPart_DocumentURLWithQuery_UsesPathFilename(t *testing.T) {
	part, ok := convertUserContentPart(kit.NewDocumentURLPart("https://example.com/files/spec.pdf?download=1"))
	if !ok {
		t.Fatal("convertContentPart returned ok=false")
	}

	if part.OfInputFile == nil {
		t.Fatal("expected input_file payload")
	}

	if got := part.OfInputFile.Filename.Value; got != "spec.pdf" {
		t.Fatalf("filename = %q, want %q", got, "spec.pdf")
	}
}

func TestConvertAssistantMessage_PreservesTextAndToolCalls(t *testing.T) {
	items := convertAssistantMessage(kit.Message{
		Role: kit.MessageRoleAssistant,
		Content: []kit.ContentPart{
			{
				Type:      kit.ContentTypeThinking,
				ID:        "rs_123",
				Text:      "chain",
				Signature: "enc_sig",
			},
			kit.NewTextPart("done"),
			kit.NewToolCallPart(kit.ToolCall{
				ID:        "call_1",
				Name:      "search",
				Arguments: map[string]any{"query": "crappy"},
			}),
		},
	})

	if got := len(items); got != 3 {
		t.Fatalf("len(items) = %d, want 3", got)
	}

	if items[0].OfReasoning == nil {
		t.Fatal("expected first item to be reasoning")
	}

	if got := items[0].OfReasoning.ID; got != "rs_123" {
		t.Fatalf("reasoning id = %q, want %q", got, "rs_123")
	}

	if got := items[0].OfReasoning.EncryptedContent.Value; got != "enc_sig" {
		t.Fatalf("encrypted_content = %q, want %q", got, "enc_sig")
	}

	if items[1].OfMessage == nil || items[1].OfMessage.Role != responses.EasyInputMessageRoleAssistant {
		t.Fatal("expected second item to be assistant message")
	}

	if items[2].OfFunctionCall == nil {
		t.Fatal("expected third item to be function call")
	}

	if items[2].OfFunctionCall.Name != "search" {
		t.Fatalf("tool name = %q, want %q", items[2].OfFunctionCall.Name, "search")
	}
}

func TestConvertAssistantMessage_SkipsThinkingWithoutProviderID(t *testing.T) {
	items := convertAssistantMessage(kit.Message{
		Role: kit.MessageRoleAssistant,
		Content: []kit.ContentPart{
			kit.NewThinkingPart("chain", "enc_sig"),
			kit.NewTextPart("done"),
		},
	})

	if got := len(items); got != 1 {
		t.Fatalf("len(items) = %d, want 1", got)
	}

	if items[0].OfMessage == nil || items[0].OfMessage.Role != responses.EasyInputMessageRoleAssistant {
		t.Fatal("expected only assistant text message to remain")
	}
}

func TestConvertReasoningPart_PreservesSummaryAndOmitsSyntheticContent(t *testing.T) {
	item := convertReasoningPart(kit.ContentPart{
		Type:      kit.ContentTypeThinking,
		ID:        "rs_123",
		Text:      "chain",
		Signature: "enc_sig",
	})

	if item.OfReasoning == nil {
		t.Fatal("expected reasoning item")
	}

	data, err := json.Marshal(item.OfReasoning)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	raw := string(data)
	if strings.Contains(raw, "\"content\"") {
		t.Fatalf("reasoning json = %s, expected no content field", raw)
	}

	if !strings.Contains(raw, "\"id\":\"rs_123\"") {
		t.Fatalf("reasoning json = %s, expected reasoning id", raw)
	}

	if !strings.Contains(raw, "\"summary\"") {
		t.Fatalf("reasoning json = %s, expected summary field", raw)
	}

	if !strings.Contains(raw, "\"text\":\"chain\"") {
		t.Fatalf("reasoning json = %s, expected summary text", raw)
	}

	if !strings.Contains(raw, "\"encrypted_content\":\"enc_sig\"") {
		t.Fatalf("reasoning json = %s, expected encrypted_content", raw)
	}
}

func TestConvertResponse_PreservesThinkingToolCallsAndUsage(t *testing.T) {
	raw := []byte(`{
		"status":"completed",
		"output":[
			{
				"type":"reasoning",
				"id":"rs_123",
				"summary":[{"type":"summary_text","text":"chain"}],
				"encrypted_content":"enc_sig"
			},
			{
				"type":"message",
				"id":"msg_123",
				"role":"assistant",
				"content":[{"type":"output_text","text":"final"}]
			},
			{
				"type":"function_call",
				"id":"fc_123",
				"call_id":"call_1",
				"name":"read_file",
				"arguments":"{\"path\":\"README.md\"}"
			}
		],
		"usage":{
			"input_tokens":9,
			"output_tokens":4,
			"input_tokens_details":{"cached_tokens":2},
			"output_tokens_details":{"reasoning_tokens":6}
		}
	}`)

	var resp responses.Response
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	got := convertResponse(&resp)

	if got.Message.Text() != "final" {
		t.Fatalf("text = %q, want %q", got.Message.Text(), "final")
	}

	if got.Message.Thinking() != "chain" {
		t.Fatalf("thinking = %q, want %q", got.Message.Thinking(), "chain")
	}

	if gotLen := len(got.Message.Content); gotLen != 3 {
		t.Fatalf("len(content) = %d, want 3", gotLen)
	}

	if got.Message.Content[0].Type != kit.ContentTypeThinking {
		t.Fatalf("content[0].type = %q, want %q", got.Message.Content[0].Type, kit.ContentTypeThinking)
	}

	if got.Message.Content[0].ID != "rs_123" {
		t.Fatalf("content[0].id = %q, want %q", got.Message.Content[0].ID, "rs_123")
	}

	if got.Message.Content[0].Signature != "enc_sig" {
		t.Fatalf("content[0].signature = %q, want %q", got.Message.Content[0].Signature, "enc_sig")
	}

	if got.Message.Content[2].Type != kit.ContentTypeToolCall {
		t.Fatalf("content[2].type = %q, want %q", got.Message.Content[2].Type, kit.ContentTypeToolCall)
	}

	if got.Message.Content[2].ID != "call_1" {
		t.Fatalf("content[2].id = %q, want %q", got.Message.Content[2].ID, "call_1")
	}

	if got.FinishReason != kit.FinishReasonToolCall {
		t.Fatalf("finish reason = %q, want %q", got.FinishReason, kit.FinishReasonToolCall)
	}

	if len(got.Message.ToolCalls()) != 1 {
		t.Fatalf("len(tool_calls) = %d, want 1", len(got.Message.ToolCalls()))
	}

	if got.Message.ToolCalls()[0].Name != "read_file" {
		t.Fatalf("tool name = %q, want %q", got.Message.ToolCalls()[0].Name, "read_file")
	}

	if got.Usage.InputTokens != 9 || got.Usage.OutputTokens != 4 || got.Usage.CacheReadTokens != 2 || got.Usage.ReasoningTokens != 6 {
		t.Fatalf("usage = %+v", got.Usage)
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
	}, "gpt-5")
	if err != nil {
		t.Fatalf("buildParams returned error: %v", err)
	}

	data, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	raw := string(data)
	if !strings.Contains(raw, `"type":"json_schema"`) ||
		!strings.Contains(raw, `"name":"structured_output"`) ||
		!strings.Contains(raw, `"answer":{"type":"string"}`) {
		t.Fatalf("params json = %s", raw)
	}
}

func TestConvertStatus_IncompleteSafety(t *testing.T) {
	got := convertStatus(&responses.Response{
		Status: responses.ResponseStatusIncomplete,
		IncompleteDetails: responses.ResponseIncompleteDetails{
			Reason: "content_filter",
		},
	})

	if got != kit.FinishReasonSafety {
		t.Fatalf("finish reason = %q, want %q", got, kit.FinishReasonSafety)
	}
}

func TestConvertFunctionCall_InvalidArguments(t *testing.T) {
	var item responses.ResponseOutputItemUnion

	err := json.Unmarshal([]byte(`{
		"type":"function_call",
		"id":"fc_123",
		"call_id":"call_1",
		"name":"broken_tool",
		"arguments":"{"
	}`), &item)
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	_, err = convertFunctionCall(item)
	if err == nil {
		t.Fatal("expected error")
	}
}

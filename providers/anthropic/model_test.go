package anthropic

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	anthropicsdk "github.com/anthropics/anthropic-sdk-go"

	"github.com/vitaliiPsl/crappy-adk/kit"
	"github.com/vitaliiPsl/crappy-adk/providers"
)

func TestModelResolvesCredentialsForEveryRequest(t *testing.T) {
	var (
		authorization []string
		apiKeys       []string
	)

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		authorization = append(authorization, request.Header.Get("Authorization"))
		apiKeys = append(apiKeys, request.Header.Get("X-Api-Key"))
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{
  "id":"message",
  "type":"message",
  "role":"assistant",
  "model":"test-model",
  "content":[],
  "stop_reason":"end_turn",
  "stop_sequence":null,
  "usage":{"input_tokens":1,"output_tokens":1}
}`))
	}))
	t.Cleanup(server.Close)

	source := &rotatingCredentialsSource{}

	model, err := New(
		"test-model",
		providers.WithBaseURL(server.URL),
		providers.WithCredentials(source),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	for range 2 {
		if _, err := model.Generate(context.Background(), kit.ModelRequest{}); err != nil {
			t.Fatalf("Generate() error = %v", err)
		}
	}

	want := []string{"Bearer token-1", "Bearer token-2"}
	if fmt.Sprint(authorization) != fmt.Sprint(want) {
		t.Fatalf("Authorization headers = %v, want %v", authorization, want)
	}

	if fmt.Sprint(apiKeys) != fmt.Sprint([]string{"", ""}) {
		t.Fatalf("X-Api-Key headers = %v, want empty", apiKeys)
	}
}

func TestModelSendsAPIKeyInKeyHeader(t *testing.T) {
	var (
		authorization string
		apiKey        string
	)

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		authorization = request.Header.Get("Authorization")
		apiKey = request.Header.Get("X-Api-Key")
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{
  "id":"message",
  "type":"message",
  "role":"assistant",
  "model":"test-model",
  "content":[],
  "stop_reason":"end_turn",
  "stop_sequence":null,
  "usage":{"input_tokens":1,"output_tokens":1}
}`))
	}))
	t.Cleanup(server.Close)

	model, err := New(
		"test-model",
		providers.WithBaseURL(server.URL),
		providers.WithAPIKey("secret"),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if _, err := model.Generate(context.Background(), kit.ModelRequest{}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	if apiKey != "secret" {
		t.Fatalf("X-Api-Key = %q, want secret", apiKey)
	}

	if authorization != "" {
		t.Fatalf("Authorization = %q, want empty", authorization)
	}
}

func TestBuildRequestParamsStructuredOutput(t *testing.T) {
	schema := map[string]any{"type": "object"}
	params := buildRequestParams("test-model", kit.ModelRequest{
		OutputSchema: &kit.OutputSchema{Name: "assessment", Schema: schema},
	})

	if fmt.Sprint(params.OutputConfig.Format.Schema) != fmt.Sprint(schema) {
		t.Fatalf("output schema = %v, want %v", params.OutputConfig.Format.Schema, schema)
	}
}

func TestBuildRequestParamsSystem(t *testing.T) {
	tests := []struct {
		name         string
		instructions string
		want         []anthropicsdk.TextBlockParam
	}{
		{name: "empty is omitted", instructions: ""},
		{name: "blank is omitted", instructions: "   \n\t "},
		{
			name:         "present is sent verbatim",
			instructions: "\nYou are helpful.\n",
			want:         []anthropicsdk.TextBlockParam{{Text: "\nYou are helpful.\n"}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			params := buildRequestParams("test-model", kit.ModelRequest{
				Instructions: test.instructions,
			})

			if fmt.Sprint(params.System) != fmt.Sprint(test.want) {
				t.Fatalf("System = %v, want %v", params.System, test.want)
			}
		})
	}
}

type rotatingCredentialsSource struct {
	requests int
}

func (s *rotatingCredentialsSource) Headers(context.Context) (http.Header, error) {
	s.requests++

	return http.Header{
		"Authorization": {fmt.Sprintf("Bearer token-%d", s.requests)},
	}, nil
}

type mockTool struct {
	name, description string
	schema            map[string]any
}

func (m *mockTool) Definition() kit.ToolDefinition {
	return kit.ToolDefinition{Name: m.name, Description: m.description, Schema: m.schema}
}

func (m *mockTool) Execute(_ *kit.RunContext, _ kit.ToolCall) (kit.ToolOutput, error) {
	return kit.ToolOutput{}, nil
}

func mustParseBlock(t *testing.T, data string) anthropicsdk.ContentBlockUnion {
	t.Helper()

	var block anthropicsdk.ContentBlockUnion
	if err := json.Unmarshal([]byte(data), &block); err != nil {
		t.Fatalf("unmarshal content block: %v", err)
	}

	return block
}

func mustParseMessage(t *testing.T, data string) *anthropicsdk.Message {
	t.Helper()

	var msg anthropicsdk.Message
	if err := json.Unmarshal([]byte(data), &msg); err != nil {
		t.Fatalf("unmarshal message: %v", err)
	}

	return &msg
}

func TestConvertRequestTools(t *testing.T) {
	t.Run("nil returns nil", func(t *testing.T) {
		if got := convertRequestTools(nil); got != nil {
			t.Errorf("expected nil, got %v", got)
		}
	})

	t.Run("empty slice returns nil", func(t *testing.T) {
		if got := convertRequestTools([]kit.Tool{}); got != nil {
			t.Errorf("expected nil, got %v", got)
		}
	})

	t.Run("single tool", func(t *testing.T) {
		schema := map[string]any{
			"type":       "object",
			"properties": map[string]any{"query": map[string]any{"type": "string"}},
			"required":   []any{"query"},
		}
		tool := &mockTool{name: "search", description: "search the web", schema: schema}

		result := convertRequestTools([]kit.Tool{tool})

		if len(result) != 1 {
			t.Fatalf("len = %d, want 1", len(result))
		}

		tp := result[0].OfTool
		if tp == nil {
			t.Fatal("OfTool is nil")
		}

		if tp.Name != "search" {
			t.Errorf("Name = %q, want %q", tp.Name, "search")
		}

		if tp.Description.Value != "search the web" {
			t.Errorf("Description = %q, want %q", tp.Description.Value, "search the web")
		}
	})

	t.Run("multiple tools preserve order", func(t *testing.T) {
		tools := []kit.Tool{
			&mockTool{name: "tool_a"},
			&mockTool{name: "tool_b"},
			&mockTool{name: "tool_c"},
		}

		result := convertRequestTools(tools)

		if len(result) != 3 {
			t.Fatalf("len = %d, want 3", len(result))
		}

		for i, want := range []string{"tool_a", "tool_b", "tool_c"} {
			if got := result[i].OfTool.Name; got != want {
				t.Errorf("result[%d].Name = %q, want %q", i, got, want)
			}
		}
	})
}

func TestConvertRequestContentItem(t *testing.T) {
	t.Run("text", func(t *testing.T) {
		content := kit.NewTextContent("hello")

		item, ok := convertRequestContentItem(content)

		if !ok {
			t.Fatal("ok = false, want true")
		}

		if item.OfText == nil {
			t.Fatal("OfText is nil")
		}

		if item.OfText.Text != "hello" {
			t.Errorf("Text = %q, want %q", item.OfText.Text, "hello")
		}
	})

	t.Run("text with nil payload returns false", func(t *testing.T) {
		content := kit.Content{Type: kit.ContentTypeText}

		if _, ok := convertRequestContentItem(content); ok {
			t.Error("expected false, got true")
		}
	})

	t.Run("thinking", func(t *testing.T) {
		content := kit.NewThinkingContent("id_1", "I think...", "sig_xyz")

		item, ok := convertRequestContentItem(content)

		if !ok {
			t.Fatal("ok = false, want true")
		}

		if item.OfThinking == nil {
			t.Fatal("OfThinking is nil")
		}

		if item.OfThinking.Thinking != "I think..." {
			t.Errorf("Thinking = %q, want %q", item.OfThinking.Thinking, "I think...")
		}

		if item.OfThinking.Signature != "sig_xyz" {
			t.Errorf("Signature = %q, want %q", item.OfThinking.Signature, "sig_xyz")
		}
	})

	t.Run("thinking with nil payload returns false", func(t *testing.T) {
		content := kit.Content{Type: kit.ContentTypeThinking}

		if _, ok := convertRequestContentItem(content); ok {
			t.Error("expected false, got true")
		}
	})

	t.Run("image bytes", func(t *testing.T) {
		content := kit.NewImageContent("image/png", []byte{1, 2, 3})

		item, ok := convertRequestContentItem(content)

		if !ok {
			t.Fatal("ok = false, want true")
		}

		if item.OfImage == nil {
			t.Fatal("OfImage is nil")
		}

		source := item.OfImage.Source.OfBase64
		if source == nil {
			t.Fatal("OfBase64 is nil")
		}

		if source.MediaType != "image/png" {
			t.Errorf("MediaType = %q, want image/png", source.MediaType)
		}

		if source.Data != base64.StdEncoding.EncodeToString([]byte{1, 2, 3}) {
			t.Errorf("Data = %q, want base64 image bytes", source.Data)
		}
	})

	t.Run("resource text document", func(t *testing.T) {
		content := kit.NewResourceContent(kit.Resource{Text: "# README"})

		item, ok := convertRequestContentItem(content)

		if !ok {
			t.Fatal("ok = false, want true")
		}

		if item.OfDocument == nil {
			t.Fatal("OfDocument is nil")
		}

		source := item.OfDocument.Source.OfText
		if source == nil {
			t.Fatal("OfText is nil")
		}

		if source.Data != "# README" {
			t.Errorf("Data = %q, want README text", source.Data)
		}
	})

	t.Run("tool call", func(t *testing.T) {
		content := kit.NewToolCallContent(kit.ToolCall{
			ID:        "call_1",
			Name:      "search",
			Arguments: map[string]any{"query": "Go"},
		})

		item, ok := convertRequestContentItem(content)

		if !ok {
			t.Fatal("ok = false, want true")
		}

		if item.OfToolUse == nil {
			t.Fatal("OfToolUse is nil")
		}

		if item.OfToolUse.ID != "call_1" {
			t.Errorf("ID = %q, want %q", item.OfToolUse.ID, "call_1")
		}

		if item.OfToolUse.Name != "search" {
			t.Errorf("Name = %q, want %q", item.OfToolUse.Name, "search")
		}
	})

	t.Run("tool call with nil payload returns false", func(t *testing.T) {
		content := kit.Content{Type: kit.ContentTypeToolCall}

		if _, ok := convertRequestContentItem(content); ok {
			t.Error("expected false, got true")
		}
	})

	t.Run("tool result success", func(t *testing.T) {
		call := kit.ToolCall{ID: "call_1", Name: "search"}
		content := kit.NewToolResultContent(kit.NewToolResult(call, kit.NewToolOutput(kit.NewTextContent("result output")), nil))

		item, ok := convertRequestContentItem(content)

		if !ok {
			t.Fatal("ok = false, want true")
		}

		tr := item.OfToolResult
		if tr == nil {
			t.Fatal("OfToolResult is nil")
		}

		if tr.ToolUseID != "call_1" {
			t.Errorf("ToolUseID = %q, want %q", tr.ToolUseID, "call_1")
		}

		if tr.Content[0].OfText.Text != "result output" {
			t.Errorf("Content = %q, want %q", tr.Content[0].OfText.Text, "result output")
		}

		if tr.IsError.Value {
			t.Error("IsError = true, want false")
		}
	})

	t.Run("tool result error", func(t *testing.T) {
		call := kit.ToolCall{ID: "call_1", Name: "search"}
		content := kit.NewToolResultContent(kit.NewToolResult(call, kit.ToolOutput{}, fmt.Errorf("tool failed")))

		item, ok := convertRequestContentItem(content)

		if !ok {
			t.Fatal("ok = false, want true")
		}

		tr := item.OfToolResult
		if tr == nil {
			t.Fatal("OfToolResult is nil")
		}

		if tr.Content[0].OfText.Text != "tool failed" {
			t.Errorf("Content = %q, want %q", tr.Content[0].OfText.Text, "tool failed")
		}

		if !tr.IsError.Value {
			t.Error("IsError = false, want true")
		}
	})

	t.Run("tool result with nil payload returns false", func(t *testing.T) {
		content := kit.Content{Type: kit.ContentTypeToolResult}

		if _, ok := convertRequestContentItem(content); ok {
			t.Error("expected false, got true")
		}
	})

	t.Run("unknown type returns false", func(t *testing.T) {
		content := kit.Content{Type: "unknown"}

		if _, ok := convertRequestContentItem(content); ok {
			t.Error("expected false, got true")
		}
	})
}

func TestConvertRequestMessage(t *testing.T) {
	t.Run("user message produces user role", func(t *testing.T) {
		msg := kit.NewUserMessage(kit.NewTextContent("hi"))

		result := convertRequestMessage(msg)

		if len(result) != 1 {
			t.Fatalf("len = %d, want 1", len(result))
		}

		if result[0].Role != "user" {
			t.Errorf("Role = %q, want %q", result[0].Role, "user")
		}
	})

	t.Run("model message produces assistant role", func(t *testing.T) {
		msg := kit.NewModelMessage(kit.NewTextContent("hello"))

		result := convertRequestMessage(msg)

		if len(result) != 1 {
			t.Fatalf("len = %d, want 1", len(result))
		}

		if result[0].Role != "assistant" {
			t.Errorf("Role = %q, want %q", result[0].Role, "assistant")
		}
	})

	t.Run("tool message produces user role", func(t *testing.T) {
		call := kit.ToolCall{ID: "call_1", Name: "fn"}
		msg := kit.NewToolMessage(
			kit.NewToolResultContent(kit.NewToolResult(call, kit.NewToolOutput(kit.NewTextContent("ok")), nil)))

		result := convertRequestMessage(msg)

		if len(result) != 1 {
			t.Fatalf("len = %d, want 1", len(result))
		}

		if result[0].Role != "user" {
			t.Errorf("Role = %q, want %q", result[0].Role, "user")
		}
	})
}

func TestConvertRequestDropsBlankText(t *testing.T) {
	t.Run("blank text block is dropped", func(t *testing.T) {
		items := convertRequestContentItems([]kit.Content{
			kit.NewTextContent("   \n\t "),
			kit.NewTextContent("kept"),
		})

		if len(items) != 1 {
			t.Fatalf("len = %d, want 1", len(items))
		}

		if items[0].OfText == nil || items[0].OfText.Text != "kept" {
			t.Errorf("block = %v, want text %q", items[0], "kept")
		}
	})

	t.Run("message with only blank text is dropped", func(t *testing.T) {
		result := convertRequestMessage(kit.NewUserMessage(kit.NewTextContent("")))

		if len(result) != 0 {
			t.Fatalf("len = %d, want 0", len(result))
		}
	})

	t.Run("message with unconvertible content is dropped", func(t *testing.T) {
		result := convertRequestMessage(kit.NewUserMessage(kit.Content{Type: kit.ContentTypeText}))

		if len(result) != 0 {
			t.Fatalf("len = %d, want 0", len(result))
		}
	})
}

func TestConvertToolResultContentEmptyOutput(t *testing.T) {
	call := kit.NewToolCall("call_1", "bash", nil)

	tests := []struct {
		name   string
		output kit.ToolOutput
	}{
		{name: "no content at all", output: kit.ToolOutput{}},
		{name: "single empty text block", output: kit.NewToolOutput(kit.NewTextContent(""))},
		{name: "blank text block", output: kit.NewToolOutput(kit.NewTextContent(" \n "))},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := kit.NewToolResult(call, test.output, nil)

			block := convertToolResultContent(&result)
			if block.OfToolResult == nil {
				t.Fatalf("block = %v, want tool result", block)
			}

			content := block.OfToolResult.Content
			if len(content) != 1 {
				t.Fatalf("len = %d, want 1", len(content))
			}

			if content[0].OfText == nil || content[0].OfText.Text != emptyToolResultText {
				t.Fatalf("text = %v, want %q", content[0], emptyToolResultText)
			}
		})
	}

	t.Run("error is preferred over the placeholder", func(t *testing.T) {
		result := kit.NewToolResult(call, kit.NewToolOutput(kit.NewTextContent("")), errors.New("boom"))

		block := convertToolResultContent(&result)

		content := block.OfToolResult.Content
		if len(content) != 1 || content[0].OfText == nil || content[0].OfText.Text != "boom" {
			t.Fatalf("text = %v, want %q", content, "boom")
		}
	})
}

func TestConvertResponseContent(t *testing.T) {
	t.Run("text block", func(t *testing.T) {
		block := mustParseBlock(t, `{"type": "text", "text": "Hello!"}`)

		got := convertResponseContent(block)

		if len(got) != 1 {
			t.Fatalf("len = %d, want 1", len(got))
		}

		if got[0].Type != kit.ContentTypeText {
			t.Errorf("Type = %q, want %q", got[0].Type, kit.ContentTypeText)
		}

		if got[0].Text.Text != "Hello!" {
			t.Errorf("Text = %q, want %q", got[0].Text.Text, "Hello!")
		}
	})

	t.Run("thinking block", func(t *testing.T) {
		block := mustParseBlock(t, `{"type": "thinking", "thinking": "I think...", "signature": "sig_xyz"}`)

		got := convertResponseContent(block)

		if len(got) != 1 {
			t.Fatalf("len = %d, want 1", len(got))
		}

		if got[0].Type != kit.ContentTypeThinking {
			t.Errorf("Type = %q, want %q", got[0].Type, kit.ContentTypeThinking)
		}

		th := got[0].Thinking
		if th.Text != "I think..." {
			t.Errorf("Text = %q, want %q", th.Text, "I think...")
		}

		if th.Signature != "sig_xyz" {
			t.Errorf("Signature = %q, want %q", th.Signature, "sig_xyz")
		}
	})

	t.Run("tool_use block", func(t *testing.T) {
		block := mustParseBlock(t, `{"type": "tool_use", "id": "tu_1", "name": "search", "input": {"query": "Go"}}`)

		got := convertResponseContent(block)

		if len(got) != 1 {
			t.Fatalf("len = %d, want 1", len(got))
		}

		if got[0].Type != kit.ContentTypeToolCall {
			t.Errorf("Type = %q, want %q", got[0].Type, kit.ContentTypeToolCall)
		}

		tc := got[0].ToolCall
		if tc.ID != "tu_1" {
			t.Errorf("ID = %q, want %q", tc.ID, "tu_1")
		}

		if tc.Name != "search" {
			t.Errorf("Name = %q, want %q", tc.Name, "search")
		}

		if tc.Arguments["query"] != "Go" {
			t.Errorf("Arguments[query] = %v, want %q", tc.Arguments["query"], "Go")
		}
	})

	t.Run("unknown type returns nil", func(t *testing.T) {
		block := mustParseBlock(t, `{"type": "server_tool_use", "id": "stu_1", "name": "web_search", "input": {}}`)

		if got := convertResponseContent(block); len(got) != 0 {
			t.Errorf("expected nil/empty, got %v", got)
		}
	})
}

func TestConvertResponseFinishReason(t *testing.T) {
	tests := []struct {
		name string
		json string
		want kit.FinishReason
	}{
		{
			name: "end_turn",
			json: `{"id":"m","type":"message","role":"assistant","model":"claude-3-5-sonnet-20241022","content":[],"stop_reason":"end_turn","stop_sequence":null,"usage":{"input_tokens":1,"output_tokens":1}}`,
			want: kit.FinishReasonStop,
		},
		{
			name: "max_tokens",
			json: `{"id":"m","type":"message","role":"assistant","model":"claude-3-5-sonnet-20241022","content":[],"stop_reason":"max_tokens","stop_sequence":null,"usage":{"input_tokens":1,"output_tokens":1}}`,
			want: kit.FinishReasonMaxTokens,
		},
		{
			name: "tool_use",
			json: `{"id":"m","type":"message","role":"assistant","model":"claude-3-5-sonnet-20241022","content":[],"stop_reason":"tool_use","stop_sequence":null,"usage":{"input_tokens":1,"output_tokens":1}}`,
			want: kit.FinishReasonToolCall,
		},
		{
			name: "refusal",
			json: `{"id":"m","type":"message","role":"assistant","model":"claude-3-5-sonnet-20241022","content":[],"stop_reason":"refusal","stop_sequence":null,"usage":{"input_tokens":1,"output_tokens":1}}`,
			want: kit.FinishReasonSafety,
		},
		{
			name: "pause_turn",
			json: `{"id":"m","type":"message","role":"assistant","model":"claude-3-5-sonnet-20241022","content":[],"stop_reason":"pause_turn","stop_sequence":null,"usage":{"input_tokens":1,"output_tokens":1}}`,
			want: kit.FinishReasonUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := mustParseMessage(t, tt.json)
			if got := convertResponseFinishReason(msg); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestConvertResponseUsage(t *testing.T) {
	msg := mustParseMessage(t, `{
		"id": "m", "type": "message", "role": "assistant",
		"model": "claude-3-5-sonnet-20241022", "content": [],
		"stop_reason": "end_turn", "stop_sequence": null,
		"usage": {
			"input_tokens": 100,
			"output_tokens": 50,
			"cache_creation_input_tokens": 20,
			"cache_read_input_tokens": 30
		}
	}`)

	got := convertResponseUsage(msg)

	if got.InputTokens != 100 {
		t.Errorf("InputTokens = %d, want 100", got.InputTokens)
	}

	if got.OutputTokens != 50 {
		t.Errorf("OutputTokens = %d, want 50", got.OutputTokens)
	}

	if got.CacheWriteTokens != 20 {
		t.Errorf("CacheWriteTokens = %d, want 20", got.CacheWriteTokens)
	}

	if got.CacheReadTokens != 30 {
		t.Errorf("CacheReadTokens = %d, want 30", got.CacheReadTokens)
	}
}

func makeAPIError(statusCode int) *anthropicsdk.Error {
	return &anthropicsdk.Error{
		StatusCode: statusCode,
		Request:    &http.Request{},
		Response:   &http.Response{StatusCode: statusCode},
	}
}

func makeAPIErrorWithBody(t *testing.T, statusCode int, body string) *anthropicsdk.Error {
	t.Helper()

	err := &anthropicsdk.Error{
		StatusCode: statusCode,
		Request:    &http.Request{},
		Response:   &http.Response{StatusCode: statusCode},
	}

	if jsonErr := json.Unmarshal([]byte(body), err); jsonErr != nil {
		t.Fatalf("makeAPIErrorWithBody: %v", jsonErr)
	}

	return err
}

func TestMapError(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		wantErr error
	}{
		{
			name:    "non-api error passes through",
			err:     fmt.Errorf("some other error"),
			wantErr: nil,
		},
		{
			name:    "401 maps to ErrAuthentication",
			err:     makeAPIError(401),
			wantErr: kit.ErrAuthentication,
		},
		{
			name:    "403 maps to ErrAuthentication",
			err:     makeAPIError(403),
			wantErr: kit.ErrAuthentication,
		},
		{
			name:    "429 maps to ErrRateLimit",
			err:     makeAPIError(429),
			wantErr: kit.ErrRateLimit,
		},
		{
			name:    "529 maps to ErrRateLimit",
			err:     makeAPIError(529),
			wantErr: kit.ErrRateLimit,
		},
		{
			name:    "400 maps to ErrInvalidRequest",
			err:     makeAPIError(400),
			wantErr: kit.ErrInvalidRequest,
		},
		{
			name:    "400 with prompt too long body maps to ErrContextLength",
			err:     makeAPIErrorWithBody(t, 400, `{"type":"error","error":{"type":"invalid_request_error","message":"prompt is too long: 210000 tokens > 200000 maximum"}}`),
			wantErr: kit.ErrContextLength,
		},
		{
			name:    "400 with context limit body maps to ErrContextLength",
			err:     makeAPIErrorWithBody(t, 400, `{"type":"error","error":{"type":"invalid_request_error","message":"context limit exceeded"}}`),
			wantErr: kit.ErrContextLength,
		},
		{
			name:    "500 maps to ErrServerError",
			err:     makeAPIError(500),
			wantErr: kit.ErrServerError,
		},
		{
			name:    "502 maps to ErrServerError",
			err:     makeAPIError(502),
			wantErr: kit.ErrServerError,
		},
		{
			name:    "503 maps to ErrServerError",
			err:     makeAPIError(503),
			wantErr: kit.ErrServerError,
		},
		{
			name:    "504 maps to ErrServerError",
			err:     makeAPIError(504),
			wantErr: kit.ErrServerError,
		},
		{
			name:    "unrecognised status passes through",
			err:     makeAPIError(422),
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapError(tt.err)

			if tt.wantErr == nil {
				if got != tt.err {
					t.Errorf("expected original error, got %v", got)
				}

				return
			}

			if !errors.Is(got, tt.wantErr) {
				t.Errorf("errors.Is(%v, %v) = false", got, tt.wantErr)
			}
		})
	}
}

func TestMapErrorUsesConciseResponseMessage(t *testing.T) {
	err := makeAPIErrorWithBody(t, http.StatusBadRequest, `{"type":"error","error":{"type":"invalid_request_error","message":"tools.0.name is invalid"}}`)

	got := mapError(err)

	if got.Error() != "invalid request: tools.0.name is invalid" {
		t.Fatalf("mapError() = %q", got)
	}
}

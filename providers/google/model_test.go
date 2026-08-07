package google

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"google.golang.org/genai"

	"github.com/vitaliiPsl/crappy-adk/kit"
	"github.com/vitaliiPsl/crappy-adk/providers"
)

func TestNewRejectsCredentialsSource(t *testing.T) {
	_, err := New(
		"test-model",
		providers.WithCredentialsSource(unsupportedCredentialsSource{}),
	)
	if err == nil || err.Error() != "google provider: credentials source is not supported" {
		t.Fatalf("New() error = %v, want unsupported credentials source", err)
	}
}

type unsupportedCredentialsSource struct{}

func (unsupportedCredentialsSource) Headers(context.Context) (http.Header, error) {
	return nil, nil
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

		decls := result[0].FunctionDeclarations
		if len(decls) != 1 {
			t.Fatalf("FunctionDeclarations len = %d, want 1", len(decls))
		}

		if decls[0].Name != "search" {
			t.Errorf("Name = %q, want %q", decls[0].Name, "search")
		}

		if decls[0].Description != "search the web" {
			t.Errorf("Description = %q, want %q", decls[0].Description, "search the web")
		}
	})

	t.Run("multiple tools are grouped in a single Tool", func(t *testing.T) {
		tools := []kit.Tool{
			&mockTool{name: "tool_a"},
			&mockTool{name: "tool_b"},
			&mockTool{name: "tool_c"},
		}

		result := convertRequestTools(tools)

		if len(result) != 1 {
			t.Fatalf("len = %d, want 1 (all declarations inside one Tool)", len(result))
		}

		decls := result[0].FunctionDeclarations
		if len(decls) != 3 {
			t.Fatalf("FunctionDeclarations len = %d, want 3", len(decls))
		}

		for i, want := range []string{"tool_a", "tool_b", "tool_c"} {
			if got := decls[i].Name; got != want {
				t.Errorf("decls[%d].Name = %q, want %q", i, got, want)
			}
		}
	})
}

func TestConvertRequestContentItem(t *testing.T) {
	t.Run("text", func(t *testing.T) {
		content := kit.NewTextContent("hello")

		part, ok := convertRequestContentItem(content)

		if !ok {
			t.Fatal("ok = false, want true")
		}

		if part.Text != "hello" {
			t.Errorf("Text = %q, want %q", part.Text, "hello")
		}
	})

	t.Run("text with nil payload returns false", func(t *testing.T) {
		content := kit.Content{Type: kit.ContentTypeText}

		if _, ok := convertRequestContentItem(content); ok {
			t.Error("expected false, got true")
		}
	})

	t.Run("thinking", func(t *testing.T) {
		content := kit.NewThinkingContent("id_1", "I think...", "c2lnX3h5eg==")

		part, ok := convertRequestContentItem(content)

		if !ok {
			t.Fatal("ok = false, want true")
		}

		if !part.Thought {
			t.Error("Thought = false, want true")
		}

		if part.Text != "I think..." {
			t.Errorf("Text = %q, want %q", part.Text, "I think...")
		}

		if string(part.ThoughtSignature) != "sig_xyz" {
			t.Errorf("ThoughtSignature = %q, want %q", string(part.ThoughtSignature), "sig_xyz")
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

		part, ok := convertRequestContentItem(content)

		if !ok {
			t.Fatal("ok = false, want true")
		}

		if part.InlineData == nil {
			t.Fatal("InlineData is nil")
		}

		if part.InlineData.MIMEType != "image/png" {
			t.Errorf("MIMEType = %q, want image/png", part.InlineData.MIMEType)
		}

		if !bytes.Equal(part.InlineData.Data, []byte{1, 2, 3}) {
			t.Errorf("Data = %v, want [1 2 3]", part.InlineData.Data)
		}
	})

	t.Run("audio uri", func(t *testing.T) {
		content := kit.NewAudioURLContent("audio/mpeg", "gs://bucket/audio.mp3")

		part, ok := convertRequestContentItem(content)

		if !ok {
			t.Fatal("ok = false, want true")
		}

		if part.FileData == nil {
			t.Fatal("FileData is nil")
		}

		if part.FileData.FileURI != "gs://bucket/audio.mp3" {
			t.Errorf("FileURI = %q, want audio URI", part.FileData.FileURI)
		}

		if part.FileData.MIMEType != "audio/mpeg" {
			t.Errorf("MIMEType = %q, want audio/mpeg", part.FileData.MIMEType)
		}
	})

	t.Run("resource text", func(t *testing.T) {
		content := kit.NewResourceContent(kit.Resource{Text: "# README"})

		part, ok := convertRequestContentItem(content)

		if !ok {
			t.Fatal("ok = false, want true")
		}

		if part.Text != "# README" {
			t.Errorf("Text = %q, want README text", part.Text)
		}
	})

	t.Run("tool call", func(t *testing.T) {
		content := kit.NewToolCallContent(kit.ToolCall{
			ID:        "call_1",
			Name:      "search",
			Arguments: map[string]any{"query": "Go"},
			Signature: "c2lnX3Rvb2w=",
		})

		part, ok := convertRequestContentItem(content)

		if !ok {
			t.Fatal("ok = false, want true")
		}

		if part.FunctionCall == nil {
			t.Fatal("FunctionCall is nil")
		}

		if part.FunctionCall.ID != "call_1" {
			t.Errorf("ID = %q, want %q", part.FunctionCall.ID, "call_1")
		}

		if part.FunctionCall.Name != "search" {
			t.Errorf("Name = %q, want %q", part.FunctionCall.Name, "search")
		}

		if part.FunctionCall.Args["query"] != "Go" {
			t.Errorf("Args[query] = %v, want %q", part.FunctionCall.Args["query"], "Go")
		}

		if string(part.ThoughtSignature) != "sig_tool" {
			t.Errorf("ThoughtSignature = %q, want %q", string(part.ThoughtSignature), "sig_tool")
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

		part, ok := convertRequestContentItem(content)

		if !ok {
			t.Fatal("ok = false, want true")
		}

		if part.FunctionResponse == nil {
			t.Fatal("FunctionResponse is nil")
		}

		if part.FunctionResponse.ID != "call_1" {
			t.Errorf("ID = %q, want %q", part.FunctionResponse.ID, "call_1")
		}

		if part.FunctionResponse.Name != "search" {
			t.Errorf("Name = %q, want %q", part.FunctionResponse.Name, "search")
		}

		if part.FunctionResponse.Response["output"] != "result output" {
			t.Errorf("Response[output] = %v, want %q", part.FunctionResponse.Response["output"], "result output")
		}

		if _, hasError := part.FunctionResponse.Response["error"]; hasError {
			t.Error("error key present, want absent")
		}
	})

	t.Run("tool result error", func(t *testing.T) {
		call := kit.ToolCall{ID: "call_1", Name: "search"}
		content := kit.NewToolResultContent(kit.NewToolResult(call, kit.ToolOutput{}, fmt.Errorf("tool failed")))

		part, ok := convertRequestContentItem(content)

		if !ok {
			t.Fatal("ok = false, want true")
		}

		if part.FunctionResponse == nil {
			t.Fatal("FunctionResponse is nil")
		}

		if part.FunctionResponse.Response["error"] != "tool failed" {
			t.Errorf("Response[error] = %v, want %q", part.FunctionResponse.Response["error"], "tool failed")
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

		if result.Role != genai.RoleUser {
			t.Errorf("Role = %q, want %q", result.Role, genai.RoleUser)
		}
	})

	t.Run("model message produces model role", func(t *testing.T) {
		msg := kit.NewModelMessage(kit.NewTextContent("hello"))

		result := convertRequestMessage(msg)

		if result.Role != genai.RoleModel {
			t.Errorf("Role = %q, want %q", result.Role, genai.RoleModel)
		}
	})

	t.Run("tool message produces user role", func(t *testing.T) {
		call := kit.ToolCall{ID: "call_1", Name: "fn"}
		msg := kit.NewToolMessage(
			kit.NewToolResultContent(kit.NewToolResult(call, kit.NewToolOutput(kit.NewTextContent("ok")), nil)))

		result := convertRequestMessage(msg)

		if result.Role != genai.RoleUser {
			t.Errorf("Role = %q, want %q", result.Role, genai.RoleUser)
		}
	})
}

func TestConvertResponseContent(t *testing.T) {
	t.Run("text part", func(t *testing.T) {
		part := genai.NewPartFromText("Hello!")

		got := convertResponseContent(part)

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

	t.Run("thinking part", func(t *testing.T) {
		part := &genai.Part{
			Thought:          true,
			Text:             "I think...",
			ThoughtSignature: []byte("sig_xyz"),
		}

		got := convertResponseContent(part)

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

		if th.Signature != "c2lnX3h5eg==" {
			t.Errorf("Signature = %q, want %q", th.Signature, "c2lnX3h5eg==")
		}
	})

	t.Run("function_call part", func(t *testing.T) {
		part := genai.NewPartFromFunctionCall("search", map[string]any{"query": "Go"})
		part.FunctionCall.ID = "call_1"
		part.ThoughtSignature = []byte("sig_tool")

		got := convertResponseContent(part)

		if len(got) != 1 {
			t.Fatalf("len = %d, want 1", len(got))
		}

		if got[0].Type != kit.ContentTypeToolCall {
			t.Errorf("Type = %q, want %q", got[0].Type, kit.ContentTypeToolCall)
		}

		tc := got[0].ToolCall
		if tc.ID != "call_1" {
			t.Errorf("ID = %q, want %q", tc.ID, "call_1")
		}

		if tc.Name != "search" {
			t.Errorf("Name = %q, want %q", tc.Name, "search")
		}

		if tc.Arguments["query"] != "Go" {
			t.Errorf("Arguments[query] = %v, want %q", tc.Arguments["query"], "Go")
		}

		if tc.Signature != "c2lnX3Rvb2w=" {
			t.Errorf("Signature = %q, want %q", tc.Signature, "c2lnX3Rvb2w=")
		}
	})

	t.Run("inline image part", func(t *testing.T) {
		part := genai.NewPartFromBytes([]byte{1, 2, 3}, "image/png")

		got := convertResponseContent(part)

		if len(got) != 1 {
			t.Fatalf("len = %d, want 1", len(got))
		}

		if got[0].Type != kit.ContentTypeImage {
			t.Fatalf("Type = %q, want image", got[0].Type)
		}

		if got[0].Image.MIMEType != "image/png" {
			t.Errorf("MIMEType = %q, want image/png", got[0].Image.MIMEType)
		}

		if !bytes.Equal(got[0].Image.Data, []byte{1, 2, 3}) {
			t.Errorf("Data = %v, want [1 2 3]", got[0].Image.Data)
		}
	})

	t.Run("empty part returns nil", func(t *testing.T) {
		part := &genai.Part{}

		if got := convertResponseContent(part); len(got) != 0 {
			t.Errorf("expected nil/empty, got %v", got)
		}
	})
}

func TestConvertResponseFinishReason(t *testing.T) {
	tests := []struct {
		name      string
		candidate *genai.Candidate
		want      kit.FinishReason
	}{
		{
			name:      "STOP maps to stop",
			candidate: &genai.Candidate{FinishReason: genai.FinishReasonStop},
			want:      kit.FinishReasonStop,
		},
		{
			name:      "MAX_TOKENS maps to max_tokens",
			candidate: &genai.Candidate{FinishReason: genai.FinishReasonMaxTokens},
			want:      kit.FinishReasonMaxTokens,
		},
		{
			name:      "SAFETY maps to safety",
			candidate: &genai.Candidate{FinishReason: genai.FinishReasonSafety},
			want:      kit.FinishReasonSafety,
		},
		{
			name:      "OTHER maps to unknown",
			candidate: &genai.Candidate{FinishReason: genai.FinishReasonOther},
			want:      kit.FinishReasonUnknown,
		},
		{
			name: "function_call in parts overrides finish reason",
			candidate: &genai.Candidate{
				FinishReason: genai.FinishReasonStop,
				Content: &genai.Content{
					Parts: []*genai.Part{
						genai.NewPartFromFunctionCall("search", nil),
					},
				},
			},
			want: kit.FinishReasonToolCall,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := convertResponseFinishReason(tt.candidate); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestConvertResponseUsage(t *testing.T) {
	resp := &genai.GenerateContentResponse{
		UsageMetadata: &genai.GenerateContentResponseUsageMetadata{
			PromptTokenCount:        100,
			CandidatesTokenCount:    50,
			CachedContentTokenCount: 30,
			ThoughtsTokenCount:      20,
		},
	}

	got := convertResponseUsage(resp)

	if got.InputTokens != 100 {
		t.Errorf("InputTokens = %d, want 100", got.InputTokens)
	}

	if got.OutputTokens != 50 {
		t.Errorf("OutputTokens = %d, want 50", got.OutputTokens)
	}

	if got.CacheReadTokens != 30 {
		t.Errorf("CacheReadTokens = %d, want 30", got.CacheReadTokens)
	}

	if got.ReasoningTokens != 20 {
		t.Errorf("ReasoningTokens = %d, want 20", got.ReasoningTokens)
	}
}

func TestConvertResponseUsageNilMetadata(t *testing.T) {
	resp := &genai.GenerateContentResponse{}

	got := convertResponseUsage(resp)

	if got != (kit.Usage{}) {
		t.Errorf("expected zero Usage, got %+v", got)
	}
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
			err:     genai.APIError{Code: 401, Message: "unauthorized"},
			wantErr: kit.ErrAuthentication,
		},
		{
			name:    "403 maps to ErrAuthentication",
			err:     genai.APIError{Code: 403, Message: "forbidden"},
			wantErr: kit.ErrAuthentication,
		},
		{
			name:    "429 maps to ErrRateLimit",
			err:     genai.APIError{Code: 429, Message: "rate limited"},
			wantErr: kit.ErrRateLimit,
		},
		{
			name:    "400 maps to ErrInvalidRequest",
			err:     genai.APIError{Code: 400, Message: "bad request"},
			wantErr: kit.ErrInvalidRequest,
		},
		{
			name:    "400 with prompt token count message maps to ErrContextLength",
			err:     genai.APIError{Code: 400, Message: "Prompt token count (102400) exceeds model context limit (32768)."},
			wantErr: kit.ErrContextLength,
		},
		{
			name:    "500 maps to ErrServerError",
			err:     genai.APIError{Code: 500, Message: "internal error"},
			wantErr: kit.ErrServerError,
		},
		{
			name:    "502 maps to ErrServerError",
			err:     genai.APIError{Code: 502, Message: "bad gateway"},
			wantErr: kit.ErrServerError,
		},
		{
			name:    "503 maps to ErrServerError",
			err:     genai.APIError{Code: 503, Message: "unavailable"},
			wantErr: kit.ErrServerError,
		},
		{
			name:    "504 maps to ErrServerError",
			err:     genai.APIError{Code: 504, Message: "timeout"},
			wantErr: kit.ErrServerError,
		},
		{
			name:    "unrecognised status passes through",
			err:     genai.APIError{Code: 422, Message: "unprocessable"},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapError(tt.err)

			if tt.wantErr == nil {
				if got.Error() != tt.err.Error() {
					t.Errorf("expected original error %v, got %v", tt.err, got)
				}

				return
			}

			if !errors.Is(got, tt.wantErr) {
				t.Errorf("errors.Is(%v, %v) = false", got, tt.wantErr)
			}
		})
	}
}

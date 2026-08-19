package openai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/openai/openai-go/v3/responses"

	openaisdk "github.com/openai/openai-go/v3"

	"github.com/vitaliiPsl/crappy-adk/kit"
	"github.com/vitaliiPsl/crappy-adk/providers"
)

func TestModelResolvesCredentialsForEveryRequest(t *testing.T) {
	var authorization []string

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		authorization = append(authorization, request.Header.Get("Authorization"))
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"status":"completed","output":[]}`))
	}))
	t.Cleanup(server.Close)

	source := &rotatingCredentialsSource{}

	model, err := New(
		"test-model",
		providers.WithBaseURL(server.URL),
		providers.WithBearerToken("static-token"),
		providers.WithHeaders(map[string]string{"Authorization": "Bearer static-header"}),
		providers.WithCredentialsSource(source),
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
}

func TestBuildRequestParamsStructuredOutput(t *testing.T) {
	schema := map[string]any{"type": "object"}
	params := buildRequestParams("test-model", kit.ModelRequest{
		OutputSchema: &kit.OutputSchema{
			Name:        "assessment",
			Description: "Completed assessment",
			Schema:      schema,
		},
	})

	format := params.Text.Format.OfJSONSchema
	if format == nil {
		t.Fatal("structured output format is nil")
	}

	if format.Name != "assessment" || format.Description.Value != "Completed assessment" {
		t.Fatalf("format metadata = %+v", format)
	}

	if !format.Strict.Value {
		t.Fatal("strict = false, want true")
	}
}

type rotatingCredentialsSource struct {
	requests int
}

func (s *rotatingCredentialsSource) Headers(context.Context) (http.Header, error) {
	s.requests++

	return http.Header{
		"Authorization": []string{fmt.Sprintf("Bearer token-%d", s.requests)},
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

func mustParseOutputItem(t *testing.T, data string) responses.ResponseOutputItemUnion {
	t.Helper()

	var item responses.ResponseOutputItemUnion
	if err := json.Unmarshal([]byte(data), &item); err != nil {
		t.Fatalf("unmarshal output item: %v", err)
	}

	return item
}

func mustParseResponse(t *testing.T, data string) *responses.Response {
	t.Helper()

	var resp responses.Response
	if err := json.Unmarshal([]byte(data), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	return &resp
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
		schema := map[string]any{"type": "object"}
		tool := &mockTool{name: "search", description: "search the web", schema: schema}

		result := convertRequestTools([]kit.Tool{tool})

		if len(result) != 1 {
			t.Fatalf("len = %d, want 1", len(result))
		}

		fn := result[0].OfFunction
		if fn == nil {
			t.Fatal("OfFunction is nil")
		}

		if fn.Name != "search" {
			t.Errorf("Name = %q, want %q", fn.Name, "search")
		}

		if fn.Description.Value != "search the web" {
			t.Errorf("Description = %q, want %q", fn.Description.Value, "search the web")
		}

		if fn.Parameters == nil {
			t.Error("Parameters is nil")
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
			if got := result[i].OfFunction.Name; got != want {
				t.Errorf("result[%d].Name = %q, want %q", i, got, want)
			}
		}
	})
}

func TestConvertRequestContentListItem(t *testing.T) {
	t.Run("image bytes", func(t *testing.T) {
		item, ok := convertRequestContentListItem(kit.NewImageContent("image/png", []byte{1, 2, 3}))

		if !ok {
			t.Fatal("ok = false, want true")
		}

		if item.OfInputImage == nil {
			t.Fatal("OfInputImage is nil")
		}

		if item.OfInputImage.ImageURL.Value != "data:image/png;base64,AQID" {
			t.Errorf("ImageURL = %q, want data URL", item.OfInputImage.ImageURL.Value)
		}
	})

	t.Run("resource blob", func(t *testing.T) {
		item, ok := convertRequestContentListItem(kit.NewResourceContent(kit.Resource{
			Name:     "report.pdf",
			MIMEType: "application/pdf",
			Blob:     []byte{1, 2, 3},
		}))

		if !ok {
			t.Fatal("ok = false, want true")
		}

		if item.OfInputFile == nil {
			t.Fatal("OfInputFile is nil")
		}

		if item.OfInputFile.FileData.Value != "AQID" {
			t.Errorf("FileData = %q, want base64 file data", item.OfInputFile.FileData.Value)
		}

		if item.OfInputFile.Filename.Value != "report.pdf" {
			t.Errorf("Filename = %q, want report.pdf", item.OfInputFile.Filename.Value)
		}
	})

	t.Run("audio falls back to text", func(t *testing.T) {
		item, ok := convertRequestContentListItem(kit.NewAudioContent("audio/mpeg", []byte{1, 2, 3}))

		if !ok {
			t.Fatal("ok = false, want true")
		}

		if item.OfInputText == nil {
			t.Fatal("OfInputText is nil")
		}

		if item.OfInputText.Text != "[audio: audio/mpeg, 3 bytes]" {
			t.Errorf("Text = %q, want audio fallback", item.OfInputText.Text)
		}
	})
}

func TestConvertResponseContent(t *testing.T) {
	t.Run("message single text part", func(t *testing.T) {
		item := mustParseOutputItem(t, `{
			"type": "message", "id": "msg_1", "role": "assistant", "status": "completed",
			"content": [{"type": "output_text", "text": "Hello!"}]
		}`)

		got := convertResponseContent(item)

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

	t.Run("message multiple text parts", func(t *testing.T) {
		item := mustParseOutputItem(t, `{
			"type": "message", "id": "msg_1", "role": "assistant", "status": "completed",
			"content": [
				{"type": "output_text", "text": "Hello"},
				{"type": "output_text", "text": " world"}
			]
		}`)

		got := convertResponseContent(item)

		if len(got) != 2 {
			t.Fatalf("len = %d, want 2", len(got))
		}

		if got[0].Text.Text != "Hello" {
			t.Errorf("got[0].Text = %q, want %q", got[0].Text.Text, "Hello")
		}

		if got[1].Text.Text != " world" {
			t.Errorf("got[1].Text = %q, want %q", got[1].Text.Text, " world")
		}
	})

	t.Run("message with no text parts", func(t *testing.T) {
		item := mustParseOutputItem(t, `{
			"type": "message", "id": "msg_1", "role": "assistant", "status": "completed",
			"content": []
		}`)

		if got := convertResponseContent(item); len(got) != 0 {
			t.Errorf("expected empty, got %v", got)
		}
	})

	t.Run("reasoning", func(t *testing.T) {
		item := mustParseOutputItem(t, `{
			"type": "reasoning", "id": "reasoning_1",
			"summary": [{"type": "summary_text", "text": "I think..."}],
			"encrypted_content": "sig_xyz"
		}`)

		got := convertResponseContent(item)

		if len(got) != 1 {
			t.Fatalf("len = %d, want 1", len(got))
		}

		if got[0].Type != kit.ContentTypeThinking {
			t.Errorf("Type = %q, want %q", got[0].Type, kit.ContentTypeThinking)
		}

		th := got[0].Thinking
		if th.ID != "reasoning_1" {
			t.Errorf("ID = %q, want %q", th.ID, "reasoning_1")
		}

		if th.Text != "I think..." {
			t.Errorf("Text = %q, want %q", th.Text, "I think...")
		}

		if th.Signature != "sig_xyz" {
			t.Errorf("Signature = %q, want %q", th.Signature, "sig_xyz")
		}
	})

	t.Run("function_call", func(t *testing.T) {
		item := mustParseOutputItem(t, `{
			"type": "function_call", "id": "item_1",
			"call_id": "call_abc", "name": "add",
			"arguments": "{\"a\": 1, \"b\": 2}"
		}`)

		got := convertResponseContent(item)

		if len(got) != 1 {
			t.Fatalf("len = %d, want 1", len(got))
		}

		if got[0].Type != kit.ContentTypeToolCall {
			t.Errorf("Type = %q, want %q", got[0].Type, kit.ContentTypeToolCall)
		}

		tc := got[0].ToolCall
		if tc.ID != "call_abc" {
			t.Errorf("ID = %q, want %q", tc.ID, "call_abc")
		}

		if tc.Name != "add" {
			t.Errorf("Name = %q, want %q", tc.Name, "add")
		}

		if tc.Arguments["a"] != float64(1) || tc.Arguments["b"] != float64(2) {
			t.Errorf("Arguments = %v, want {a:1, b:2}", tc.Arguments)
		}
	})

	t.Run("unknown type returns nil", func(t *testing.T) {
		item := mustParseOutputItem(t, `{"type": "web_search_call", "id": "ws_1"}`)
		if got := convertResponseContent(item); len(got) != 0 {
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
			name: "completed without tool calls",
			json: `{"status": "completed", "output": [{"type": "message", "id": "m1", "role": "assistant", "status": "completed", "content": []}]}`,
			want: kit.FinishReasonStop,
		},
		{
			name: "completed with function call",
			json: `{"status": "completed", "output": [{"type": "function_call", "id": "fc1", "call_id": "c1", "name": "fn", "arguments": "{}"}]}`,
			want: kit.FinishReasonToolCall,
		},
		{
			name: "incomplete max_output_tokens",
			json: `{"status": "incomplete", "incomplete_details": {"reason": "max_output_tokens"}, "output": []}`,
			want: kit.FinishReasonMaxTokens,
		},
		{
			name: "incomplete content_filter",
			json: `{"status": "incomplete", "incomplete_details": {"reason": "content_filter"}, "output": []}`,
			want: kit.FinishReasonSafety,
		},
		{
			name: "incomplete unknown reason",
			json: `{"status": "incomplete", "incomplete_details": {"reason": "something_else"}, "output": []}`,
			want: kit.FinishReasonUnknown,
		},
		{
			name: "failed status",
			json: `{"status": "failed", "output": []}`,
			want: kit.FinishReasonUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := mustParseResponse(t, tt.json)
			if got := convertResponseFinishReason(resp); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestConvertResponseUsage(t *testing.T) {
	resp := mustParseResponse(t, `{
		"status": "completed",
		"output": [],
		"usage": {
			"input_tokens": 100,
			"output_tokens": 50,
			"total_tokens": 150,
			"input_tokens_details": {"cached_tokens": 30},
			"output_tokens_details": {"reasoning_tokens": 10}
		}
	}`)

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

	if got.ReasoningTokens != 10 {
		t.Errorf("ReasoningTokens = %d, want 10", got.ReasoningTokens)
	}
}

func makeAPIError(statusCode int, code, message string) *openaisdk.Error {
	return &openaisdk.Error{
		StatusCode: statusCode,
		Code:       code,
		Message:    message,
		Request:    &http.Request{},
		Response:   &http.Response{StatusCode: statusCode},
	}
}

func makeAPIErrorWithBody(t *testing.T, statusCode int, body string) *openaisdk.Error {
	t.Helper()

	err := makeAPIError(statusCode, "", "")
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
			err:     makeAPIError(401, "", "invalid api key"),
			wantErr: kit.ErrAuthentication,
		},
		{
			name:    "403 maps to ErrAuthentication",
			err:     makeAPIError(403, "", "forbidden"),
			wantErr: kit.ErrAuthentication,
		},
		{
			name:    "429 maps to ErrRateLimit",
			err:     makeAPIError(429, "", "rate limit exceeded"),
			wantErr: kit.ErrRateLimit,
		},
		{
			name:    "400 context_length_exceeded maps to ErrContextLength",
			err:     makeAPIError(400, "context_length_exceeded", "too many tokens"),
			wantErr: kit.ErrContextLength,
		},
		{
			name:    "400 other code maps to ErrInvalidRequest",
			err:     makeAPIError(400, "invalid_request_error", "bad param"),
			wantErr: kit.ErrInvalidRequest,
		},
		{
			name:    "500 maps to ErrServerError",
			err:     makeAPIError(500, "", "internal server error"),
			wantErr: kit.ErrServerError,
		},
		{
			name:    "502 maps to ErrServerError",
			err:     makeAPIError(502, "", "bad gateway"),
			wantErr: kit.ErrServerError,
		},
		{
			name:    "503 maps to ErrServerError",
			err:     makeAPIError(503, "", "service unavailable"),
			wantErr: kit.ErrServerError,
		},
		{
			name:    "504 maps to ErrServerError",
			err:     makeAPIError(504, "", "gateway timeout"),
			wantErr: kit.ErrServerError,
		},
		{
			name:    "unrecognised status passes through",
			err:     makeAPIError(422, "", "unprocessable"),
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

func TestMapErrorUsesResponseDetailWhenMessageIsEmpty(t *testing.T) {
	err := makeAPIErrorWithBody(t, http.StatusBadRequest, `{"detail":"Store must be set to false"}`)

	got := mapError(err)

	if got.Error() != "invalid request: Store must be set to false" {
		t.Fatalf("mapError() = %q", got)
	}
}

func TestMapErrorFallsBackToHTTPStatus(t *testing.T) {
	got := mapError(makeAPIError(http.StatusBadRequest, "", ""))

	if got.Error() != "invalid request: Bad Request" {
		t.Fatalf("mapError() = %q", got)
	}
}

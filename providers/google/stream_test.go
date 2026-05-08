package google

import (
	"testing"

	"google.golang.org/genai"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

type recordModelEvents struct {
	events []kit.ModelEvent
}

func (r *recordModelEvents) Emit(event kit.ModelEvent) error {
	r.events = append(r.events, event)

	return nil
}

func textChunk(text string) *genai.GenerateContentResponse {
	return &genai.GenerateContentResponse{
		Candidates: []*genai.Candidate{{
			Content: &genai.Content{
				Role:  genai.RoleModel,
				Parts: []*genai.Part{{Text: text}},
			},
		}},
	}
}

func thinkingChunk(text string) *genai.GenerateContentResponse {
	return &genai.GenerateContentResponse{
		Candidates: []*genai.Candidate{{
			Content: &genai.Content{
				Role:  genai.RoleModel,
				Parts: []*genai.Part{{Text: text, Thought: true}},
			},
		}},
	}
}

func toolCallChunk(id, name string, args map[string]any) *genai.GenerateContentResponse {
	return &genai.GenerateContentResponse{
		Candidates: []*genai.Candidate{{
			Content: &genai.Content{
				Role:  genai.RoleModel,
				Parts: []*genai.Part{{FunctionCall: &genai.FunctionCall{ID: id, Name: name, Args: args}}},
			},
		}},
	}
}

func finalChunk(reason genai.FinishReason, inputTokens, outputTokens int32) *genai.GenerateContentResponse {
	return &genai.GenerateContentResponse{
		Candidates: []*genai.Candidate{{
			FinishReason: reason,
		}},
		UsageMetadata: &genai.GenerateContentResponseUsageMetadata{
			PromptTokenCount:     inputTokens,
			CandidatesTokenCount: outputTokens,
		},
	}
}

func processChunks(t *testing.T, chunks []*genai.GenerateContentResponse, events *recordModelEvents) kit.ModelResponse {
	t.Helper()

	var acc streamAcc
	for _, chunk := range chunks {
		if err := acc.handleChunk(chunk, events); err != nil {
			t.Fatalf("handle chunk: %v", err)
		}
	}

	if err := acc.flush(events); err != nil {
		t.Fatalf("flush: %v", err)
	}

	return acc.result()
}

func TestHandleChunk_TextLifecycle(t *testing.T) {
	events := &recordModelEvents{}

	chunks := []*genai.GenerateContentResponse{
		textChunk("Hel"),
		textChunk("lo"),
		finalChunk(genai.FinishReasonStop, 10, 2),
	}

	result := processChunks(t, chunks, events)

	wantTypes := []kit.EventType{
		kit.EventContentStarted,
		kit.EventContentDelta,
		kit.EventContentDelta,
		kit.EventContentDone,
	}
	if len(events.events) != len(wantTypes) {
		t.Fatalf("events len = %d, want %d", len(events.events), len(wantTypes))
	}

	for i, want := range wantTypes {
		if got := events.events[i].Type; got != want {
			t.Errorf("events[%d].Type = %q, want %q", i, got, want)
		}
	}

	if got := events.events[1].Content.Text.Text; got != "Hel" {
		t.Errorf("first delta text = %q, want %q", got, "Hel")
	}

	if got := events.events[3].Content.Text.Text; got != "Hello" {
		t.Errorf("done text = %q, want %q", got, "Hello")
	}

	if result.FinishReason != kit.FinishReasonStop {
		t.Errorf("FinishReason = %q, want %q", result.FinishReason, kit.FinishReasonStop)
	}

	if got := result.Message.TextContent().Text; got != "Hello" {
		t.Errorf("TextContent = %q, want %q", got, "Hello")
	}

	if result.Usage.InputTokens != 10 || result.Usage.OutputTokens != 2 {
		t.Errorf("Usage = %+v, want input=10 output=2", result.Usage)
	}
}

func TestHandleChunk_ThinkingLifecycle(t *testing.T) {
	events := &recordModelEvents{}

	thinkChunkWithSig := &genai.GenerateContentResponse{
		Candidates: []*genai.Candidate{{
			Content: &genai.Content{
				Role:  genai.RoleModel,
				Parts: []*genai.Part{{Text: " think", Thought: true, ThoughtSignature: []byte("sig_abc")}},
			},
		}},
	}

	chunks := []*genai.GenerateContentResponse{
		thinkingChunk("Let me"),
		thinkChunkWithSig,
		finalChunk(genai.FinishReasonStop, 5, 10),
	}

	result := processChunks(t, chunks, events)

	wantTypes := []kit.EventType{
		kit.EventContentStarted,
		kit.EventContentDelta,
		kit.EventContentDelta,
		kit.EventContentDone,
	}
	if len(events.events) != len(wantTypes) {
		t.Fatalf("events len = %d, want %d", len(events.events), len(wantTypes))
	}

	for i, want := range wantTypes {
		if got := events.events[i].Type; got != want {
			t.Errorf("events[%d].Type = %q, want %q", i, got, want)
		}
	}

	if got := events.events[1].Content.Thinking.Text; got != "Let me" {
		t.Errorf("first delta thinking = %q, want %q", got, "Let me")
	}

	if got := events.events[3].Content.Thinking.Signature; got != "" {
		t.Errorf("done event signature = %q, want empty (signature only in final response)", got)
	}

	thinking := result.Message.ThinkingContent()
	if thinking == nil {
		t.Fatal("result message has no thinking content")
	}

	if thinking.Text != "Let me think" {
		t.Errorf("result thinking text = %q, want %q", thinking.Text, "Let me think")
	}

	if thinking.Signature != "sig_abc" {
		t.Errorf("result thinking signature = %q, want %q", thinking.Signature, "sig_abc")
	}
}

func TestHandleChunk_FunctionCallLifecycle(t *testing.T) {
	events := &recordModelEvents{}

	chunks := []*genai.GenerateContentResponse{
		toolCallChunk("call_abc", "add", map[string]any{"a": float64(1)}),
		finalChunk(genai.FinishReasonStop, 8, 5),
	}

	result := processChunks(t, chunks, events)

	if len(events.events) != 2 {
		t.Fatalf("events len = %d, want 2", len(events.events))
	}

	if got := events.events[0].Type; got != kit.EventContentStarted {
		t.Errorf("events[0].Type = %q, want %q", got, kit.EventContentStarted)
	}

	if got := events.events[1].Type; got != kit.EventContentDone {
		t.Errorf("events[1].Type = %q, want %q", got, kit.EventContentDone)
	}

	started := kit.NewModelMessage([]kit.Content{*events.events[0].Content})

	startedCalls := started.ToolCalls()
	if len(startedCalls) != 1 {
		t.Fatalf("started ToolCalls len = %d, want 1", len(startedCalls))
	}

	done := kit.NewModelMessage([]kit.Content{*events.events[1].Content})

	calls := done.ToolCalls()
	if len(calls) != 1 {
		t.Fatalf("ToolCalls len = %d, want 1", len(calls))
	}

	if calls[0].ID != "call_abc" || calls[0].Name != "add" {
		t.Errorf("ToolCall = %+v, want id=call_abc name=add", calls[0])
	}

	if calls[0].Arguments["a"] != float64(1) {
		t.Errorf("Arguments = %v, want a=1", calls[0].Arguments)
	}

	if result.FinishReason != kit.FinishReasonToolCall {
		t.Errorf("FinishReason = %q, want %q", result.FinishReason, kit.FinishReasonToolCall)
	}
}

func TestHandleChunk_ThinkingThenText(t *testing.T) {
	events := &recordModelEvents{}

	chunks := []*genai.GenerateContentResponse{
		thinkingChunk("I think"),
		textChunk("Hello"),
		finalChunk(genai.FinishReasonStop, 5, 5),
	}

	result := processChunks(t, chunks, events)

	wantTypes := []kit.EventType{
		kit.EventContentStarted, // thinking started
		kit.EventContentDelta,   // thinking delta
		kit.EventContentDone,    // thinking done
		kit.EventContentStarted, // text started
		kit.EventContentDelta,   // text delta
		kit.EventContentDone,    // text done
	}
	if len(events.events) != len(wantTypes) {
		t.Fatalf("events len = %d, want %d", len(events.events), len(wantTypes))
	}

	for i, want := range wantTypes {
		if got := events.events[i].Type; got != want {
			t.Errorf("events[%d].Type = %q, want %q", i, got, want)
		}
	}

	thinking := result.Message.ThinkingContent()
	if thinking == nil || thinking.Text != "I think" {
		t.Errorf("thinking text = %q, want %q", thinking.Text, "I think")
	}

	text := result.Message.TextContent()
	if text == nil || text.Text != "Hello" {
		t.Errorf("text = %q, want %q", text.Text, "Hello")
	}
}

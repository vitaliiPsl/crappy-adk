package anthropic

import (
	"encoding/json"
	"testing"

	anthropicsdk "github.com/anthropics/anthropic-sdk-go"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

type recordModelEvents struct {
	events []kit.ModelEvent
}

func (r *recordModelEvents) Emit(event kit.ModelEvent) error {
	r.events = append(r.events, event)

	return nil
}

func mustParseStreamEvent(t *testing.T, data string) anthropicsdk.MessageStreamEventUnion {
	t.Helper()

	var event anthropicsdk.MessageStreamEventUnion
	if err := json.Unmarshal([]byte(data), &event); err != nil {
		t.Fatalf("unmarshal stream event: %v", err)
	}

	return event
}

func processEvents(t *testing.T, input []string, events *recordModelEvents) anthropicsdk.Message {
	t.Helper()

	var acc anthropicsdk.Message
	for _, data := range input {
		event := mustParseStreamEvent(t, data)

		if err := acc.Accumulate(event); err != nil {
			t.Fatalf("accumulate: %v", err)
		}

		if err := handleStreamEvent(event, &acc, events); err != nil {
			t.Fatalf("handle stream event: %v", err)
		}
	}

	return acc
}

func TestHandleStreamEvent_TextLifecycle(t *testing.T) {
	events := &recordModelEvents{}

	input := []string{
		`{"type": "message_start", "message": {"id": "msg_1", "type": "message", "role": "assistant", "content": [], "model": "claude-opus-4-5", "stop_reason": null, "stop_sequence": null, "usage": {"input_tokens": 10, "output_tokens": 0}}}`,
		`{"type": "content_block_start", "index": 0, "content_block": {"type": "text", "text": ""}}`,
		`{"type": "content_block_delta", "index": 0, "delta": {"type": "text_delta", "text": "Hel"}}`,
		`{"type": "content_block_delta", "index": 0, "delta": {"type": "text_delta", "text": "lo"}}`,
		`{"type": "content_block_stop", "index": 0}`,
		`{"type": "message_delta", "delta": {"stop_reason": "end_turn", "stop_sequence": null}, "usage": {"output_tokens": 2}}`,
		`{"type": "message_stop"}`,
	}

	acc := processEvents(t, input, events)

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

	result := convertResponse(&acc)
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

func TestHandleStreamEvent_ThinkingLifecycle(t *testing.T) {
	events := &recordModelEvents{}

	input := []string{
		`{"type": "message_start", "message": {"id": "msg_1", "type": "message", "role": "assistant", "content": [], "model": "claude-opus-4-5", "stop_reason": null, "stop_sequence": null, "usage": {"input_tokens": 5, "output_tokens": 0}}}`,
		`{"type": "content_block_start", "index": 0, "content_block": {"type": "thinking", "thinking": "", "signature": ""}}`,
		`{"type": "content_block_delta", "index": 0, "delta": {"type": "thinking_delta", "thinking": "Let me"}}`,
		`{"type": "content_block_delta", "index": 0, "delta": {"type": "thinking_delta", "thinking": " think"}}`,
		`{"type": "content_block_delta", "index": 0, "delta": {"type": "signature_delta", "signature": "sig_abc"}}`,
		`{"type": "content_block_stop", "index": 0}`,
		`{"type": "message_delta", "delta": {"stop_reason": "end_turn", "stop_sequence": null}, "usage": {"output_tokens": 10}}`,
		`{"type": "message_stop"}`,
	}

	acc := processEvents(t, input, events)

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

	doneEvent := events.events[3]
	if got := doneEvent.Content.Thinking.Signature; got != "" {
		t.Errorf("done event signature = %q, want empty (signature only in final response)", got)
	}

	result := convertResponse(&acc)

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

func TestHandleStreamEvent_FunctionCallLifecycle(t *testing.T) {
	events := &recordModelEvents{}

	input := []string{
		`{"type": "message_start", "message": {"id": "msg_1", "type": "message", "role": "assistant", "content": [], "model": "claude-opus-4-5", "stop_reason": null, "stop_sequence": null, "usage": {"input_tokens": 8, "output_tokens": 0}}}`,
		`{"type": "content_block_start", "index": 0, "content_block": {"type": "tool_use", "id": "call_abc", "name": "add", "input": {}}}`,
		`{"type": "content_block_delta", "index": 0, "delta": {"type": "input_json_delta", "partial_json": "{\"a\":"}}`,
		`{"type": "content_block_delta", "index": 0, "delta": {"type": "input_json_delta", "partial_json": "1}"}}`,
		`{"type": "content_block_stop", "index": 0}`,
		`{"type": "message_delta", "delta": {"stop_reason": "tool_use", "stop_sequence": null}, "usage": {"output_tokens": 5}}`,
		`{"type": "message_stop"}`,
	}

	acc := processEvents(t, input, events)

	if len(events.events) != 2 {
		t.Fatalf("events len = %d, want 2", len(events.events))
	}

	if got := events.events[0].Type; got != kit.EventContentStarted {
		t.Errorf("events[0].Type = %q, want %q", got, kit.EventContentStarted)
	}

	if got := events.events[1].Type; got != kit.EventContentDone {
		t.Errorf("events[1].Type = %q, want %q", got, kit.EventContentDone)
	}

	started := kit.NewModelMessage(*events.events[0].Content)

	startedCalls := started.ToolCalls()
	if len(startedCalls) != 1 {
		t.Fatalf("started ToolCalls len = %d, want 1", len(startedCalls))
	}

	if startedCalls[0].Arguments != nil {
		t.Errorf("started Arguments = %v, want nil", startedCalls[0].Arguments)
	}

	done := kit.NewModelMessage(*events.events[1].Content)

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

	result := convertResponse(&acc)
	if result.FinishReason != kit.FinishReasonToolCall {
		t.Errorf("FinishReason = %q, want %q", result.FinishReason, kit.FinishReasonToolCall)
	}
}

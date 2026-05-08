package openai

import (
	"encoding/json"
	"testing"

	"github.com/openai/openai-go/v3/responses"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

type recordModelEvents struct {
	events []kit.ModelEvent
}

func (r *recordModelEvents) Emit(event kit.ModelEvent) error {
	r.events = append(r.events, event)

	return nil
}

func mustParseStreamEvent(t *testing.T, data string) responses.ResponseStreamEventUnion {
	t.Helper()

	var event responses.ResponseStreamEventUnion
	if err := json.Unmarshal([]byte(data), &event); err != nil {
		t.Fatalf("unmarshal stream event: %v", err)
	}

	return event
}

func TestHandleStreamEvent_TextLifecycle(t *testing.T) {
	events := &recordModelEvents{}

	var result kit.ModelResponse

	input := []string{
		`{
			"type": "response.content_part.added",
			"sequence_number": 1,
			"item_id": "msg_1",
			"output_index": 0,
			"content_index": 0,
			"part": {"type": "output_text", "text": ""}
		}`,
		`{
			"type": "response.output_text.delta",
			"sequence_number": 2,
			"item_id": "msg_1",
			"output_index": 0,
			"content_index": 0,
			"delta": "Hel",
			"logprobs": []
		}`,
		`{
			"type": "response.output_text.delta",
			"sequence_number": 3,
			"item_id": "msg_1",
			"output_index": 0,
			"content_index": 0,
			"delta": "lo",
			"logprobs": []
		}`,
		`{
			"type": "response.output_text.done",
			"sequence_number": 4,
			"item_id": "msg_1",
			"output_index": 0,
			"content_index": 0,
			"text": "Hello",
			"logprobs": []
		}`,
		`{
			"type": "response.completed",
			"sequence_number": 5,
			"response": {
				"status": "completed",
				"output": [{
					"type": "message",
					"id": "msg_1",
					"role": "assistant",
					"status": "completed",
					"content": [{"type": "output_text", "text": "Hello"}]
				}],
				"usage": {
					"input_tokens": 10,
					"output_tokens": 2,
					"total_tokens": 12,
					"input_tokens_details": {"cached_tokens": 3},
					"output_tokens_details": {"reasoning_tokens": 1}
				}
			}
		}`,
	}

	for _, data := range input {
		event := mustParseStreamEvent(t, data)
		switch event.Type {
		case "response.completed":
			resp := event.AsResponseCompleted().Response
			result = convertResponse(&resp)

			continue
		}

		if err := handleStreamEvent(event.AsAny(), events); err != nil {
			t.Fatalf("handle stream event: %v", err)
		}
	}

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

func TestHandleStreamEvent_PartialTextOnlyEmitsEvents(t *testing.T) {
	events := &recordModelEvents{}

	var result kit.ModelResponse

	input := []string{
		`{
			"type": "response.content_part.added",
			"sequence_number": 1,
			"item_id": "msg_1",
			"output_index": 0,
			"content_index": 0,
			"part": {"type": "output_text", "text": ""}
		}`,
		`{
			"type": "response.output_text.delta",
			"sequence_number": 2,
			"item_id": "msg_1",
			"output_index": 0,
			"content_index": 0,
			"delta": "Hel",
			"logprobs": []
		}`,
	}

	for _, data := range input {
		if err := handleStreamEvent(mustParseStreamEvent(t, data).AsAny(), events); err != nil {
			t.Fatalf("handle stream event: %v", err)
		}
	}

	if result.Message.Content != nil {
		t.Errorf("Message.Content = %v, want nil", result.Message.Content)
	}

	if result.FinishReason != "" {
		t.Errorf("FinishReason = %q, want empty", result.FinishReason)
	}

	wantTypes := []kit.EventType{
		kit.EventContentStarted,
		kit.EventContentDelta,
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
		t.Errorf("delta text = %q, want %q", got, "Hel")
	}
}

func TestHandleStreamEvent_FunctionCallLifecycle(t *testing.T) {
	events := &recordModelEvents{}

	input := []string{
		`{
			"type": "response.output_item.added",
			"sequence_number": 1,
			"output_index": 0,
			"item": {
				"type": "function_call",
				"id": "item_1",
				"call_id": "call_abc",
				"name": "add",
				"arguments": ""
			}
		}`,
		`{
			"type": "response.output_item.done",
			"sequence_number": 2,
			"output_index": 0,
			"item": {
				"type": "function_call",
				"id": "item_1",
				"call_id": "call_abc",
				"name": "add",
				"arguments": "{\"a\":1}"
			}
		}`,
	}

	for _, data := range input {
		if err := handleStreamEvent(mustParseStreamEvent(t, data).AsAny(), events); err != nil {
			t.Fatalf("handle stream event: %v", err)
		}
	}

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

	if startedCalls[0].Arguments != nil {
		t.Errorf("started Arguments = %v, want nil", startedCalls[0].Arguments)
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
}

func TestHandleStreamEvent_ReasoningLifecycle(t *testing.T) {
	events := &recordModelEvents{}

	var result kit.ModelResponse

	input := []string{
		`{
			"type": "response.reasoning_summary_part.added",
			"sequence_number": 1,
			"item_id": "rs_1",
			"output_index": 0,
			"summary_index": 0,
			"part": {"type": "summary_text", "text": ""}
		}`,
		`{
			"type": "response.reasoning_summary_text.delta",
			"sequence_number": 2,
			"item_id": "rs_1",
			"output_index": 0,
			"summary_index": 0,
			"delta": "Let me"
		}`,
		`{
			"type": "response.reasoning_summary_text.delta",
			"sequence_number": 3,
			"item_id": "rs_1",
			"output_index": 0,
			"summary_index": 0,
			"delta": " think"
		}`,
		`{
			"type": "response.reasoning_summary_text.done",
			"sequence_number": 4,
			"item_id": "rs_1",
			"output_index": 0,
			"summary_index": 0,
			"text": "Let me think"
		}`,
		`{
			"type": "response.completed",
			"sequence_number": 5,
			"response": {
				"status": "completed",
				"output": [{
					"type": "reasoning",
					"id": "rs_1",
					"summary": [{"type": "summary_text", "text": "Let me think"}],
					"encrypted_content": "abc123sig"
				}],
				"usage": {
					"input_tokens": 5,
					"output_tokens": 10,
					"total_tokens": 15,
					"input_tokens_details": {"cached_tokens": 0},
					"output_tokens_details": {"reasoning_tokens": 10}
				}
			}
		}`,
	}

	for _, data := range input {
		event := mustParseStreamEvent(t, data)
		if event.Type == "response.completed" {
			resp := event.AsResponseCompleted().Response
			result = convertResponse(&resp)

			continue
		}

		if err := handleStreamEvent(event.AsAny(), events); err != nil {
			t.Fatalf("handle stream event: %v", err)
		}
	}

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
		t.Errorf("first delta text = %q, want %q", got, "Let me")
	}

	if got := events.events[3].Content.Thinking.Text; got != "Let me think" {
		t.Errorf("done text = %q, want %q", got, "Let me think")
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

	if thinking.Signature != "abc123sig" {
		t.Errorf("result thinking signature = %q, want %q", thinking.Signature, "abc123sig")
	}
}

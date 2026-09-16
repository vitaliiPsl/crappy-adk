package kit

import (
	"context"
	"testing"
)

type runContextMemory struct {
	messages []Message
}

type runContextEvents struct {
	events []AgentEvent
}

func (m *runContextMemory) Context(context.Context) ([]Message, error) {
	return m.messages, nil
}

func (m *runContextMemory) History(context.Context) ([]Message, error) {
	return m.messages, nil
}

func (m *runContextMemory) Record(_ context.Context, messages ...Message) error {
	m.messages = append(m.messages, messages...)

	return nil
}

func (e *runContextEvents) Emit(event AgentEvent) error {
	e.events = append(e.events, event)

	return nil
}

func TestRunContextRecordModelResponse(t *testing.T) {
	memory := &runContextMemory{}
	events := &runContextEvents{}
	rc := &RunContext{Context: context.Background(), Memory: memory, Events: events}
	message := NewModelMessage(NewTextContent("done"))
	response := ModelResponse{
		Message:      message,
		Usage:        Usage{InputTokens: 3, OutputTokens: 2},
		FinishReason: FinishReasonStop,
	}

	if err := rc.RecordModelResponse(response); err != nil {
		t.Fatalf("RecordModelResponse: %v", err)
	}

	if len(rc.Messages) != 1 || rc.Messages[0].TextContent().Text != "done" {
		t.Fatalf("Messages = %+v, want recorded message", rc.Messages)
	}

	if len(memory.messages) != 1 || memory.messages[0].TextContent().Text != "done" {
		t.Fatalf("memory messages = %+v, want recorded message", memory.messages)
	}

	if rc.Usage != response.Usage || rc.LastUsage != response.Usage {
		t.Fatalf("usage = %+v, last usage = %+v, want %+v", rc.Usage, rc.LastUsage, response.Usage)
	}

	if rc.FinishReason != FinishReasonStop {
		t.Fatalf("FinishReason = %q, want %q", rc.FinishReason, FinishReasonStop)
	}

	if len(events.events) != 1 || events.events[0].Message == nil {
		t.Fatalf("events = %+v, want model message event", events.events)
	}
}

func TestRunContextRecordToolResult(t *testing.T) {
	memory := &runContextMemory{}
	events := &runContextEvents{}
	rc := &RunContext{Context: context.Background(), Memory: memory, Events: events}
	call := NewToolCall("call-1", "search", nil)
	result := NewToolResult(call, NewToolOutput(NewTextContent("found")), nil)

	if err := rc.RecordToolResult(result); err != nil {
		t.Fatalf("RecordToolResult: %v", err)
	}

	if len(rc.Messages) != 1 || len(rc.Messages[0].ToolResults()) != 1 {
		t.Fatalf("Messages = %+v, want tool-result message", rc.Messages)
	}

	if len(memory.messages) != 1 || len(memory.messages[0].ToolResults()) != 1 {
		t.Fatalf("memory messages = %+v, want tool-result message", memory.messages)
	}

	if len(events.events) != 2 || events.events[0].ToolResult == nil || events.events[1].Message == nil {
		t.Fatalf("events = %+v, want tool-result event followed by message event", events.events)
	}
}

func TestRunContextResponse_OutputComesFromLastModelMessage(t *testing.T) {
	rc := &RunContext{
		Messages: []Message{
			NewUserMessage(NewTextContent("user text")),
			NewModelMessage(NewTextContent("final model")),
		},
	}

	resp := rc.Response()
	if resp.Output == nil {
		t.Fatal("Output is nil")
	}

	if resp.Output.Text != "final model" {
		t.Fatalf("Output = %q, want final model", resp.Output.Text)
	}
}

func TestRunContextResponse_LastNonModelMessageLeavesOutputNil(t *testing.T) {
	rc := &RunContext{
		Messages: []Message{
			NewModelMessage(NewTextContent("older text")),
			NewToolMessage(
				NewToolResultContent(NewToolResult(ToolCall{ID: "call-1", Name: "tool"}, NewToolOutput(NewTextContent("tool output")), nil)),
			),
		},
	}

	resp := rc.Response()
	if resp.Output != nil {
		t.Fatalf("Output = %q, want nil when latest run message is not model", resp.Output.Text)
	}
}

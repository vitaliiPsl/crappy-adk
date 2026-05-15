package summarization

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/vitaliiPsl/crappy-adk/kit"
	"github.com/vitaliiPsl/crappy-adk/kittest"
	"github.com/vitaliiPsl/crappy-adk/x/memory"
)

func TestSummarize_RecordsSummaryAndPreservesHistory(t *testing.T) {
	msgs := []kit.Message{
		kit.NewUserMessage(kit.NewTextContent("turn 1")),
		kit.NewModelMessage(kit.NewTextContent("reply 1")),
		kit.NewUserMessage(kit.NewTextContent("turn 2")),
		kit.NewModelMessage(kit.NewTextContent("reply 2")),
		kit.NewUserMessage(kit.NewTextContent("turn 3")),
		kit.NewModelMessage(kit.NewTextContent("reply 3")),
	}

	summarizer := kittest.NewModel(t, kittest.ModelResult{
		Response: kit.ModelResponse{
			Message:      kit.NewModelMessage(kit.NewTextContent("summary text")),
			FinishReason: kit.FinishReasonStop,
			Usage:        kit.Usage{InputTokens: 11, OutputTokens: 7},
		},
	})

	mem := memory.NewHistory(msgs...)
	events := &recordAgentEvents{}
	rc := &kit.RunContext{
		Context: context.Background(),
		Memory:  mem,
		Usage:   kit.Usage{InputTokens: 100, OutputTokens: 50},
		Events:  events,
	}

	if err := Summarize(summarizer, "summarize this")(rc); err != nil {
		t.Fatalf("Summarize: %v", err)
	}

	history, err := mem.History(context.Background())
	if err != nil {
		t.Fatalf("History: %v", err)
	}

	if got := len(history); got != len(msgs)+1 {
		t.Fatalf("len(History) = %d, want original history + summary", got)
	}

	summaryMsg := history[len(history)-1]

	if summaryMsg.Role != kit.RoleUser {
		t.Fatalf("summary message role = %q, want %q", summaryMsg.Role, kit.RoleUser)
	}

	if summaryMsg.Content[0].Type != kit.ContentTypeSummary {
		t.Fatalf("summary content type = %q, want %q", summaryMsg.Content[0].Type, kit.ContentTypeSummary)
	}

	if summaryMsg.Content[0].Summary.Text != "summary text" {
		t.Fatalf("summary text = %q, want %q", summaryMsg.Content[0].Summary.Text, "summary text")
	}

	if !reflect.DeepEqual(rc.Messages, []kit.Message{summaryMsg}) {
		t.Fatalf("run messages = %+v, want summary message", rc.Messages)
	}

	if len(events.events) != 3 {
		t.Fatalf("len(events) = %d, want 3", len(events.events))
	}

	if events.events[0].Type != kit.EventContentStarted {
		t.Fatalf("event[0] type = %q, want %q", events.events[0].Type, kit.EventContentStarted)
	}

	if events.events[0].Content == nil || events.events[0].Content.Type != kit.ContentTypeSummary {
		t.Fatalf("event[0] content = %+v, want summary content", events.events[0].Content)
	}

	if events.events[1].Type != kit.EventContentDone {
		t.Fatalf("event[1] type = %q, want %q", events.events[1].Type, kit.EventContentDone)
	}

	if events.events[1].Content == nil || !reflect.DeepEqual(*events.events[1].Content, summaryMsg.Content[0]) {
		t.Fatalf("event[1] content = %+v, want summary content", events.events[1].Content)
	}

	if events.events[2].Type != kit.EventMessage {
		t.Fatalf("event[2] type = %q, want %q", events.events[2].Type, kit.EventMessage)
	}

	if events.events[2].Message == nil || !reflect.DeepEqual(*events.events[2].Message, summaryMsg) {
		t.Fatalf("event[2] message = %+v, want summary message", events.events[2].Message)
	}

	contextMessages, err := mem.Context(context.Background())
	if err != nil {
		t.Fatalf("Context: %v", err)
	}

	if len(contextMessages) != 1 {
		t.Fatalf("len(Context) = %d, want latest summary only", len(contextMessages))
	}

	if contextMessages[0].Content[0].Summary.Text != "summary text" {
		t.Fatalf("context summary = %q, want summary text", contextMessages[0].Content[0].Summary.Text)
	}

	if rc.Usage.InputTokens != 111 || rc.Usage.OutputTokens != 57 {
		t.Fatalf("Usage = %+v, want input=111 output=57", rc.Usage)
	}

	summarizer.AssertCallCount(t, 1)
}

func TestSummarize_SendsFlattenedTranscriptAsSingleUserMessage(t *testing.T) {
	msgs := []kit.Message{
		kit.NewUserMessage(kit.NewTextContent("hello")),
		kit.NewModelMessage(kit.NewTextContent("hi there")),
	}

	summarizer := kittest.NewModel(t, kittest.ModelResult{
		Response: kit.ModelResponse{
			Message: kit.NewModelMessage(kit.NewTextContent("summary text")),
		},
	})

	rc := &kit.RunContext{
		Context: context.Background(),
		Memory:  memory.NewHistory(msgs...),
		Events:  &recordAgentEvents{},
	}

	if err := Summarize(summarizer, "instructions")(rc); err != nil {
		t.Fatalf("Summarize: %v", err)
	}

	req := summarizer.CallAt(0)
	if req.Instructions != "instructions" {
		t.Fatalf("Instructions = %q, want %q", req.Instructions, "instructions")
	}

	if len(req.Messages) != 1 {
		t.Fatalf("len(Messages) = %d, want 1 flattened transcript", len(req.Messages))
	}

	if req.Messages[0].Role != kit.RoleUser {
		t.Fatalf("Messages[0].Role = %q, want %q", req.Messages[0].Role, kit.RoleUser)
	}

	text := req.Messages[0].TextContent()
	if text == nil {
		t.Fatal("Messages[0] has no text content")
	}

	for _, msg := range msgs {
		want := msg.Content[0].Text.Text
		if !strings.Contains(text.Text, want) {
			t.Fatalf("flattened transcript missing %q\n--- transcript ---\n%s", want, text.Text)
		}
	}
}

func TestSummarize_NoOpWhenContextIsEmpty(t *testing.T) {
	summarizer := kittest.NewModel(t)

	mem := memory.NewHistory()
	rc := &kit.RunContext{
		Context: context.Background(),
		Memory:  mem,
	}

	if err := Summarize(summarizer, "summarize")(rc); err != nil {
		t.Fatalf("Summarize: %v", err)
	}

	history, err := mem.History(context.Background())
	if err != nil {
		t.Fatalf("History: %v", err)
	}

	if got := len(history); got != 0 {
		t.Fatalf("len(History) = %d, want 0", got)
	}

	summarizer.AssertCallCount(t, 0)
}

func TestSummarize_PropagatesMemoryReadError(t *testing.T) {
	wantErr := errors.New("read failed")

	summarizer := kittest.NewModel(t)

	rc := &kit.RunContext{
		Context: context.Background(),
		Memory:  errMemory{contextErr: wantErr},
	}

	err := Summarize(summarizer, "summarize")(rc)
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want wraps %v", err, wantErr)
	}

	summarizer.AssertCallCount(t, 0)
}

func TestSummarize_PropagatesSummarizerError(t *testing.T) {
	wantErr := errors.New("summarizer down")

	summarizer := kittest.NewModel(t, kittest.ModelResult{Error: wantErr})

	mem := memory.NewHistory(
		kit.NewUserMessage(kit.NewTextContent("a")),
		kit.NewModelMessage(kit.NewTextContent("b")),
	)
	rc := &kit.RunContext{
		Context: context.Background(),
		Memory:  mem,
		Events:  &recordAgentEvents{},
	}

	err := Summarize(summarizer, "summarize")(rc)
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want wraps %v", err, wantErr)
	}

	history, _ := mem.History(context.Background())
	if len(history) != 2 {
		t.Fatalf("len(History) = %d, want 2 (no summary recorded on failure)", len(history))
	}
}

func TestSummarize_ErrorsOnEmptyTextResponse(t *testing.T) {
	summarizer := kittest.NewModel(t, kittest.ModelResult{
		Response: kit.ModelResponse{
			Message: kit.NewModelMessage(kit.NewTextContent("")),
		},
	})

	mem := memory.NewHistory(kit.NewUserMessage(kit.NewTextContent("a")))
	rc := &kit.RunContext{
		Context: context.Background(),
		Memory:  mem,
		Events:  &recordAgentEvents{},
	}

	if err := Summarize(summarizer, "summarize")(rc); err == nil {
		t.Fatal("expected error for empty text response, got nil")
	}

	history, _ := mem.History(context.Background())
	if len(history) != 1 {
		t.Fatalf("len(History) = %d, want 1 (no summary recorded)", len(history))
	}
}

func TestSummarize_ErrorsOnMissingTextContent(t *testing.T) {
	summarizer := kittest.NewModel(t, kittest.ModelResult{
		Response: kit.ModelResponse{Message: kit.NewModelMessage()},
	})

	rc := &kit.RunContext{
		Context: context.Background(),
		Memory:  memory.NewHistory(kit.NewUserMessage(kit.NewTextContent("a"))),
		Events:  &recordAgentEvents{},
	}

	if err := Summarize(summarizer, "summarize")(rc); err == nil {
		t.Fatal("expected error for missing text content, got nil")
	}
}

func TestSummarize_PropagatesMemoryRecordError(t *testing.T) {
	wantErr := errors.New("record failed")

	summarizer := kittest.NewModel(t, kittest.ModelResult{
		Response: kit.ModelResponse{
			Message: kit.NewModelMessage(kit.NewTextContent("summary text")),
		},
	})

	rc := &kit.RunContext{
		Context: context.Background(),
		Memory: errMemory{
			messages:  []kit.Message{kit.NewUserMessage(kit.NewTextContent("a"))},
			recordErr: wantErr,
		},
		Events: &recordAgentEvents{},
	}

	err := Summarize(summarizer, "summarize")(rc)
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want wraps %v", err, wantErr)
	}
}

type recordAgentEvents struct {
	events []kit.AgentEvent
}

func (r *recordAgentEvents) Emit(event kit.AgentEvent) error {
	r.events = append(r.events, event)

	return nil
}

type errMemory struct {
	messages   []kit.Message
	contextErr error
	recordErr  error
}

func (m errMemory) Context(context.Context) ([]kit.Message, error) {
	if m.contextErr != nil {
		return nil, m.contextErr
	}

	return m.messages, nil
}

func (m errMemory) History(context.Context) ([]kit.Message, error) {
	return m.messages, nil
}

func (m errMemory) Record(context.Context, ...kit.Message) error {
	return m.recordErr
}

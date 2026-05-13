package summarization

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/vitaliiPsl/crappy-adk/agent"
	"github.com/vitaliiPsl/crappy-adk/kit"
	"github.com/vitaliiPsl/crappy-adk/kittest"
	"github.com/vitaliiPsl/crappy-adk/x/memory"
)

type recordAgentEvents struct {
	events []kit.AgentEvent
}

func (r *recordAgentEvents) Emit(event kit.AgentEvent) error {
	r.events = append(r.events, event)

	return nil
}

func TestSummarizationHook_RunsStrategyWhenTriggerFires(t *testing.T) {
	called := false
	hook := onTurnStart(
		func(*kit.RunContext) bool { return true },
		func(*kit.RunContext) error {
			called = true

			return nil
		},
	)

	if err := hook(&kit.RunContext{}); err != nil {
		t.Fatalf("hook: %v", err)
	}

	if !called {
		t.Fatal("strategy was not called when trigger returned true")
	}
}

func TestSummarizationHook_SkipsStrategyWhenTriggerDoesNotFire(t *testing.T) {
	hook := onTurnStart(
		func(*kit.RunContext) bool { return false },
		func(*kit.RunContext) error {
			t.Fatal("strategy ran when trigger returned false")

			return nil
		},
	)

	if err := hook(&kit.RunContext{}); err != nil {
		t.Fatalf("hook: %v", err)
	}
}

func TestWithSummarization_ConfiguresAgentSummarization(t *testing.T) {
	model := kittest.NewModel(t, kittest.ModelResult{
		Response: kit.ModelResponse{
			Message:      kit.NewModelMessage(kit.NewTextContent("done")),
			FinishReason: kit.FinishReasonStop,
		},
	})

	called := false
	mem := memory.NewHistory()

	a, err := agent.New(
		model,
		mem,
		WithSummarization(
			func(*kit.RunContext) bool { return true },
			func(rc *kit.RunContext) error {
				called = true

				return rc.Memory.Record(rc.Context, kit.NewUserMessage(kit.NewSummaryContent("summarized")))
			},
		),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, err = a.Run(context.Background(), kit.NewUserMessage(kit.NewTextContent("hello")))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if !called {
		t.Fatal("strategy was not called")
	}

	req := model.CallAt(0)
	if got := len(req.Messages); got != 1 {
		t.Fatalf("model received %d messages, want summary context", got)
	}

	if req.Messages[0].Content[0].Type != kit.ContentTypeSummary {
		t.Fatalf("message content type = %q, want summary", req.Messages[0].Content[0].Type)
	}
}

func TestWhenUsageTokensExceed_BelowThreshold(t *testing.T) {
	trigger := WhenUsageTokensExceed(100)

	if trigger(&kit.RunContext{LastUsage: kit.Usage{InputTokens: 50}}) {
		t.Fatal("trigger fired below threshold")
	}
}

func TestWhenUsageTokensExceed_AtThreshold(t *testing.T) {
	trigger := WhenUsageTokensExceed(100)

	if trigger(&kit.RunContext{LastUsage: kit.Usage{InputTokens: 100}}) {
		t.Fatal("trigger fired at exactly threshold")
	}
}

func TestWhenUsageTokensExceed_AboveThreshold(t *testing.T) {
	trigger := WhenUsageTokensExceed(100)

	if !trigger(&kit.RunContext{LastUsage: kit.Usage{InputTokens: 101}}) {
		t.Fatal("trigger did not fire above threshold")
	}
}

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

	req := summarizer.CallAt(0)
	if req.Instructions != "summarize this" {
		t.Fatalf("summarizer instructions = %q, want %q", req.Instructions, "summarize this")
	}

	if len(req.Messages) != len(msgs) {
		t.Fatalf("summarizer received %d messages, want %d", len(req.Messages), len(msgs))
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

func TestSummarize_PropagatesSummarizerError(t *testing.T) {
	wantErr := errors.New("summarizer down")

	summarizer := kittest.NewModel(t, kittest.ModelResult{Error: wantErr})

	mem := memory.NewHistory(
		kit.NewUserMessage(kit.NewTextContent("a")),
		kit.NewModelMessage(kit.NewTextContent("b")),
		kit.NewUserMessage(kit.NewTextContent("c")),
		kit.NewModelMessage(kit.NewTextContent("d")),
	)
	rc := &kit.RunContext{
		Context: context.Background(),
		Memory:  mem,
	}

	err := Summarize(summarizer, "summarize")(rc)
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want wraps %v", err, wantErr)
	}
}

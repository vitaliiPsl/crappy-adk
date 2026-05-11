package compaction

import (
	"context"
	"errors"
	"testing"

	"github.com/vitaliiPsl/crappy-adk/agent"
	"github.com/vitaliiPsl/crappy-adk/kit"
	"github.com/vitaliiPsl/crappy-adk/kittest"
	"github.com/vitaliiPsl/crappy-adk/x/memory"
)

func TestCompactionHook_RunsStrategyWhenTriggerFires(t *testing.T) {
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

func TestCompactionHook_SkipsStrategyWhenTriggerDoesNotFire(t *testing.T) {
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

func TestWithCompaction_ConfiguresAgentCompaction(t *testing.T) {
	model := kittest.NewModel(t, kittest.ModelResult{
		Response: kit.ModelResponse{
			Message:      kit.NewModelMessage([]kit.Content{kit.NewTextContent("done")}),
			FinishReason: kit.FinishReasonStop,
		},
	})

	called := false
	mem := memory.NewHistory()

	a, err := agent.New(
		model,
		mem,
		WithCompaction(
			func(*kit.RunContext) bool { return true },
			func(rc *kit.RunContext) error {
				called = true

				return rc.Memory.Record(rc.Context, kit.NewSummaryMessage("compacted"))
			},
		),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, err = a.Run(context.Background(), kit.NewUserMessage([]kit.Content{kit.NewTextContent("hello")}))
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
		kit.NewUserMessage([]kit.Content{kit.NewTextContent("turn 1")}),
		kit.NewModelMessage([]kit.Content{kit.NewTextContent("reply 1")}),
		kit.NewUserMessage([]kit.Content{kit.NewTextContent("turn 2")}),
		kit.NewModelMessage([]kit.Content{kit.NewTextContent("reply 2")}),
		kit.NewUserMessage([]kit.Content{kit.NewTextContent("turn 3")}),
		kit.NewModelMessage([]kit.Content{kit.NewTextContent("reply 3")}),
	}

	summarizer := kittest.NewModel(t, kittest.ModelResult{
		Response: kit.ModelResponse{
			Message:      kit.NewModelMessage([]kit.Content{kit.NewTextContent("summary text")}),
			FinishReason: kit.FinishReasonStop,
			Usage:        kit.Usage{InputTokens: 11, OutputTokens: 7},
		},
	})

	mem := memory.NewHistory(msgs...)
	rc := &kit.RunContext{
		Context: context.Background(),
		Memory:  mem,
		Usage:   kit.Usage{InputTokens: 100, OutputTokens: 50},
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
		kit.NewUserMessage([]kit.Content{kit.NewTextContent("a")}),
		kit.NewModelMessage([]kit.Content{kit.NewTextContent("b")}),
		kit.NewUserMessage([]kit.Content{kit.NewTextContent("c")}),
		kit.NewModelMessage([]kit.Content{kit.NewTextContent("d")}),
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

package compaction

import (
	"context"
	"errors"
	"testing"

	"github.com/vitaliiPsl/crappy-adk/agent"
	"github.com/vitaliiPsl/crappy-adk/kit"
	"github.com/vitaliiPsl/crappy-adk/kittest"
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

	a, err := agent.New(
		model,
		WithCompaction(
			func(*kit.RunContext) bool { return true },
			func(rc *kit.RunContext) error {
				called = true

				rc.Messages = append(rc.Messages, kit.NewSummaryMessage("compacted"))

				return nil
			},
		),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, err = a.Run(context.Background(), []kit.Message{
		kit.NewUserMessage([]kit.Content{kit.NewTextContent("hello")}),
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if !called {
		t.Fatal("strategy was not called")
	}

	req := model.CallAt(0)
	if got := len(req.Messages); got != 2 {
		t.Fatalf("model received %d messages, want original + summary", got)
	}

	if req.Messages[1].Content[0].Type != kit.ContentTypeSummary {
		t.Fatalf("second message content type = %q, want summary", req.Messages[1].Content[0].Type)
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

func TestSafeCutoff_KeepMoreThanTotal(t *testing.T) {
	msgs := []kit.Message{kit.NewUserMessage(nil), kit.NewModelMessage(nil)}

	if got := safeCutoff(msgs, 5); got != 0 {
		t.Fatalf("safeCutoff = %d, want 0", got)
	}
}

func TestSafeCutoff_CleanBoundary(t *testing.T) {
	msgs := []kit.Message{
		kit.NewUserMessage(nil),
		kit.NewModelMessage(nil),
		kit.NewUserMessage(nil),
		kit.NewModelMessage(nil),
	}

	if got := safeCutoff(msgs, 2); got != 2 {
		t.Fatalf("safeCutoff = %d, want 2", got)
	}
}

func TestSafeCutoff_BoundaryOnToolMessagePushesForward(t *testing.T) {
	msgs := []kit.Message{
		kit.NewUserMessage(nil),
		kit.NewModelMessage(nil),
		kit.NewToolMessage(nil),
		kit.NewModelMessage(nil),
	}

	if got := safeCutoff(msgs, 2); got != 3 {
		t.Fatalf("safeCutoff = %d, want 3", got)
	}
}

func TestSafeCutoff_ConsecutiveToolMessagesPushedPast(t *testing.T) {
	msgs := []kit.Message{
		kit.NewUserMessage(nil),
		kit.NewModelMessage(nil),
		kit.NewToolMessage(nil),
		kit.NewToolMessage(nil),
		kit.NewModelMessage(nil),
	}

	if got := safeCutoff(msgs, 3); got != 4 {
		t.Fatalf("safeCutoff = %d, want 4", got)
	}
}

func TestSummarizeWithRecent_ReplacesPrefixAndAppendsToGenerated(t *testing.T) {
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

	rc := &kit.RunContext{
		Context:  context.Background(),
		Messages: append([]kit.Message{}, msgs...),
		Usage:    kit.Usage{InputTokens: 100, OutputTokens: 50},
	}

	if err := SummarizeWithRecent(summarizer, "summarize this", 2)(rc); err != nil {
		t.Fatalf("SummarizeWithRecent: %v", err)
	}

	if got := len(rc.Messages); got != 3 {
		t.Fatalf("len(Messages) = %d, want 3 (summary + 2 kept)", got)
	}

	summaryMsg := rc.Messages[0]

	if summaryMsg.Role != kit.RoleUser {
		t.Fatalf("summary message role = %q, want %q", summaryMsg.Role, kit.RoleUser)
	}

	if summaryMsg.Content[0].Type != kit.ContentTypeSummary {
		t.Fatalf("summary content type = %q, want %q", summaryMsg.Content[0].Type, kit.ContentTypeSummary)
	}

	if summaryMsg.Content[0].Summary.Text != "summary text" {
		t.Fatalf("summary text = %q, want %q", summaryMsg.Content[0].Summary.Text, "summary text")
	}

	if rc.Messages[1].Content[0].Text.Text != "turn 3" {
		t.Fatalf("kept[0] text = %q, want %q", rc.Messages[1].Content[0].Text.Text, "turn 3")
	}

	if len(rc.Generated) != 1 {
		t.Fatalf("len(Generated) = %d, want 1", len(rc.Generated))
	}

	if rc.Generated[0].Content[0].Type != kit.ContentTypeSummary {
		t.Fatalf("Generated[0] is not a summary message")
	}

	if rc.Usage.InputTokens != 111 || rc.Usage.OutputTokens != 57 {
		t.Fatalf("Usage = %+v, want input=111 output=57", rc.Usage)
	}

	summarizer.AssertCallCount(t, 1)

	req := summarizer.CallAt(0)
	if req.Instructions != "summarize this" {
		t.Fatalf("summarizer instructions = %q, want %q", req.Instructions, "summarize this")
	}

	if len(req.Messages) != 4 {
		t.Fatalf("summarizer received %d messages, want 4", len(req.Messages))
	}
}

func TestSummarizeWithRecent_NoOpWhenNothingToSummarize(t *testing.T) {
	summarizer := kittest.NewModel(t)

	rc := &kit.RunContext{
		Context: context.Background(),
		Messages: []kit.Message{
			kit.NewUserMessage([]kit.Content{kit.NewTextContent("hi")}),
			kit.NewModelMessage([]kit.Content{kit.NewTextContent("hello")}),
		},
	}

	if err := SummarizeWithRecent(summarizer, "summarize", 5)(rc); err != nil {
		t.Fatalf("SummarizeWithRecent: %v", err)
	}

	if len(rc.Messages) != 2 {
		t.Fatalf("len(Messages) = %d, want 2", len(rc.Messages))
	}

	if len(rc.Generated) != 0 {
		t.Fatalf("len(Generated) = %d, want 0", len(rc.Generated))
	}

	summarizer.AssertCallCount(t, 0)
}

func TestSummarizeWithRecent_PropagatesSummarizerError(t *testing.T) {
	wantErr := errors.New("summarizer down")

	summarizer := kittest.NewModel(t, kittest.ModelResult{Error: wantErr})

	rc := &kit.RunContext{
		Context: context.Background(),
		Messages: []kit.Message{
			kit.NewUserMessage([]kit.Content{kit.NewTextContent("a")}),
			kit.NewModelMessage([]kit.Content{kit.NewTextContent("b")}),
			kit.NewUserMessage([]kit.Content{kit.NewTextContent("c")}),
			kit.NewModelMessage([]kit.Content{kit.NewTextContent("d")}),
		},
	}

	err := SummarizeWithRecent(summarizer, "summarize", 1)(rc)
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want wraps %v", err, wantErr)
	}
}

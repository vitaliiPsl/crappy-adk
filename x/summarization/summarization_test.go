package summarization

import (
	"context"
	"testing"

	"github.com/vitaliiPsl/crappy-adk/agent"
	"github.com/vitaliiPsl/crappy-adk/kit"
	"github.com/vitaliiPsl/crappy-adk/kittest"
	"github.com/vitaliiPsl/crappy-adk/x/memory"
)

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

func TestSummarizationHook_PropagatesStrategyError(t *testing.T) {
	wantErr := errString("strategy boom")

	hook := onTurnStart(
		func(*kit.RunContext) bool { return true },
		func(*kit.RunContext) error { return wantErr },
	)

	if err := hook(&kit.RunContext{}); err != wantErr {
		t.Fatalf("hook err = %v, want %v", err, wantErr)
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

package limits_test

import (
	"context"
	"errors"
	"testing"

	"github.com/vitaliiPsl/crappy-adk/agent"
	"github.com/vitaliiPsl/crappy-adk/kit"
	"github.com/vitaliiPsl/crappy-adk/kittest"
	"github.com/vitaliiPsl/crappy-adk/x/limits"
)

func TestWithMaxUsage_AllowsFinalResponse(t *testing.T) {
	model := kittest.NewModel(t,
		kittest.ModelTurn{
			Text:  "too much",
			Usage: kit.Usage{InputTokens: 10, OutputTokens: 20},
		},
	)

	a, err := agent.New(model,
		limits.WithMaxUsage(limits.UsageLimits{OutputTokens: 10}),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, err = a.Run(context.Background(), []kit.Message{
		kit.NewUserMessage(kit.NewTextPart("Hi")),
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
}

func TestWithMaxUsage_StopsNextTurn(t *testing.T) {
	tool := kittest.NewTool(t, "noop", "Noop",
		kittest.ToolResponse{Result: "ok"},
	)

	model := kittest.NewModel(t,
		kittest.ModelTurn{
			ToolCalls: []kit.ToolCall{{ID: "call_1", Name: "noop"}},
			Usage:     kit.Usage{InputTokens: 10, OutputTokens: 20},
		},
	)

	a, err := agent.New(model,
		agent.WithTools(tool),
		limits.WithMaxUsage(limits.UsageLimits{OutputTokens: 10}),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, err = a.Run(context.Background(), []kit.Message{
		kit.NewUserMessage(kit.NewTextPart("Hi")),
	})
	if err == nil {
		t.Fatal("expected limit error, got nil")
	}

	if !errors.Is(err, kit.ErrLimitExceeded) {
		t.Fatalf("error = %v, want kit.ErrLimitExceeded", err)
	}

	tool.AssertCallCount(t, 1)
}

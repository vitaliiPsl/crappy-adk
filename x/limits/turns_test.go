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

func TestWithMaxTurns(t *testing.T) {
	tool := kittest.NewTool(t, "noop", "Noop",
		kittest.ToolResponse{Result: "ok"},
	)

	model := kittest.NewModel(t,
		kittest.ModelTurn{ToolCalls: []kit.ToolCall{{ID: "call_1", Name: "noop"}}},
		kittest.ModelTurn{Text: "would require second turn"},
	)

	a, err := agent.New(model,
		agent.WithTools(tool),
		limits.WithMaxTurns(1),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, err = a.Run(context.Background(), []kit.Message{
		kit.NewUserMessage(kit.NewTextPart("Do it")),
	})
	if err == nil {
		t.Fatal("expected limit error, got nil")
	}

	if !errors.Is(err, kit.ErrLimitExceeded) {
		t.Fatalf("error = %v, want kit.ErrLimitExceeded", err)
	}

	model.AssertCallCount(t, 1)
	tool.AssertCallCount(t, 1)
}

func TestWithMaxTurns_OptionReuseCreatesIndependentGuards(t *testing.T) {
	opt := limits.WithMaxTurns(1)

	firstModel := kittest.NewModel(t, kittest.ModelTurn{Text: "first"})

	first, err := agent.New(firstModel, opt)
	if err != nil {
		t.Fatalf("New first: %v", err)
	}

	secondModel := kittest.NewModel(t, kittest.ModelTurn{Text: "second"})

	second, err := agent.New(secondModel, opt)
	if err != nil {
		t.Fatalf("New second: %v", err)
	}

	if _, err := first.Run(context.Background(), []kit.Message{
		kit.NewUserMessage(kit.NewTextPart("Hi")),
	}); err != nil {
		t.Fatalf("Run first: %v", err)
	}

	if _, err := second.Run(context.Background(), []kit.Message{
		kit.NewUserMessage(kit.NewTextPart("Hi")),
	}); err != nil {
		t.Fatalf("Run second: %v", err)
	}
}

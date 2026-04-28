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

func TestWithMaxToolCalls(t *testing.T) {
	tool := kittest.NewTool(t, "search", "Search",
		kittest.ToolResponse{Result: "A"},
		kittest.ToolResponse{Result: "B"},
	)

	model := kittest.NewModel(t,
		kittest.ModelTurn{ToolCalls: []kit.ToolCall{
			{ID: "call_1", Name: "search", Arguments: map[string]any{"q": "A"}},
			{ID: "call_2", Name: "search", Arguments: map[string]any{"q": "B"}},
		}},
	)

	a, err := agent.New(model,
		agent.WithTools(tool),
		limits.WithMaxToolCalls(1),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, err = a.Run(context.Background(), []kit.Message{
		kit.NewUserMessage(kit.NewTextPart("Search")),
	})
	if err == nil {
		t.Fatal("expected limit error, got nil")
	}

	if !errors.Is(err, kit.ErrLimitExceeded) {
		t.Fatalf("error = %v, want kit.ErrLimitExceeded", err)
	}

	tool.AssertCallCount(t, 2)
}

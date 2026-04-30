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

func TestWithToolLoopDetection(t *testing.T) {
	searchTool := kittest.NewTool(t, "search", "Search",
		kittest.ToolResponse{Result: "same result"},
		kittest.ToolResponse{Result: "same result"},
		kittest.ToolResponse{Result: "same result"},
	)

	sameCall := kit.ToolCall{ID: "c", Name: "search", Arguments: map[string]any{"q": "crappy"}}

	model := kittest.NewModel(t,
		kittest.ModelTurn{ToolCalls: []kit.ToolCall{sameCall}},
		kittest.ModelTurn{ToolCalls: []kit.ToolCall{sameCall}},
		kittest.ModelTurn{ToolCalls: []kit.ToolCall{sameCall}},
	)

	a, err := agent.New(model,
		agent.WithTools(searchTool),
		limits.WithToolLoopDetection(2, 15),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, err = a.Run(context.Background(), []kit.Message{
		kit.NewUserMessage(kit.NewTextPart("Search for crappy")),
	})
	if err == nil {
		t.Fatal("expected loop error, got nil")
	}

	if !errors.Is(err, kit.ErrToolLoop) {
		t.Fatalf("error = %v, want kit.ErrToolLoop", err)
	}

	searchTool.AssertCallCount(t, 2)
}

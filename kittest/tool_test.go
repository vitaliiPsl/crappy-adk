package kittest

import (
	"context"
	"errors"
	"testing"
)

func TestTool_Definition(t *testing.T) {
	tool := NewTool(t, "search", "search things")

	got := tool.Definition()
	if got.Name != "search" {
		t.Fatalf("Name = %q, want %q", got.Name, "search")
	}

	if got.Description != "search things" {
		t.Fatalf("Description = %q, want %q", got.Description, "search things")
	}
}

func TestTool_Execute(t *testing.T) {
	wantErr := errors.New("boom")
	tool := NewTool(t, "search", "search things", ToolResult{Result: "done", Error: wantErr})

	output, err := tool.Execute(context.Background(), map[string]any{"query": "hello"})
	if output != "done" {
		t.Fatalf("output = %q, want %q", output, "done")
	}

	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}

	tool.AssertCalledWith(t, 0, map[string]any{"query": "hello"})
}

func TestTool_Execute_ConsumesResponsesInOrder(t *testing.T) {
	tool := NewTool(t,
		"search",
		"search things",
		ToolResult{Result: "first"},
		ToolResult{Result: "second"},
	)

	first, err := tool.Execute(context.Background(), map[string]any{"value": 1})
	if err != nil || first != "first" {
		t.Fatalf("first Execute = %q, %v; want first, nil", first, err)
	}

	second, err := tool.Execute(context.Background(), map[string]any{"value": 2})
	if err != nil || second != "second" {
		t.Fatalf("second Execute = %q, %v; want second, nil", second, err)
	}

	tool.AssertCallCount(t, 2)

	if got := tool.CallAt(1)["value"]; got != 2 {
		t.Fatalf("CallAt(1)[value] = %v, want 2", got)
	}
}

func TestTool_AssertNeverCalled(t *testing.T) {
	tool := NewTool(t, "unused", "unused tool")

	tool.AssertNeverCalled(t)
}

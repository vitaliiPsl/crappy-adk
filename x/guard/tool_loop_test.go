package guard

import (
	"context"
	"errors"
	"testing"

	"github.com/vitaliiPsl/crappy-adk/agent"
	"github.com/vitaliiPsl/crappy-adk/kit"
	"github.com/vitaliiPsl/crappy-adk/kittest"
	"github.com/vitaliiPsl/crappy-adk/x/memory"
)

func TestWithRepeatedToolCallLimit_DetectsRepeatedCallIgnoringCallID(t *testing.T) {
	firstCall := kit.NewToolCall("call-1", "add", map[string]any{"a": 3, "b": 4})
	repeatedCall := kit.NewToolCall("call-2", "add", map[string]any{"a": 3, "b": 4})
	otherCall := kit.NewToolCall("call-3", "add", map[string]any{"a": 1, "b": 2})

	tool := kittest.NewTool(t, "add", "add numbers", kittest.ToolResult{Result: "7"})
	model := kittest.NewModel(t,
		kittest.ModelResult{
			Response: kit.ModelResponse{
				Message:      kit.NewModelMessage(kit.NewToolCallContent(firstCall)),
				FinishReason: kit.FinishReasonToolCall,
			},
		},
		kittest.ModelResult{
			Response: kit.ModelResponse{
				Message: kit.NewModelMessage(
					kit.NewToolCallContent(repeatedCall),
					kit.NewToolCallContent(otherCall),
				),
				FinishReason: kit.FinishReasonToolCall,
			},
		},
	)

	a, err := agent.New(model, memory.NewHistory(), agent.WithTools(tool), WithRepeatedToolCallLimit(1, 2))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, err = a.Run(context.Background(), kit.NewUserMessage(kit.NewTextContent("add 3 and 4")))
	if !errors.Is(err, ErrToolLoopDetected) {
		t.Fatalf("Run error = %v, want ErrToolLoopDetected", err)
	}

	tool.AssertCallCount(t, 1)
	model.AssertCallCount(t, 2)
}

func TestWithRepeatedToolCallLimit_CountsDuplicatesInCurrentResponse(t *testing.T) {
	firstCall := kit.NewToolCall("call-1", "add", map[string]any{"a": 3, "b": 4})
	repeatedCall := kit.NewToolCall("call-2", "add", map[string]any{"a": 3, "b": 4})

	tool := kittest.NewTool(t, "add", "add numbers")
	model := kittest.NewModel(t, kittest.ModelResult{
		Response: kit.ModelResponse{
			Message: kit.NewModelMessage(
				kit.NewToolCallContent(firstCall),
				kit.NewToolCallContent(repeatedCall),
			),
			FinishReason: kit.FinishReasonToolCall,
		},
	})

	a, err := agent.New(model, memory.NewHistory(), agent.WithTools(tool), WithRepeatedToolCallLimit(1, 1))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, err = a.Run(context.Background(), kit.NewUserMessage(kit.NewTextContent("add 3 and 4 twice")))
	if !errors.Is(err, ErrToolLoopDetected) {
		t.Fatalf("Run error = %v, want ErrToolLoopDetected", err)
	}

	tool.AssertNeverCalled(t)
	model.AssertCallCount(t, 1)
}

func TestWithRepeatedToolCallLimit_OnlyCountsWithinWindow(t *testing.T) {
	firstCall := kit.NewToolCall("call-1", "add", map[string]any{"a": 1, "b": 2})
	differentCall := kit.NewToolCall("call-2", "add", map[string]any{"a": 3, "b": 4})
	repeatedLaterCall := kit.NewToolCall("call-3", "add", map[string]any{"a": 1, "b": 2})

	tool := kittest.NewTool(t,
		"add",
		"add numbers",
		kittest.ToolResult{Result: "3"},
		kittest.ToolResult{Result: "7"},
		kittest.ToolResult{Result: "3"},
	)
	model := kittest.NewModel(t,
		kittest.ModelResult{
			Response: kit.ModelResponse{
				Message:      kit.NewModelMessage(kit.NewToolCallContent(firstCall)),
				FinishReason: kit.FinishReasonToolCall,
			},
		},
		kittest.ModelResult{
			Response: kit.ModelResponse{
				Message:      kit.NewModelMessage(kit.NewToolCallContent(differentCall)),
				FinishReason: kit.FinishReasonToolCall,
			},
		},
		kittest.ModelResult{
			Response: kit.ModelResponse{
				Message:      kit.NewModelMessage(kit.NewToolCallContent(repeatedLaterCall)),
				FinishReason: kit.FinishReasonToolCall,
			},
		},
		kittest.ModelResult{
			Response: kit.ModelResponse{
				Message:      kit.NewModelMessage(kit.NewTextContent("done")),
				FinishReason: kit.FinishReasonStop,
			},
		},
	)

	a, err := agent.New(model, memory.NewHistory(), agent.WithTools(tool), WithRepeatedToolCallLimit(1, 2))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	resp, err := a.Run(context.Background(), kit.NewUserMessage(kit.NewTextContent("add numbers")))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if resp.Output.Text != "done" {
		t.Fatalf("Output = %q, want done", resp.Output.Text)
	}

	tool.AssertCallCount(t, 3)
	model.AssertCallCount(t, 4)
}

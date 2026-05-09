package guard

import (
	"context"
	"errors"
	"testing"

	"github.com/vitaliiPsl/crappy-adk/agent"
	"github.com/vitaliiPsl/crappy-adk/kit"
	"github.com/vitaliiPsl/crappy-adk/kittest"
)

func TestWithMaxTurns_StopsBeforeNextModelCall(t *testing.T) {
	call := kit.NewToolCall("call-1", "add", map[string]any{"a": 3, "b": 4})
	toolCallMessage := kit.NewModelMessage([]kit.Content{kit.NewToolCallContent(call)})

	tool := kittest.NewTool(t, "add", "add numbers", kittest.ToolResult{Result: "7"})
	model := kittest.NewModel(t,
		kittest.ModelResult{
			Response: kit.ModelResponse{
				Message:      toolCallMessage,
				FinishReason: kit.FinishReasonToolCall,
			},
		},
		kittest.ModelResult{
			Response: kit.ModelResponse{
				Message:      kit.NewModelMessage([]kit.Content{kit.NewTextContent("should not be called")}),
				FinishReason: kit.FinishReasonStop,
			},
		},
	)

	a, err := agent.New(model, agent.WithTools(tool), WithMaxTurns(1))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	resp, err := a.Run(context.Background(), []kit.Message{
		kit.NewUserMessage([]kit.Content{kit.NewTextContent("add 3 and 4")}),
	})
	if !errors.Is(err, ErrMaxTurnsExceeded) {
		t.Fatalf("Run error = %v, want ErrMaxTurnsExceeded", err)
	}

	if len(resp.Messages) != 2 {
		t.Fatalf("len(Messages) = %d, want tool call message and tool result message", len(resp.Messages))
	}

	model.AssertCallCount(t, 1)
	tool.AssertCallCount(t, 1)
}

func TestWithMaxToolCalls_StopsBeforeToolExecution(t *testing.T) {
	calls := []kit.Content{
		kit.NewToolCallContent(kit.NewToolCall("call-1", "add", map[string]any{"a": 1, "b": 2})),
		kit.NewToolCallContent(kit.NewToolCall("call-2", "add", map[string]any{"a": 3, "b": 4})),
	}

	tool := kittest.NewTool(t, "add", "add numbers")
	model := kittest.NewModel(t, kittest.ModelResult{
		Response: kit.ModelResponse{
			Message:      kit.NewModelMessage(calls),
			FinishReason: kit.FinishReasonToolCall,
		},
	})

	a, err := agent.New(model, agent.WithTools(tool), WithMaxToolCalls(1))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	resp, err := a.Run(context.Background(), []kit.Message{
		kit.NewUserMessage([]kit.Content{kit.NewTextContent("add these")}),
	})
	if !errors.Is(err, ErrMaxToolCallsExceeded) {
		t.Fatalf("Run error = %v, want ErrMaxToolCallsExceeded", err)
	}

	if len(resp.Messages) != 0 {
		t.Fatalf("len(Messages) = %d, want 0 because guarded response is rejected before append", len(resp.Messages))
	}

	tool.AssertNeverCalled(t)
	model.AssertCallCount(t, 1)
}

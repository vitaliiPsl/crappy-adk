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

func TestWithBudget_AllowsUsageAtLimit(t *testing.T) {
	model := kittest.NewModel(t, kittest.ModelResult{
		Response: kit.ModelResponse{
			Message:      kit.NewModelMessage(kit.NewTextContent("done")),
			FinishReason: kit.FinishReasonStop,
			Usage:        kit.Usage{InputTokens: 7, OutputTokens: 3},
		},
	})

	a, err := agent.New(model, memory.NewHistory(),
		WithInputTokenBudget(7),
		WithOutputTokenBudget(3),
		WithTotalTokenBudget(10),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	resp, err := a.Run(context.Background(), kit.NewUserMessage(kit.NewTextContent("hello")))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if resp.Output.Text != "done" {
		t.Fatalf("Output = %q, want done", resp.Output.Text)
	}

	if resp.Usage.InputTokens != 7 || resp.Usage.OutputTokens != 3 {
		t.Fatalf("Usage = %+v, want input=7 output=3", resp.Usage)
	}
}

func TestWithBudget_RejectsResponseThatExceedsCumulativeBudget(t *testing.T) {
	call := kit.NewToolCall("call-1", "add", map[string]any{"a": 3, "b": 4})
	toolCallMessage := kit.NewModelMessage(kit.NewToolCallContent(call))
	finalMessage := kit.NewModelMessage(kit.NewTextContent("should not be appended"))

	tool := kittest.NewTool(t, "add", "add numbers", kittest.ToolResult{Result: "7"})
	model := kittest.NewModel(t,
		kittest.ModelResult{
			Response: kit.ModelResponse{
				Message:      toolCallMessage,
				FinishReason: kit.FinishReasonToolCall,
				Usage:        kit.Usage{InputTokens: 6, OutputTokens: 2},
			},
		},
		kittest.ModelResult{
			Response: kit.ModelResponse{
				Message:      finalMessage,
				FinishReason: kit.FinishReasonStop,
				Usage:        kit.Usage{InputTokens: 5, OutputTokens: 1},
			},
		},
	)

	a, err := agent.New(model, memory.NewHistory(), agent.WithTools(tool), WithTotalTokenBudget(13))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	resp, err := a.Run(context.Background(), kit.NewUserMessage(kit.NewTextContent("add 3 and 4")))
	if !errors.Is(err, ErrBudgetExceeded) {
		t.Fatalf("Run error = %v, want ErrBudgetExceeded", err)
	}

	if len(resp.Messages) != 2 {
		t.Fatalf("len(Messages) = %d, want first model message and tool message only", len(resp.Messages))
	}

	if resp.Output != nil {
		t.Fatalf("Output = %+v, want nil because final response was rejected", resp.Output)
	}

	if resp.Usage.InputTokens != 6 || resp.Usage.OutputTokens != 2 {
		t.Fatalf("Usage = %+v, want only accepted first response usage", resp.Usage)
	}

	tool.AssertCallCount(t, 1)
	model.AssertCallCount(t, 2)
}

func TestWithInputTokenBudget_RejectsInputBudget(t *testing.T) {
	model := kittest.NewModel(t, kittest.ModelResult{
		Response: kit.ModelResponse{
			Message:      kit.NewModelMessage(kit.NewTextContent("done")),
			FinishReason: kit.FinishReasonStop,
			Usage:        kit.Usage{InputTokens: 4},
		},
	})

	a, err := agent.New(model, memory.NewHistory(), WithInputTokenBudget(3))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	resp, err := a.Run(context.Background(), kit.NewUserMessage(kit.NewTextContent("think")))
	if !errors.Is(err, ErrBudgetExceeded) {
		t.Fatalf("Run error = %v, want ErrBudgetExceeded", err)
	}

	if len(resp.Messages) != 0 {
		t.Fatalf("len(Messages) = %d, want 0 because guarded response is rejected before append", len(resp.Messages))
	}
}

func TestWithOutputTokenBudget_RejectsOutputBudget(t *testing.T) {
	model := kittest.NewModel(t, kittest.ModelResult{
		Response: kit.ModelResponse{
			Message:      kit.NewModelMessage(kit.NewTextContent("done")),
			FinishReason: kit.FinishReasonStop,
			Usage:        kit.Usage{OutputTokens: 4},
		},
	})

	a, err := agent.New(model, memory.NewHistory(), WithOutputTokenBudget(3))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	resp, err := a.Run(context.Background(), kit.NewUserMessage(kit.NewTextContent("write")))
	if !errors.Is(err, ErrBudgetExceeded) {
		t.Fatalf("Run error = %v, want ErrBudgetExceeded", err)
	}

	if len(resp.Messages) != 0 {
		t.Fatalf("len(Messages) = %d, want 0 because guarded response is rejected before append", len(resp.Messages))
	}
}

package agent

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/vitaliiPsl/crappy-adk/kit"
	"github.com/vitaliiPsl/crappy-adk/kittest"
)

func TestRun_ReturnsFinalResponse(t *testing.T) {
	tool := kittest.NewTool(t, "unused", "unused tool")

	model := kittest.NewModel(t, kittest.ModelResult{
		Response: kit.ModelResponse{
			Message:      kit.NewModelMessage([]kit.Content{kit.NewTextContent("done")}),
			FinishReason: kit.FinishReasonStop,
			Usage:        kit.Usage{InputTokens: 3, OutputTokens: 2},
		},
	})

	messages := []kit.Message{
		kit.NewUserMessage([]kit.Content{kit.NewTextContent("hello")}),
	}

	a, err := New(model, []kit.Tool{tool}, kit.AgentConfig{Instructions: "be brief"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	response, err := a.Run(context.Background(), messages)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if response.Output.Text != "done" {
		t.Fatalf("Output = %q, want done", response.Output.Text)
	}

	if response.Usage.InputTokens != 3 || response.Usage.OutputTokens != 2 {
		t.Fatalf("Usage = %+v, want input 3 output 2", response.Usage)
	}

	if len(response.Messages) != 1 {
		t.Fatalf("len(Messages) = %d, want 1", len(response.Messages))
	}

	if response.Messages[0].TextContent().Text != "done" {
		t.Fatalf("Messages[0] text = %q, want done", response.Messages[0].TextContent().Text)
	}

	model.AssertCalledWith(t, 0, kit.ModelRequest{
		Instructions: "be brief",
		Messages:     messages,
		Tools:        []kit.Tool{tool},
	})

	tool.AssertNeverCalled(t)
}

func TestRun_ReturnsModelError(t *testing.T) {
	wantErr := errors.New("model failed")

	model := kittest.NewModel(t, kittest.ModelResult{Error: wantErr})

	a, err := New(model, nil, kit.AgentConfig{Instructions: "be brief"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, err = a.Run(context.Background(), []kit.Message{
		kit.NewUserMessage([]kit.Content{kit.NewTextContent("hello")}),
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Run error = %v, want %v", err, wantErr)
	}

	model.AssertCallCount(t, 1)
}

func TestRun_ExecutesToolCallAndContinues(t *testing.T) {
	call := kit.NewToolCall("call-1", "add", map[string]any{"a": 3, "b": 4})
	toolCallMessage := kit.NewModelMessage([]kit.Content{kit.NewToolCallContent(call)})
	finalMessage := kit.NewModelMessage([]kit.Content{kit.NewTextContent("3 + 4 = 7")})

	tool := kittest.NewTool(t, "add", "add numbers", kittest.ToolResult{Result: "7"})

	model := kittest.NewModel(t,
		kittest.ModelResult{
			Response: kit.ModelResponse{
				Message:      toolCallMessage,
				FinishReason: kit.FinishReasonToolCall,
				Usage:        kit.Usage{InputTokens: 5, OutputTokens: 2},
			},
		},
		kittest.ModelResult{
			Response: kit.ModelResponse{
				Message:      finalMessage,
				FinishReason: kit.FinishReasonStop,
				Usage:        kit.Usage{InputTokens: 7, OutputTokens: 3},
			},
		},
	)

	messages := []kit.Message{
		kit.NewUserMessage([]kit.Content{kit.NewTextContent("add 3 and 4")}),
	}

	a, err := New(model, []kit.Tool{tool}, kit.AgentConfig{Instructions: "use tools"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	response, err := a.Run(context.Background(), messages)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if response.Output.Text != "3 + 4 = 7" {
		t.Fatalf("Output = %q, want final text", response.Output.Text)
	}

	if response.Usage.InputTokens != 12 || response.Usage.OutputTokens != 5 {
		t.Fatalf("Usage = %+v, want input 12 output 5", response.Usage)
	}

	if len(response.Messages) != 3 {
		t.Fatalf("len(Messages) = %d, want 3", len(response.Messages))
	}

	tool.AssertCalledWith(t, 0, map[string]any{"a": 3, "b": 4})

	model.AssertCalledWith(t, 0, kit.ModelRequest{
		Instructions: "use tools",
		Messages:     messages,
		Tools:        []kit.Tool{tool},
	})

	model.AssertCalledWith(t, 1, kit.ModelRequest{
		Instructions: "use tools",
		Messages: append(messages,
			toolCallMessage,
			kit.NewToolMessage([]kit.Content{
				kit.NewToolResultContent(kit.NewToolResult(call, "7", nil)),
			}),
		),
		Tools: []kit.Tool{tool},
	})
}

func TestRun_SendsToolErrorBackToModel(t *testing.T) {
	toolErr := errors.New("tool failed")
	call := kit.NewToolCall("call-1", "fail", map[string]any{"input": "x"})
	toolCallMessage := kit.NewModelMessage([]kit.Content{kit.NewToolCallContent(call)})

	tool := kittest.NewTool(t, "fail", "failing tool", kittest.ToolResult{Error: toolErr})

	model := kittest.NewModel(t,
		kittest.ModelResult{
			Response: kit.ModelResponse{
				Message:      toolCallMessage,
				FinishReason: kit.FinishReasonToolCall,
			},
		},
		kittest.ModelResult{
			Response: kit.ModelResponse{
				Message:      kit.NewModelMessage([]kit.Content{kit.NewTextContent("handled")}),
				FinishReason: kit.FinishReasonStop,
			},
		},
	)

	a, err := New(model, []kit.Tool{tool}, kit.AgentConfig{Instructions: "use tools"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, err = a.Run(context.Background(), []kit.Message{
		kit.NewUserMessage([]kit.Content{kit.NewTextContent("try failing tool")}),
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	secondRequest := model.CallAt(1)

	results := secondRequest.Messages[len(secondRequest.Messages)-1].ToolResults()
	if len(results) != 1 {
		t.Fatalf("len(ToolResults) = %d, want 1", len(results))
	}

	if results[0].Error != "tool failed" {
		t.Fatalf("tool result error = %q, want tool failed", results[0].Error)
	}

	if results[0].Output != "" {
		t.Fatalf("tool result output = %q, want empty", results[0].Output)
	}
}

func TestRun_SendsMissingToolErrorBackToModel(t *testing.T) {
	call := kit.NewToolCall("call-1", "missing", map[string]any{"input": "x"})

	model := kittest.NewModel(t,
		kittest.ModelResult{
			Response: kit.ModelResponse{
				Message:      kit.NewModelMessage([]kit.Content{kit.NewToolCallContent(call)}),
				FinishReason: kit.FinishReasonToolCall,
			},
		},
		kittest.ModelResult{
			Response: kit.ModelResponse{
				Message:      kit.NewModelMessage([]kit.Content{kit.NewTextContent("handled")}),
				FinishReason: kit.FinishReasonStop,
			},
		},
	)

	a, err := New(model, nil, kit.AgentConfig{Instructions: "use tools"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, err = a.Run(context.Background(), []kit.Message{
		kit.NewUserMessage([]kit.Content{kit.NewTextContent("try missing tool")}),
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	secondRequest := model.CallAt(1)

	results := secondRequest.Messages[len(secondRequest.Messages)-1].ToolResults()
	if len(results) != 1 {
		t.Fatalf("len(ToolResults) = %d, want 1", len(results))
	}

	if results[0].Error != `tool "missing" not found` {
		t.Fatalf("tool result error = %q, want missing tool error", results[0].Error)
	}
}

type panicTool struct {
	name string
}

func (p panicTool) Definition() kit.ToolDefinition {
	return kit.ToolDefinition{Name: p.name}
}

func (p panicTool) Execute(context.Context, map[string]any) (string, error) {
	panic("bad things")
}

func TestExecuteTool_Success(t *testing.T) {
	tool := kittest.NewTool(t, "ok", "ok tool", kittest.ToolResult{Result: "done"})

	a, err := New(nil, []kit.Tool{tool}, kit.AgentConfig{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	call := kit.NewToolCall("call-1", "ok", map[string]any{"value": "x"})

	result, err := a.executeTool(&kit.RunContext{Context: context.Background()}, call)
	if err != nil {
		t.Fatalf("executeTool: %v", err)
	}

	if result.Output != "done" {
		t.Fatalf("Output = %q, want %q", result.Output, "done")
	}

	if result.Error != "" {
		t.Fatalf("Error = %q, want empty", result.Error)
	}

	tool.AssertCalledWith(t, 0, map[string]any{"value": "x"})
}

func TestExecuteTool_Error(t *testing.T) {
	tool := kittest.NewTool(t, "fail", "fail tool", kittest.ToolResult{Error: errors.New("boom")})

	a, err := New(nil, []kit.Tool{tool}, kit.AgentConfig{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	result, err := a.executeTool(&kit.RunContext{Context: context.Background()}, kit.NewToolCall("call-1", "fail", nil))
	if err != nil {
		t.Fatalf("executeTool: %v", err)
	}

	if result.Error != "boom" {
		t.Fatalf("Error = %q, want %q", result.Error, "boom")
	}
}

func TestExecuteTool_NotFound(t *testing.T) {
	a, err := New(nil, nil, kit.AgentConfig{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	result, err := a.executeTool(&kit.RunContext{Context: context.Background()}, kit.NewToolCall("call-1", "missing", nil))
	if err != nil {
		t.Fatalf("executeTool: %v", err)
	}

	if result.Error != `tool "missing" not found` {
		t.Fatalf("Error = %q, want tool not found error", result.Error)
	}
}

func TestExecuteTool_RecoversPanic(t *testing.T) {
	tool := panicTool{name: "panic"}

	a, err := New(nil, []kit.Tool{tool}, kit.AgentConfig{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	result, err := a.executeTool(&kit.RunContext{Context: context.Background()}, kit.NewToolCall("call-1", "panic", nil))
	if err != nil {
		t.Fatalf("executeTool: %v", err)
	}

	if result.Output != "" {
		t.Fatalf("Output = %q, want empty", result.Output)
	}

	if !strings.Contains(result.Error, `tool "panic" panicked: bad things`) {
		t.Fatalf("Error = %q, want panic recovery error", result.Error)
	}
}

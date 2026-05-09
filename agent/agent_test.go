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

	a, err := New(model, WithTools(tool), WithInstructions("be brief"))
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

	a, err := New(model, WithInstructions("be brief"))
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

func TestRun_ReturnsCanceledContextBeforeModelCall(t *testing.T) {
	model := kittest.NewModel(t)

	a, err := New(model, WithInstructions("be brief"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	resp, err := a.Run(ctx, []kit.Message{
		kit.NewUserMessage([]kit.Content{kit.NewTextContent("hello")}),
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run error = %v, want context canceled", err)
	}

	if len(resp.Messages) != 0 {
		t.Fatalf("len(Messages) = %d, want 0", len(resp.Messages))
	}

	model.AssertNeverCalled(t)
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

	a, err := New(model, WithTools(tool), WithInstructions("use tools"))
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

	a, err := New(model, WithTools(tool), WithInstructions("use tools"))
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

	a, err := New(model, WithInstructions("use tools"))
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

type cancelTool struct {
	cancel context.CancelFunc
	called bool
}

func (c *cancelTool) Definition() kit.ToolDefinition {
	return kit.ToolDefinition{Name: "cancel"}
}

func (c *cancelTool) Execute(ctx context.Context, _ map[string]any) (string, error) {
	c.called = true
	c.cancel()

	return "", ctx.Err()
}

func TestExecuteTool_Success(t *testing.T) {
	tool := kittest.NewTool(t, "ok", "ok tool", kittest.ToolResult{Result: "done"})

	a, err := New(nil, WithTools(tool))
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

	a, err := New(nil, WithTools(tool))
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
	a, err := New(nil)
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

	a, err := New(nil, WithTools(tool))
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

func TestRun_ContextCanceledDuringToolAbortsRun(t *testing.T) {
	call := kit.NewToolCall("call-1", "cancel", nil)
	toolCallMessage := kit.NewModelMessage([]kit.Content{kit.NewToolCallContent(call)})

	model := kittest.NewModel(t, kittest.ModelResult{
		Response: kit.ModelResponse{
			Message:      toolCallMessage,
			FinishReason: kit.FinishReasonToolCall,
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	tool := &cancelTool{cancel: cancel}

	a, err := New(model, WithTools(tool), WithInstructions("use tools"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	resp, err := a.Run(ctx, []kit.Message{
		kit.NewUserMessage([]kit.Content{kit.NewTextContent("cancel during tool")}),
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run error = %v, want context canceled", err)
	}

	if !tool.called {
		t.Fatal("tool was not called")
	}

	model.AssertCallCount(t, 1)

	if len(resp.Messages) != 1 {
		t.Fatalf("len(Messages) = %d, want only the model tool-call message", len(resp.Messages))
	}

	if got := len(resp.Messages[0].ToolCalls()); got != 1 {
		t.Fatalf("len(ToolCalls) = %d, want 1", got)
	}
}

func TestStream_ReturnsEventsAndResult(t *testing.T) {
	model := kittest.NewModel(t, kittest.ModelResult{
		Events: []kit.ModelEvent{
			kit.NewModelContentStartedEvent(kit.NewTextContent("done")),
			kit.NewModelContentDoneEvent(kit.NewTextContent("done")),
		},
		Response: kit.ModelResponse{
			Message:      kit.NewModelMessage([]kit.Content{kit.NewTextContent("done")}),
			FinishReason: kit.FinishReasonStop,
			Usage:        kit.Usage{InputTokens: 3, OutputTokens: 2},
		},
	})

	a, err := New(model, WithInstructions("be brief"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	stream := a.Stream(context.Background(), []kit.Message{
		kit.NewUserMessage([]kit.Content{kit.NewTextContent("hello")}),
	})

	var events []kit.AgentEvent
	for event := range stream.Iter() {
		events = append(events, event)
	}

	if len(events) != 3 {
		t.Fatalf("len(events) = %d, want 3", len(events))
	}

	if events[0].Type != kit.EventContentStarted {
		t.Fatalf("events[0].Type = %q, want content_started", events[0].Type)
	}

	if events[0].Content == nil {
		t.Fatal("events[0].Content is nil")
	}

	if events[1].Type != kit.EventContentDone {
		t.Fatalf("events[1].Type = %q, want content_done", events[1].Type)
	}

	if events[2].Type != kit.EventMessage {
		t.Fatalf("events[2].Type = %q, want message", events[2].Type)
	}

	if events[2].Message == nil || events[2].Message.TextContent().Text != "done" {
		t.Fatalf("events[2].Message = %+v, want final model message", events[2].Message)
	}

	result, err := stream.Result()
	if err != nil {
		t.Fatalf("Result: %v", err)
	}

	if result.Output.Text != "done" {
		t.Fatalf("Output = %q, want done", result.Output.Text)
	}

	if result.Usage.InputTokens != 3 || result.Usage.OutputTokens != 2 {
		t.Fatalf("Usage = %+v, want input 3 output 2", result.Usage)
	}
}

func TestStream_EmitsToolResultAndContinues(t *testing.T) {
	call := kit.NewToolCall("call-1", "add", map[string]any{"a": 3, "b": 4})
	toolCallMessage := kit.NewModelMessage([]kit.Content{kit.NewToolCallContent(call)})
	finalMessage := kit.NewModelMessage([]kit.Content{kit.NewTextContent("3 + 4 = 7")})

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
				Message:      finalMessage,
				FinishReason: kit.FinishReasonStop,
			},
		},
	)

	a, err := New(model, WithTools(tool), WithInstructions("use tools"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	stream := a.Stream(context.Background(), []kit.Message{
		kit.NewUserMessage([]kit.Content{kit.NewTextContent("add 3 and 4")}),
	})

	var sawToolResult bool

	for event := range stream.Iter() {
		if event.ToolResult != nil {
			sawToolResult = true

			if event.ToolResult.Output != "7" {
				t.Fatalf("ToolResult.Output = %q, want 7", event.ToolResult.Output)
			}
		}
	}

	if !sawToolResult {
		t.Fatal("stream did not emit tool result")
	}

	result, err := stream.Result()
	if err != nil {
		t.Fatalf("Result: %v", err)
	}

	if result.Output.Text != "3 + 4 = 7" {
		t.Fatalf("Output = %q, want final text", result.Output.Text)
	}

	tool.AssertCalledWith(t, 0, map[string]any{"a": 3, "b": 4})
	model.AssertCallCount(t, 2)
}

func TestStream_ResultAfterEarlyStopReturnsPartialResponse(t *testing.T) {
	model := kittest.NewModel(t, kittest.ModelResult{
		Events: []kit.ModelEvent{
			kit.NewModelContentStartedEvent(kit.NewTextContent("done")),
			kit.NewModelContentDoneEvent(kit.NewTextContent("done")),
		},
		Response: kit.ModelResponse{
			Message:      kit.NewModelMessage([]kit.Content{kit.NewTextContent("done")}),
			FinishReason: kit.FinishReasonStop,
			Usage:        kit.Usage{InputTokens: 3, OutputTokens: 2},
		},
	})

	a, err := New(model, WithInstructions("be brief"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	stream := a.Stream(context.Background(), []kit.Message{
		kit.NewUserMessage([]kit.Content{kit.NewTextContent("hello")}),
	})

	for range stream.Iter() {
		break
	}

	result, err := stream.Result()
	if err != nil {
		t.Fatalf("Result: %v", err)
	}

	if len(result.Messages) != 0 {
		t.Fatalf("len(Messages) = %d, want 0 because final model response was not appended yet", len(result.Messages))
	}
}

func TestExecuteTool_OnToolCallHookErrorBecomesResult(t *testing.T) {
	tool := kittest.NewTool(t, "ok", "ok tool")

	hookErr := errors.New("policy denied")

	a, err := New(nil,
		WithTools(tool),
		WithOnToolCall(func(_ *kit.RunContext, _ kit.ToolCall) (kit.ToolCall, error) {
			return kit.ToolCall{}, hookErr
		}),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	call := kit.NewToolCall("call-1", "ok", map[string]any{"value": "x"})

	result, err := a.executeTool(&kit.RunContext{Context: context.Background()}, call)
	if err != nil {
		t.Fatalf("executeTool: %v", err)
	}

	if result.Call.ID != "call-1" {
		t.Fatalf("result.Call.ID = %q, want call-1", result.Call.ID)
	}

	if result.Error != "policy denied" {
		t.Fatalf("Error = %q, want policy denied", result.Error)
	}

	if result.Output != "" {
		t.Fatalf("Output = %q, want empty (tool must not run)", result.Output)
	}

	tool.AssertNeverCalled(t)
}

func TestExecuteTool_OnToolResultHookErrorBecomesResult(t *testing.T) {
	tool := kittest.NewTool(t, "ok", "ok tool", kittest.ToolResult{Result: "raw output"})

	hookErr := errors.New("redaction failed")

	a, err := New(nil,
		WithTools(tool),
		WithOnToolResult(func(_ *kit.RunContext, _ kit.ToolResult) (kit.ToolResult, error) {
			return kit.ToolResult{}, hookErr
		}),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	call := kit.NewToolCall("call-1", "ok", map[string]any{"value": "x"})

	result, err := a.executeTool(&kit.RunContext{Context: context.Background()}, call)
	if err != nil {
		t.Fatalf("executeTool: %v", err)
	}

	if result.Call.ID != "call-1" {
		t.Fatalf("result.Call.ID = %q, want call-1", result.Call.ID)
	}

	if result.Error != "redaction failed" {
		t.Fatalf("Error = %q, want redaction failed", result.Error)
	}

	if result.Output != "" {
		t.Fatalf("Output = %q, want empty (output replaced by hook error)", result.Output)
	}

	tool.AssertCallCount(t, 1)
}

func TestWithTools_DuplicateNameKeepsFirst(t *testing.T) {
	first := kittest.NewTool(t, "search", "v1", kittest.ToolResult{Result: "from v1"})
	other := kittest.NewTool(t, "calc", "calc")
	second := kittest.NewTool(t, "search", "v2")

	a, err := New(nil, WithTools(first, other), WithTools(second))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if len(a.tools) != 2 {
		t.Fatalf("len(tools) = %d, want 2", len(a.tools))
	}

	if a.tools[0].Definition().Description != "v1" {
		t.Fatalf("tools[0] description = %q, want v1 (first wins)", a.tools[0].Definition().Description)
	}

	if a.tools[1].Definition().Name != "calc" {
		t.Fatalf("tools[1] name = %q, want calc", a.tools[1].Definition().Name)
	}

	result, err := a.executeTool(&kit.RunContext{Context: context.Background()}, kit.NewToolCall("call-1", "search", nil))
	if err != nil {
		t.Fatalf("executeTool: %v", err)
	}

	if result.Output != "from v1" {
		t.Fatalf("Output = %q, want from v1 (first registration wins)", result.Output)
	}

	first.AssertCallCount(t, 1)
	second.AssertNeverCalled(t)
}

func TestRun_ToolHookErrorIsSentBackToModel(t *testing.T) {
	call := kit.NewToolCall("call-1", "search", map[string]any{"q": "go"})

	tool := kittest.NewTool(t, "search", "search tool")

	model := kittest.NewModel(t,
		kittest.ModelResult{
			Response: kit.ModelResponse{
				Message:      kit.NewModelMessage([]kit.Content{kit.NewToolCallContent(call)}),
				FinishReason: kit.FinishReasonToolCall,
			},
		},
		kittest.ModelResult{
			Response: kit.ModelResponse{
				Message:      kit.NewModelMessage([]kit.Content{kit.NewTextContent("recovered")}),
				FinishReason: kit.FinishReasonStop,
			},
		},
	)

	a, err := New(model,
		WithTools(tool),
		WithOnToolCall(func(_ *kit.RunContext, _ kit.ToolCall) (kit.ToolCall, error) {
			return kit.ToolCall{}, errors.New("policy denied")
		}),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	resp, err := a.Run(context.Background(), []kit.Message{
		kit.NewUserMessage([]kit.Content{kit.NewTextContent("search go")}),
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if resp.Output.Text != "recovered" {
		t.Fatalf("Output = %q, want recovered", resp.Output.Text)
	}

	results := model.CallAt(1).Messages[len(model.CallAt(1).Messages)-1].ToolResults()
	if len(results) != 1 {
		t.Fatalf("len(ToolResults) = %d, want 1", len(results))
	}

	if results[0].Call.ID != "call-1" {
		t.Fatalf("ToolResult.Call.ID = %q, want call-1", results[0].Call.ID)
	}

	if results[0].Error != "policy denied" {
		t.Fatalf("ToolResult.Error = %q, want policy denied", results[0].Error)
	}

	tool.AssertNeverCalled(t)
}

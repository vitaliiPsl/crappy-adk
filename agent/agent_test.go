package agent_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"

	"github.com/vitaliiPsl/crappy-adk/agent"
	"github.com/vitaliiPsl/crappy-adk/kit"
	"github.com/vitaliiPsl/crappy-adk/kittest"
)

func TestAgent_Run_TextOnly(t *testing.T) {
	model := kittest.NewModel(t,
		kittest.ModelTurn{Text: "Hello there!"},
	)

	agent, err := agent.New(model)
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}

	resp, err := agent.Run(context.Background(), []kit.Message{
		kit.NewUserMessage(kit.NewTextPart("Hi")),
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if got := resp.Output.Text; got != "Hello there!" {
		t.Errorf("text = %q, want %q", got, "Hello there!")
	}

	model.AssertCallCount(t, 1)
}

func TestAgent_Run_FinalOutputSkipsThinkingPart(t *testing.T) {
	model := kittest.NewModel(t,
		kittest.ModelTurn{Text: "Final answer", Thinking: "internal reasoning"},
	)

	agent, err := agent.New(model)
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}

	resp, err := agent.Run(context.Background(), []kit.Message{
		kit.NewUserMessage(kit.NewTextPart("Hi")),
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if got := resp.Output.Type; got != kit.ContentTypeText {
		t.Fatalf("output.type = %q, want %q", got, kit.ContentTypeText)
	}

	if got := resp.Output.Text; got != "Final answer" {
		t.Fatalf("output.text = %q, want %q", got, "Final answer")
	}
}

func TestMessage_Output_FallsBackToThinkingWhenItIsTheOnlyContent(t *testing.T) {
	msg := kit.Message{
		Role: kit.MessageRoleAssistant,
		Content: []kit.ContentPart{
			kit.NewThinkingPart("internal reasoning", ""),
		},
	}

	output := msg.Output()
	if got := output.Type; got != kit.ContentTypeThinking {
		t.Fatalf("output.type = %q, want %q", got, kit.ContentTypeThinking)
	}

	if got := output.Text; got != "internal reasoning" {
		t.Fatalf("output.text = %q, want %q", got, "internal reasoning")
	}
}

func TestAgent_Run_WithResponseSchemaPassesSchemaAndReturnsStructuredOutput(t *testing.T) {
	schema := &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"answer": {Type: "string"},
		},
		Required: []string{"answer"},
	}

	model := kittest.NewModel(t,
		kittest.ModelTurn{
			Text: "ignored for structured output consumers",
			StructuredOutput: &kit.StructuredOutput{
				JSON: []byte(`{"answer":"done"}`),
			},
		},
	)

	agent, err := agent.New(model, agent.WithResponseSchema(schema))
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}

	resp, err := agent.Run(context.Background(), []kit.Message{
		kit.NewUserMessage(kit.NewTextPart("Hi")),
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if resp.StructuredOutput == nil {
		t.Fatal("expected structured output, got nil")
	}

	if got := string(resp.StructuredOutput.JSON); got != `{"answer":"done"}` {
		t.Fatalf("structured output json = %q, want %q", got, `{"answer":"done"}`)
	}

	req := model.CallAt(0)
	if req.ResponseSchema == nil {
		t.Fatal("expected response schema on model request")
	}

	if req.ResponseSchema.Type != "object" {
		t.Fatalf("response schema type = %q, want %q", req.ResponseSchema.Type, "object")
	}
}

func TestAgent_Run_WithResponseSchemaForInfersSchema(t *testing.T) {
	type releaseNotes struct {
		Title    string `json:"title"`
		Breaking bool   `json:"breaking"`
	}

	model := kittest.NewModel(t,
		kittest.ModelTurn{Text: "done"},
	)

	agent, err := agent.New(model, agent.WithResponseSchemaFor[releaseNotes]())
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}

	_, err = agent.Run(context.Background(), []kit.Message{
		kit.NewUserMessage(kit.NewTextPart("Hi")),
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	req := model.CallAt(0)
	if req.ResponseSchema == nil {
		t.Fatal("expected response schema on model request")
	}

	if req.ResponseSchema.Type != "object" {
		t.Fatalf("response schema type = %q, want %q", req.ResponseSchema.Type, "object")
	}

	if req.ResponseSchema.Properties["title"] == nil {
		t.Fatal("expected title property in inferred response schema")
	}

	if req.ResponseSchema.AdditionalProperties == nil {
		t.Fatal("expected inferred struct schema to disallow additional properties")
	}
}

func TestAgent_Run_ToolCall(t *testing.T) {
	searchTool := kittest.NewTool(t, "search", "Search the web",
		kittest.ToolResponse{Result: `{"results": ["Crappy is not that crappy"]}`},
	)

	model := kittest.NewModel(t,
		kittest.ModelTurn{ToolCalls: []kit.ToolCall{
			{ID: "call_1", Name: "search", Arguments: map[string]any{"query": "Crappy"}},
		}},
		kittest.ModelTurn{Text: "Crappy is not that crappy"},
	)

	agent, err := agent.New(model, agent.WithTools(searchTool))
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}

	resp, err := agent.Run(context.Background(), []kit.Message{
		kit.NewUserMessage(kit.NewTextPart("Tell me about Crappy")),
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if got := resp.Output.Text; got != "Crappy is not that crappy" {
		t.Errorf("text = %q, want %q", got, "Crappy is not that crappy")
	}

	model.AssertCallCount(t, 2)
	model.AssertToolCalled(t, "search")

	searchTool.AssertCallCount(t, 1)
	searchTool.AssertCalledWith(t, 0, map[string]any{"query": "Crappy"})
}

func TestAgent_Run_ToolCallFeedsAssistantAndToolMessagesToNextModelTurn(t *testing.T) {
	searchTool := kittest.NewTool(t, "search", "Search the web",
		kittest.ToolResponse{Result: `{"results": ["Crappy is not that crappy"]}`},
	)

	model := kittest.NewModel(t,
		kittest.ModelTurn{ToolCalls: []kit.ToolCall{
			{ID: "call_1", Name: "search", Arguments: map[string]any{"query": "Crappy"}},
		}},
		kittest.ModelTurn{Text: "Crappy is not that crappy"},
	)

	agent, err := agent.New(model, agent.WithTools(searchTool))
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}

	_, err = agent.Run(context.Background(), []kit.Message{
		kit.NewUserMessage(kit.NewTextPart("Tell me about Crappy")),
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	req := model.CallAt(1)
	if len(req.Messages) != 3 {
		t.Fatalf("len(messages) = %d, want %d", len(req.Messages), 3)
	}

	if req.Messages[1].Role != kit.MessageRoleAssistant {
		t.Fatalf("messages[1].role = %q, want %q", req.Messages[1].Role, kit.MessageRoleAssistant)
	}

	if got := len(req.Messages[1].ToolCalls()); got != 1 {
		t.Fatalf("len(messages[1].tool_calls) = %d, want %d", got, 1)
	}

	if req.Messages[1].ToolCalls()[0].ID != "call_1" {
		t.Fatalf("messages[1].tool_calls[0].id = %q, want %q", req.Messages[1].ToolCalls()[0].ID, "call_1")
	}

	if req.Messages[2].Role != kit.MessageRoleTool {
		t.Fatalf("messages[2].role = %q, want %q", req.Messages[2].Role, kit.MessageRoleTool)
	}

	if part, ok := req.Messages[2].ToolResult(); !ok || part.ID != "call_1" {
		t.Fatalf("messages[2].tool_result.id = %q, want %q", part.ID, "call_1")
	}

	if got := req.Messages[2].Text(); got != `{"results": ["Crappy is not that crappy"]}` {
		t.Fatalf("messages[2].text = %q, want %q", got, `{"results": ["Crappy is not that crappy"]}`)
	}
}

func TestAgent_Run_MultipleToolCalls(t *testing.T) {
	searchTool := kittest.NewTool(t, "search", "Search",
		kittest.ToolResponse{Result: "result A"},
		kittest.ToolResponse{Result: "result B"},
	)

	model := kittest.NewModel(t,
		kittest.ModelTurn{ToolCalls: []kit.ToolCall{
			{ID: "call_1", Name: "search", Arguments: map[string]any{"q": "A"}},
			{ID: "call_2", Name: "search", Arguments: map[string]any{"q": "B"}},
		}},
		kittest.ModelTurn{Text: "Got both results."},
	)

	agent, err := agent.New(model,
		agent.WithTools(searchTool),
	)
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}

	resp, err := agent.Run(context.Background(), []kit.Message{
		kit.NewUserMessage(kit.NewTextPart("Search for A and B")),
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if got := resp.Output.Text; got != "Got both results." {
		t.Errorf("text = %q, want %q", got, "Got both results.")
	}

	searchTool.AssertCallCount(t, 2)
}

func TestAgent_Run_MultiTurn(t *testing.T) {
	readTool := kittest.NewTool(t, "read", "Read file",
		kittest.ToolResponse{Result: "file contents"},
	)
	writeTool := kittest.NewTool(t, "write", "Write file",
		kittest.ToolResponse{Result: "ok"},
	)

	model := kittest.NewModel(t,
		kittest.ModelTurn{ToolCalls: []kit.ToolCall{
			{ID: "call_1", Name: "read", Arguments: map[string]any{"path": "main.go"}},
		}},
		kittest.ModelTurn{ToolCalls: []kit.ToolCall{
			{ID: "call_2", Name: "write", Arguments: map[string]any{"path": "main.go", "content": "updated"}},
		}},
		kittest.ModelTurn{Text: "Done."},
	)

	agent, err := agent.New(model,
		agent.WithTools(readTool, writeTool),
	)
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}

	resp, err := agent.Run(context.Background(), []kit.Message{
		kit.NewUserMessage(kit.NewTextPart("Update main.go")),
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if got := resp.Output.Text; got != "Done." {
		t.Errorf("text = %q, want %q", got, "Done.")
	}

	model.AssertCallCount(t, 3)
	readTool.AssertCallCount(t, 1)
	writeTool.AssertCallCount(t, 1)
}

func TestAgent_Run_ToolNotFound(t *testing.T) {
	model := kittest.NewModel(t,
		kittest.ModelTurn{ToolCalls: []kit.ToolCall{
			{ID: "call_1", Name: "missing_tool", Arguments: map[string]any{}},
		}},
		kittest.ModelTurn{Text: "Sorry, that tool is unavailable."},
	)

	agent, err := agent.New(model)
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}

	resp, err := agent.Run(context.Background(), []kit.Message{
		kit.NewUserMessage(kit.NewTextPart("Use missing tool")),
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Agent should still complete, tool error is fed back to model as a tool message
	model.AssertCallCount(t, 2)

	if got := resp.Output.Text; got != "Sorry, that tool is unavailable." {
		t.Errorf("text = %q, want %q", got, "Sorry, that tool is unavailable.")
	}
}

func TestAgent_Run_ToolError(t *testing.T) {
	failTool := kittest.NewTool(t, "fail", "Always fails",
		kittest.ToolResponse{Error: errors.New("connection refused")},
	)

	model := kittest.NewModel(t,
		kittest.ModelTurn{ToolCalls: []kit.ToolCall{
			{ID: "call_1", Name: "fail", Arguments: map[string]any{}},
		}},
		kittest.ModelTurn{Text: "The tool failed."},
	)

	agent, err := agent.New(model, agent.WithTools(failTool))
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}

	resp, err := agent.Run(context.Background(), []kit.Message{
		kit.NewUserMessage(kit.NewTextPart("Run the failing tool")),
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if got := resp.Output.Text; got != "The tool failed." {
		t.Errorf("text = %q, want %q", got, "The tool failed.")
	}

	failTool.AssertCallCount(t, 1)
}

func TestAgent_Run_ModelError(t *testing.T) {
	model := kittest.NewModel(t,
		kittest.ModelTurn{Error: errors.New("model unavailable")},
	)

	agent, err := agent.New(model)
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}

	_, err = agent.Run(context.Background(), []kit.Message{
		kit.NewUserMessage(kit.NewTextPart("Hi")),
	})
	if err == nil {
		t.Fatal("expected error from Run")
	}

	if got := err.Error(); got != "model unavailable" {
		t.Errorf("error = %q, want %q", got, "model unavailable")
	}
}

func TestAgent_Run_ContextLengthErrorCompactsAndRetries(t *testing.T) {
	model := kittest.NewModel(t,
		kittest.ModelTurn{Error: kit.ErrContextLength},
		kittest.ModelTurn{Text: "Recovered after compaction."},
	)

	compactor := &stubCompactor{
		compacted: []kit.Message{
			kit.NewSummaryMessage("summary"),
			kit.NewUserMessage(kit.NewTextPart("Hi")),
		},
		summary: "summary",
	}

	agent, err := agent.New(model, agent.WithCompactor(compactor))
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}

	resp, err := agent.Run(context.Background(), []kit.Message{
		kit.NewUserMessage(kit.NewTextPart("Hi")),
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if got := resp.Output.Text; got != "Recovered after compaction." {
		t.Fatalf("text = %q, want %q", got, "Recovered after compaction.")
	}

	model.AssertCallCount(t, 2)

	if compactor.calls != 1 {
		t.Fatalf("compactor calls = %d, want %d", compactor.calls, 1)
	}

	req := model.CallAt(1)
	if len(req.Messages) != 2 {
		t.Fatalf("len(messages) = %d, want %d", len(req.Messages), 2)
	}

	if !req.Messages[0].IsSummary {
		t.Fatal("expected compacted history to start with summary message")
	}
}

func TestAgent_Run_ContextLengthErrorUsesCompactedMessagesWithoutSummary(t *testing.T) {
	model := kittest.NewModel(t,
		kittest.ModelTurn{Error: kit.ErrContextLength},
		kittest.ModelTurn{Text: "Recovered after sliding window."},
	)

	compactor := &stubCompactor{
		compacted: []kit.Message{
			kit.NewUserMessage(kit.NewTextPart("Most recent message")),
		},
	}

	agent, err := agent.New(model, agent.WithCompactor(compactor))
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}

	resp, err := agent.Run(context.Background(), []kit.Message{
		kit.NewUserMessage(kit.NewTextPart("Old message")),
		kit.NewUserMessage(kit.NewTextPart("Most recent message")),
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if got := resp.Output.Text; got != "Recovered after sliding window." {
		t.Fatalf("text = %q, want %q", got, "Recovered after sliding window.")
	}

	req := model.CallAt(1)
	if len(req.Messages) != 1 {
		t.Fatalf("len(messages) = %d, want %d", len(req.Messages), 1)
	}

	if got := req.Messages[0].Text(); got != "Most recent message" {
		t.Fatalf("messages[0].text = %q, want %q", got, "Most recent message")
	}

	if len(resp.Messages) != 1 {
		t.Fatalf("len(result.messages) = %d, want only final assistant message", len(resp.Messages))
	}
}

func TestAgent_Run_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	model := kittest.NewModel(t)

	agent, err := agent.New(model)
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}

	_, err = agent.Run(ctx, []kit.Message{
		kit.NewUserMessage(kit.NewTextPart("Hi")),
	})
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}

	if !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v, want context.Canceled", err)
	}
}

type stubCompactor struct {
	calls     int
	compacted []kit.Message
	summary   string
	err       error
}

func (c *stubCompactor) Compact(_ context.Context, _ []kit.Message) ([]kit.Message, string, error) {
	c.calls++

	return c.compacted, c.summary, c.err
}

func TestAgent_Run_SystemPrompt(t *testing.T) {
	model := kittest.NewModel(t,
		kittest.ModelTurn{Text: "I am a helpful bot."},
	)

	agent, err := agent.New(model, agent.WithSystemPrompt("You are a helpful bot."))
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}

	_, err = agent.Run(context.Background(), []kit.Message{
		kit.NewUserMessage(kit.NewTextPart("Who are you?")),
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	req := model.CallAt(0)
	if req.Instruction != "You are a helpful bot." {
		t.Errorf("instruction = %q, want %q", req.Instruction, "You are a helpful bot.")
	}
}

func TestAgent_New_WithInstructionsAppendsToSystemPrompt(t *testing.T) {
	model := kittest.NewModel(t,
		kittest.ModelTurn{Text: "I am a helpful bot."},
	)

	agent, err := agent.New(model,
		agent.WithSystemPrompt("You are a helpful bot."),
		agent.WithInstructions(
			func() (string, error) {
				return "Use concise answers.", nil
			},
			func() (string, error) {
				return "Prefer Go examples.", nil
			},
		),
	)
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}

	_, err = agent.Run(context.Background(), []kit.Message{
		kit.NewUserMessage(kit.NewTextPart("Who are you?")),
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	req := model.CallAt(0)

	want := "You are a helpful bot.\n\nUse concise answers.\n\nPrefer Go examples."
	if req.Instruction != want {
		t.Errorf("instruction = %q, want %q", req.Instruction, want)
	}
}

func TestAgent_New_WithInstructionsEvaluatesOnce(t *testing.T) {
	model := kittest.NewModel(t,
		kittest.ModelTurn{Text: "First."},
		kittest.ModelTurn{Text: "Second."},
	)

	calls := 0

	agent, err := agent.New(model,
		agent.WithInstructions(func() (string, error) {
			calls++

			return "Stable instruction.", nil
		}),
	)
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}

	for range 2 {
		_, err = agent.Run(context.Background(), []kit.Message{
			kit.NewUserMessage(kit.NewTextPart("Hi")),
		})
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
	}

	if calls != 1 {
		t.Fatalf("instruction calls = %d, want 1", calls)
	}

	for i := range 2 {
		req := model.CallAt(i)
		if req.Instruction != "Stable instruction." {
			t.Fatalf("call %d instruction = %q, want %q", i, req.Instruction, "Stable instruction.")
		}
	}
}

func TestAgent_Run_UsageAccumulated(t *testing.T) {
	model := kittest.NewModel(t,
		kittest.ModelTurn{
			ToolCalls: []kit.ToolCall{{ID: "call_1", Name: "noop", Arguments: map[string]any{}}},
			Usage:     kit.Usage{InputTokens: 100, OutputTokens: 50},
		},
		kittest.ModelTurn{
			Text:  "Done.",
			Usage: kit.Usage{InputTokens: 200, OutputTokens: 30},
		},
	)

	noopTool := kittest.NewTool(t, "noop", "Does nothing",
		kittest.ToolResponse{Result: "ok"},
	)

	agent, err := agent.New(model, agent.WithTools(noopTool))
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}

	resp, err := agent.Run(context.Background(), []kit.Message{
		kit.NewUserMessage(kit.NewTextPart("Do something")),
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if resp.Usage.InputTokens != 300 {
		t.Errorf("InputTokens = %d, want 300", resp.Usage.InputTokens)
	}

	if resp.Usage.OutputTokens != 80 {
		t.Errorf("OutputTokens = %d, want 80", resp.Usage.OutputTokens)
	}
}

func TestAgent_Stream_Events(t *testing.T) {
	model := kittest.NewModel(t,
		kittest.ModelTurn{
			Text: "Hello world",
			Stream: []kittest.ChunkResult{
				{Event: kit.NewContentPartStartedEvent(kit.ContentTypeText)},
				{Event: kit.NewContentPartDeltaEvent(kit.ContentTypeText, "Hello ")},
				{Event: kit.NewContentPartDeltaEvent(kit.ContentTypeText, "world")},
				{Event: kit.NewContentPartDoneEvent(kit.NewTextPart("Hello world"))},
			},
		},
	)

	agent, err := agent.New(model)
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}

	stream, err := agent.Stream(context.Background(), []kit.Message{
		kit.NewUserMessage(kit.NewTextPart("Hi")),
	})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}

	var eventTypes []kit.EventType

	for event := range stream.Iter() {
		eventTypes = append(eventTypes, event.Type)
	}

	if _, err := stream.Result(); err != nil {
		t.Fatalf("stream error: %v", err)
	}

	expected := []kit.EventType{
		kit.EventContentPartStarted,
		kit.EventContentPartDelta,
		kit.EventContentPartDelta,
		kit.EventContentPartDone,
		kit.EventMessage,
	}
	if len(eventTypes) != len(expected) {
		t.Fatalf("event count = %d, want %d: %v", len(eventTypes), len(expected), eventTypes)
	}

	for idx, got := range eventTypes {
		if got != expected[idx] {
			t.Errorf("event[%d] = %q, want %q", idx, got, expected[idx])
		}
	}
}

func TestAgent_Stream_ToolCallEvents(t *testing.T) {
	searchCall := kit.ToolCall{ID: "call_1", Name: "search", Arguments: map[string]any{"q": "test"}}

	tool := kittest.NewTool(t, "search", "Search",
		kittest.ToolResponse{Result: "found it"},
	)

	model := kittest.NewModel(t,
		kittest.ModelTurn{
			ToolCalls: []kit.ToolCall{searchCall},
			Stream: []kittest.ChunkResult{
				{Event: kit.NewContentPartStartedEvent(kit.ContentTypeThinking)},
				{Event: kit.NewContentPartDeltaEvent(kit.ContentTypeThinking, "let me search")},
				{Event: kit.NewContentPartDoneEvent(kit.NewThinkingPart("let me search", ""))},
				{Event: kit.NewContentPartStartedEvent(kit.ContentTypeToolCall)},
				{Event: kit.NewContentPartDoneEvent(kit.NewToolCallPart(searchCall))},
			},
		},
		kittest.ModelTurn{
			Text: "Here you go.",
			Stream: []kittest.ChunkResult{
				{Event: kit.NewContentPartStartedEvent(kit.ContentTypeText)},
				{Event: kit.NewContentPartDeltaEvent(kit.ContentTypeText, "Here you go.")},
				{Event: kit.NewContentPartDoneEvent(kit.NewTextPart("Here you go."))},
			},
		},
	)

	agent, err := agent.New(model,
		agent.WithTools(tool),
	)
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}

	stream, err := agent.Stream(context.Background(), []kit.Message{
		kit.NewUserMessage(kit.NewTextPart("Search for test")),
	})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}

	var eventTypes []kit.EventType

	for event := range stream.Iter() {
		eventTypes = append(eventTypes, event.Type)
	}

	if _, err := stream.Result(); err != nil {
		t.Fatalf("stream error: %v", err)
	}

	// Turn 1: thinking lifecycle, tool call lifecycle, assistant message, tool result, tool message
	// Turn 2: content part lifecycle, assistant message
	expected := []kit.EventType{
		kit.EventContentPartStarted,
		kit.EventContentPartDelta,
		kit.EventContentPartDone,
		kit.EventContentPartStarted,
		kit.EventContentPartDone,
		kit.EventMessage,
		kit.EventContentPartStarted,
		kit.EventContentPartDone,
		kit.EventMessage,
		kit.EventContentPartStarted,
		kit.EventContentPartDelta,
		kit.EventContentPartDone,
		kit.EventMessage,
	}

	if len(eventTypes) != len(expected) {
		t.Fatalf("event count = %d, want %d: %v", len(eventTypes), len(expected), eventTypes)
	}

	for idx, got := range eventTypes {
		if got != expected[idx] {
			t.Errorf("event[%d] = %q, want %q", idx, got, expected[idx])
		}
	}
}

func TestAgent_Stream_ConsumerStopMidIteration(t *testing.T) {
	model := kittest.NewModel(t,
		kittest.ModelTurn{
			Text: "Hello world",
			Stream: []kittest.ChunkResult{
				{Event: kit.NewContentPartStartedEvent(kit.ContentTypeText)},
				{Event: kit.NewContentPartDeltaEvent(kit.ContentTypeText, "Hello ")},
				{Event: kit.NewContentPartDeltaEvent(kit.ContentTypeText, "world")},
				{Event: kit.NewContentPartDoneEvent(kit.NewTextPart("Hello world"))},
			},
		},
	)

	agent, err := agent.New(model)
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}

	stream, err := agent.Stream(context.Background(), []kit.Message{
		kit.NewUserMessage(kit.NewTextPart("Hi")),
	})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}

	count := 0
	for range stream.Iter() {
		count++

		break
	}

	result, err := stream.Result()
	if err != nil {
		t.Fatalf("Result: %v", err)
	}

	if count != 1 {
		t.Fatalf("event count = %d, want 1", count)
	}

	if len(result.Messages) != 0 {
		t.Fatalf("result messages = %d, want partial empty result", len(result.Messages))
	}
}

func TestAgent_Run_StreamError(t *testing.T) {
	model := kittest.NewModel(t,
		kittest.ModelTurn{Stream: []kittest.ChunkResult{
			{Err: errors.New("stream broke")},
		}},
	)

	agent, err := agent.New(model)
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}

	_, err = agent.Run(context.Background(), []kit.Message{
		kit.NewUserMessage(kit.NewTextPart("Hi")),
	})
	if err == nil {
		t.Fatal("expected error from Run")
	}

	if got := err.Error(); got != "stream broke" {
		t.Errorf("error = %q, want %q", got, "stream broke")
	}
}

func TestAgent_Run_MidStreamError(t *testing.T) {
	model := kittest.NewModel(t,
		kittest.ModelTurn{Stream: []kittest.ChunkResult{
			{Event: kit.NewContentPartStartedEvent(kit.ContentTypeText)},
			{Event: kit.NewContentPartDeltaEvent(kit.ContentTypeText, "partial ")},
			{Err: errors.New("connection lost")},
		}},
	)

	agent, err := agent.New(model)
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}

	_, err = agent.Run(context.Background(), []kit.Message{
		kit.NewUserMessage(kit.NewTextPart("Hi")),
	})
	if err == nil {
		t.Fatal("expected error from Run")
	}

	if got := err.Error(); got != "connection lost" {
		t.Errorf("error = %q, want %q", got, "connection lost")
	}
}

func TestAgent_Run_Hooks(t *testing.T) {
	model := kittest.NewModel(t,
		kittest.ModelTurn{Text: "Hello"},
	)

	var hookOrder []string

	agent, err := agent.New(model,
		agent.WithOnRunStart(func(ctx context.Context, msgs []kit.Message) (context.Context, []kit.Message, error) {
			hookOrder = append(hookOrder, "run_start")

			return ctx, msgs, nil
		}),
		agent.WithOnTurnStart(func(ctx context.Context, msgs []kit.Message) (context.Context, []kit.Message, error) {
			hookOrder = append(hookOrder, "turn_start")

			return ctx, msgs, nil
		}),
		agent.WithOnModelRequest(func(ctx context.Context, req kit.ModelRequest) (context.Context, kit.ModelRequest, error) {
			hookOrder = append(hookOrder, "model_request")

			return ctx, req, nil
		}),
		agent.WithOnModelResponse(func(ctx context.Context, resp kit.ModelResponse) (context.Context, kit.ModelResponse, error) {
			hookOrder = append(hookOrder, "model_response")

			return ctx, resp, nil
		}),
		agent.WithOnRunEnd(func(ctx context.Context, _ kit.Result, _ error) (context.Context, error) {
			hookOrder = append(hookOrder, "run_end")

			return ctx, nil
		}),
	)
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}

	_, err = agent.Run(context.Background(), []kit.Message{
		kit.NewUserMessage(kit.NewTextPart("Hi")),
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	expected := []string{"run_start", "turn_start", "model_request", "model_response", "run_end"}
	if len(hookOrder) != len(expected) {
		t.Fatalf("hook count = %d, want %d: %v", len(hookOrder), len(expected), hookOrder)
	}

	for idx, got := range hookOrder {
		if got != expected[idx] {
			t.Errorf("hook[%d] = %q, want %q", idx, got, expected[idx])
		}
	}
}

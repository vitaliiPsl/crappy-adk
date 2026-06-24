package agent

import (
	"context"
	"errors"
	"testing"

	"github.com/vitaliiPsl/crappy-adk/kit"
	"github.com/vitaliiPsl/crappy-adk/kittest"
	xmemory "github.com/vitaliiPsl/crappy-adk/x/memory"
	xtool "github.com/vitaliiPsl/crappy-adk/x/tool"
)

func TestWithInstructionsAppendsSources(t *testing.T) {
	model := doneModel(t)

	a, err := New(
		model,
		xmemory.NewHistory(),
		xtool.NewSet(),
		WithInstructions("first", "second"),
		WithInstructions("third"),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, err = a.Run(context.Background(), kit.NewUserMessage(kit.NewTextContent("hello")))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	got := model.CallAt(0).Instructions

	want := "first\n\nsecond\n\nthird"
	if got != want {
		t.Fatalf("Instructions = %q, want %q", got, want)
	}
}

func TestWithDynamicInstructionsAppendsResolvedInstructions(t *testing.T) {
	model := doneModel(t)

	a, err := New(
		model,
		xmemory.NewHistory(),
		xtool.NewSet(),
		WithInstructions("static"),
		WithDynamicInstructions(
			func(*kit.RunContext) (string, error) { return " dynamic one ", nil },
			func(*kit.RunContext) (string, error) { return "", nil },
			func(*kit.RunContext) (string, error) { return "dynamic two", nil },
		),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, err = a.Run(context.Background(), kit.NewUserMessage(kit.NewTextContent("hello")))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	got := model.CallAt(0).Instructions

	want := "static\n\ndynamic one\n\ndynamic two"
	if got != want {
		t.Fatalf("Instructions = %q, want %q", got, want)
	}
}

func TestWithDynamicInstructionsReturnsResolverError(t *testing.T) {
	wantErr := errors.New("boom")
	model := kittest.NewModel(t)

	a, err := New(
		model,
		xmemory.NewHistory(),
		xtool.NewSet(),
		WithDynamicInstructions(func(*kit.RunContext) (string, error) {
			return "", wantErr
		}),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, err = a.Run(context.Background(), kit.NewUserMessage(kit.NewTextContent("hello")))
	if !errors.Is(err, wantErr) {
		t.Fatalf("Run error = %v, want %v", err, wantErr)
	}

	model.AssertCallCount(t, 0)
}

func TestGenerationOptionsConfigureModelRequest(t *testing.T) {
	model := doneModel(t)

	a, err := New(
		model,
		xmemory.NewHistory(),
		xtool.NewSet(),
		WithTemperature(0.25),
		WithMaxOutputTokens(2048),
		WithThinking(kit.ThinkingLevelHigh),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, err = a.Run(context.Background(), kit.NewUserMessage(kit.NewTextContent("hello")))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	cfg := model.CallAt(0).Config
	if cfg.Temperature == nil || *cfg.Temperature != 0.25 {
		t.Fatalf("Temperature = %v, want 0.25", cfg.Temperature)
	}

	if cfg.MaxOutputTokens == nil || *cfg.MaxOutputTokens != 2048 {
		t.Fatalf("MaxOutputTokens = %v, want 2048", cfg.MaxOutputTokens)
	}

	if cfg.Thinking == nil || *cfg.Thinking != kit.ThinkingLevelHigh {
		t.Fatalf("Thinking = %v, want high", cfg.Thinking)
	}
}

func TestWithToolsAddsTools(t *testing.T) {
	first := kittest.NewTool(t, "first", "first tool")
	second := kittest.NewTool(t, "second", "second tool")

	a, err := New(doneModel(t), xmemory.NewHistory(), xtool.NewSet(), WithTools(first, second))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	got := a.tools.List()
	if len(got) != 2 || got[0] != first || got[1] != second {
		t.Fatalf("tools = %#v, want first and second", got)
	}
}

func TestHookOptionsRunInOrder(t *testing.T) {
	call := kit.NewToolCall("call-1", "lookup", map[string]any{"query": "original"})
	model := kittest.NewModel(t, kittest.ModelResult{
		Response: kit.ModelResponse{
			Message:      kit.NewModelMessage(kit.NewToolCallContent(call)),
			FinishReason: kit.FinishReasonToolCall,
		},
	}, kittest.ModelResult{
		Response: kit.ModelResponse{
			Message:      kit.NewModelMessage(kit.NewTextContent("done")),
			FinishReason: kit.FinishReasonStop,
		},
	})

	tool := kittest.NewTool(t, "lookup", "lookup tool", kittest.ToolResult{Result: "tool output"})

	var (
		turnStartCount int
		turnEndCount   int
	)

	a, err := New(
		model,
		xmemory.NewHistory(),
		xtool.NewSet(tool),
		WithOnTurnStart(func(*kit.RunContext) error {
			turnStartCount++

			return nil
		}),
		WithOnModelRequest(func(_ *kit.RunContext, req kit.ModelRequest) (kit.ModelRequest, error) {
			req.Instructions = "hooked request"

			return req, nil
		}),
		WithOnModelResponse(func(_ *kit.RunContext, resp kit.ModelResponse) (kit.ModelResponse, error) {
			if resp.FinishReason != kit.FinishReasonToolCall {
				resp.Message.Content = []kit.Content{kit.NewTextContent("hooked response")}
			}

			return resp, nil
		}),
		WithOnToolCall(func(_ *kit.RunContext, call kit.ToolCall) (kit.ToolCall, error) {
			call.Arguments["query"] = "hooked call"

			return call, nil
		}),
		WithOnToolResult(func(_ *kit.RunContext, result kit.ToolResult) (kit.ToolResult, error) {
			result.Output = "hooked result"

			return result, nil
		}),
		WithOnTurnEnd(func(*kit.RunContext) error {
			turnEndCount++

			return nil
		}),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	response, err := a.Run(context.Background(), kit.NewUserMessage(kit.NewTextContent("hello")))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if turnStartCount != 2 || turnEndCount != 2 {
		t.Fatalf("turn hooks = start %d end %d, want 2/2", turnStartCount, turnEndCount)
	}

	if got := model.CallAt(0).Instructions; got != "hooked request" {
		t.Fatalf("first request instructions = %q, want hooked request", got)
	}

	tool.AssertCalledWith(t, 0, map[string]any{"query": "hooked call"})

	secondRequest := model.CallAt(1)
	lastMessage := secondRequest.Messages[len(secondRequest.Messages)-1]

	results := lastMessage.ToolResults()
	if len(results) != 1 || results[0].Output != "hooked result" {
		t.Fatalf("tool results = %+v, want hooked result", results)
	}

	if response.Output.Text != "hooked response" {
		t.Fatalf("Output = %q, want hooked final response", response.Output.Text)
	}
}

func TestWithExtensionsAppliesNestedOptions(t *testing.T) {
	model := doneModel(t)

	a, err := New(
		model,
		xmemory.NewHistory(),
		xtool.NewSet(),
		WithExtensions(
			WithInstructions("nested"),
			WithTemperature(0.5),
		),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, err = a.Run(context.Background(), kit.NewUserMessage(kit.NewTextContent("hello")))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	req := model.CallAt(0)
	if req.Instructions != "nested" {
		t.Fatalf("Instructions = %q, want nested", req.Instructions)
	}

	if req.Config.Temperature == nil || *req.Config.Temperature != 0.5 {
		t.Fatalf("Temperature = %v, want 0.5", req.Config.Temperature)
	}
}

func TestWithExtensionsReturnsNestedError(t *testing.T) {
	wantErr := errors.New("bad option")

	_, err := New(
		doneModel(t),
		xmemory.NewHistory(),
		xtool.NewSet(),
		WithExtensions(func(*Agent) error {
			return wantErr
		}),
	)
	if !errors.Is(err, wantErr) {
		t.Fatalf("New error = %v, want %v", err, wantErr)
	}
}

func doneModel(t testing.TB) *kittest.Model {
	t.Helper()

	return kittest.NewModel(t, kittest.ModelResult{
		Response: kit.ModelResponse{
			Message:      kit.NewModelMessage(kit.NewTextContent("done")),
			FinishReason: kit.FinishReasonStop,
		},
	})
}

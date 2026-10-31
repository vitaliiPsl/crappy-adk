package agent

import (
	"context"
	"errors"
	"testing"

	"github.com/vitaliiPsl/crappy-adk/kit"
	"github.com/vitaliiPsl/crappy-adk/kittest"
	"github.com/vitaliiPsl/crappy-adk/x/history"
)

func TestInstructionOptions_RenderInstructionsBeforeSourcedSections(t *testing.T) {
	model := doneModel(t)
	sources := []string{" first ", "", "second"}

	a, err := New(model, history.New(),
		WithInstructionsSource(" workspace ", sources...),
		WithInstructions(" persona "),
		WithInstructionsSource("style", "third"),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	sources[0] = "changed after registration"

	if _, err := a.Run(context.Background(), kit.NewUserMessage(kit.NewTextContent("hello"))); err != nil {
		t.Fatalf("Run: %v", err)
	}

	want := "persona\n\n# workspace\n\nfirst\n\n# workspace\n\nsecond\n\n# style\n\nthird"
	if got := model.CallAt(0).Instructions; got != want {
		t.Fatalf("Instructions = %q, want %q", got, want)
	}
}

func TestGenerationOptions_ConfigureModelRequest(t *testing.T) {
	model := doneModel(t)

	a, err := New(
		model,
		history.New(),
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

func TestHookOptions_RunInOrder(t *testing.T) {
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
		history.New(),
		WithTools(tool),
		WithOnTurnStart(func(*kit.RunContext) error {
			turnStartCount++

			return nil
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
			result.Output.Content = []kit.Content{kit.NewTextContent("hooked result")}

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

	tool.AssertCalledWith(t, 0, map[string]any{"query": "hooked call"})

	secondRequest := model.CallAt(1)
	lastMessage := secondRequest.Messages[len(secondRequest.Messages)-1]

	results := lastMessage.ToolResults()
	if len(results) != 1 || kit.ContentsText(results[0].Output.Content) != "hooked result" {
		t.Fatalf("tool results = %+v, want hooked result", results)
	}

	if response.Output.Text != "hooked response" {
		t.Fatalf("Output = %q, want hooked final response", response.Output.Text)
	}
}

func TestWithExtensions_AppliesNestedOptions(t *testing.T) {
	model := doneModel(t)

	a, err := New(
		model,
		history.New(),
		WithExtensions(
			WithMaxOutputTokens(64),
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
	if req.Config.MaxOutputTokens == nil || *req.Config.MaxOutputTokens != 64 {
		t.Fatalf("MaxOutputTokens = %v, want 64", req.Config.MaxOutputTokens)
	}

	if req.Config.Temperature == nil || *req.Config.Temperature != 0.5 {
		t.Fatalf("Temperature = %v, want 0.5", req.Config.Temperature)
	}
}

func TestWithExtensions_ReturnsNestedError(t *testing.T) {
	wantErr := errors.New("bad option")

	_, err := New(
		doneModel(t),
		history.New(),
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

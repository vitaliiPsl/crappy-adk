package agent

import (
	"context"
	"reflect"
	"testing"

	"github.com/vitaliiPsl/crappy-adk/kit"
	"github.com/vitaliiPsl/crappy-adk/kittest"
	"github.com/vitaliiPsl/crappy-adk/x/stream"
)

func TestModelRunner_Run_AppliesHooksAndForwardsEvents(t *testing.T) {
	model := kittest.NewModel(t, kittest.ModelTurn{
		Text:     "original text",
		Thinking: "thinking",
		Usage:    kit.Usage{InputTokens: 11, OutputTokens: 7},
	})
	tool := kittest.NewTool(t, "search", "Search the web")

	runner := modelRunner{
		model: model,
		toolDefinitions: []kit.ToolDefinition{
			tool.Definition(),
		},
		config: &Config{
			Thinking: kit.ThinkingLevelHigh,
		},
		hooks: &hooks{
			modelRequest: []kit.OnModelRequest{
				func(ctx context.Context, req kit.ModelRequest) (context.Context, kit.ModelRequest, error) {
					req.Instruction += "\n\nextra instruction"

					return ctx, req, nil
				},
			},
			modelResponse: []kit.OnModelResponse{
				func(ctx context.Context, resp kit.ModelResponse) (context.Context, kit.ModelResponse, error) {
					resp.Message = kit.NewAssistantMessage("hooked text", resp.Message.Thinking(), resp.Message.ToolCalls())
					resp.Usage.OutputTokens = 9

					return ctx, resp, nil
				},
			},
		},
	}

	var gotEvents []kit.Event

	resp, err := runner.run(context.Background(), "base instruction", []kit.Message{
		kit.NewUserMessage(kit.NewTextPart("hello")),
	}, stream.NewEmitter[kit.Event](func(event kit.Event) bool {
		gotEvents = append(gotEvents, event)

		return true
	}))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	call := model.CallAt(0)
	if call.Instruction != "base instruction\n\nextra instruction" {
		t.Fatalf("instruction = %q", call.Instruction)
	}

	if !reflect.DeepEqual(call.Tools, []kit.ToolDefinition{tool.Definition()}) {
		t.Fatalf("tools = %#v, want %#v", call.Tools, []kit.ToolDefinition{tool.Definition()})
	}

	if call.Config.Thinking != kit.ThinkingLevelHigh {
		t.Fatalf("thinking = %q, want %q", call.Config.Thinking, kit.ThinkingLevelHigh)
	}

	if got := resp.Message.Text(); got != "hooked text" {
		t.Fatalf("message text = %q, want %q", got, "hooked text")
	}

	if resp.Usage.OutputTokens != 9 {
		t.Fatalf("output tokens = %d, want %d", resp.Usage.OutputTokens, 9)
	}

	wantEvents := []kit.Event{
		kit.NewContentPartStartedEvent(kit.ContentTypeThinking),
		kit.NewContentPartDeltaEvent(kit.ContentTypeThinking, "thinking"),
		kit.NewContentPartDoneEvent(kit.NewThinkingPart("thinking", "")),
		kit.NewContentPartStartedEvent(kit.ContentTypeText),
		kit.NewContentPartDeltaEvent(kit.ContentTypeText, "original text"),
		kit.NewContentPartDoneEvent(kit.NewTextPart("original text")),
	}

	if !reflect.DeepEqual(gotEvents, wantEvents) {
		t.Fatalf("events = %#v, want %#v", gotEvents, wantEvents)
	}
}

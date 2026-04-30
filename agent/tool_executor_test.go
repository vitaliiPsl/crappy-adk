package agent

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/vitaliiPsl/crappy-adk/kit"
	"github.com/vitaliiPsl/crappy-adk/kittest"
	"github.com/vitaliiPsl/crappy-adk/x/stream"
)

func TestToolExecutor_Run_AppliesHooksAndYieldsMessages(t *testing.T) {
	readTool := kittest.NewTool(t, "read", "Read file",
		kittest.ToolResponse{Result: "contents"},
	)

	registry := newToolRegistry()
	registry.register(readTool)

	runner := toolExecutor{
		registry: registry,
		hooks: &hooks{
			toolCall: []kit.OnToolCall{
				func(ctx context.Context, call kit.ToolCall) (context.Context, kit.ToolCall, error) {
					call.Arguments["path"] = strings.ToUpper(call.Arguments["path"].(string))

					return ctx, call, nil
				},
			},
			toolResult: []kit.OnToolResult{
				func(ctx context.Context, result kit.ToolResult) (context.Context, kit.ToolResult, error) {
					result.Content += " [hooked]"

					return ctx, result, nil
				},
			},
		},
	}

	var gotEvents []kit.Event

	msgs, err := runner.run(context.Background(), []kit.ToolCall{
		{ID: "call-1", Name: "read", Arguments: map[string]any{"path": "main.go"}},
	}, stream.NewEmitter(func(event kit.Event) bool {
		gotEvents = append(gotEvents, event)

		return true
	}))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	readTool.AssertCalledWith(t, 0, map[string]any{"path": "MAIN.GO"})

	wantMsgs := []kit.Message{
		kit.NewToolMessage("contents [hooked]", kit.ToolCall{
			ID:        "call-1",
			Name:      "read",
			Arguments: map[string]any{"path": "MAIN.GO"},
		}),
	}
	if !reflect.DeepEqual(msgs, wantMsgs) {
		t.Fatalf("messages = %#v, want %#v", msgs, wantMsgs)
	}

	wantEvents := []kit.Event{
		kit.NewContentPartStartedEvent(kit.ContentTypeToolResult),
		kit.NewContentPartDoneEvent(kit.NewToolResultPart("contents [hooked]", kit.ToolCall{
			ID:        "call-1",
			Name:      "read",
			Arguments: map[string]any{"path": "MAIN.GO"},
		})),
		kit.NewMessageEvent(wantMsgs[0]),
	}
	if !reflect.DeepEqual(gotEvents, wantEvents) {
		t.Fatalf("events = %#v, want %#v", gotEvents, wantEvents)
	}
}

func TestToolExecutor_RunMultipleCalls_YieldsMessagesInOrder(t *testing.T) {
	searchTool := kittest.NewTool(t, "search", "Search",
		kittest.ToolResponse{Result: "result A"},
	)
	readTool := kittest.NewTool(t, "read", "Read",
		kittest.ToolResponse{Result: "result B"},
	)

	registry := newToolRegistry()
	registry.register(searchTool)
	registry.register(readTool)

	runner := toolExecutor{
		registry: registry,
		hooks:    &hooks{},
	}

	var gotEvents []kit.Event

	msgs, err := runner.run(context.Background(), []kit.ToolCall{
		{ID: "call-1", Name: "search", Arguments: map[string]any{"q": "A"}},
		{ID: "call-2", Name: "read", Arguments: map[string]any{"path": "b.txt"}},
	}, stream.NewEmitter(func(event kit.Event) bool {
		gotEvents = append(gotEvents, event)

		return true
	}))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	searchTool.AssertCalledWith(t, 0, map[string]any{"q": "A"})
	readTool.AssertCalledWith(t, 0, map[string]any{"path": "b.txt"})

	if len(msgs) != 2 {
		t.Fatalf("message count = %d, want %d", len(msgs), 2)
	}

	part0, ok := msgs[0].ToolResult()
	if !ok {
		t.Fatal("expected first tool result part")
	}

	part1, ok := msgs[1].ToolResult()
	if !ok {
		t.Fatal("expected second tool result part")
	}

	if part0.ID != "call-1" {
		t.Fatalf("msgs[0] tool call id = %q, want %q", part0.ID, "call-1")
	}

	if part1.ID != "call-2" {
		t.Fatalf("msgs[1] tool call id = %q, want %q", part1.ID, "call-2")
	}

	if len(gotEvents) != 6 {
		t.Fatalf("event count = %d, want %d", len(gotEvents), 6)
	}
}

func TestToolExecutor_Call_RecoversPanic(t *testing.T) {
	registry := newToolRegistry()
	registry.register(panicTool{})

	runner := toolExecutor{
		registry: registry,
		hooks:    &hooks{},
	}

	result := runner.call(context.Background(), kit.ToolCall{
		ID:   "call-1",
		Name: "panic",
	})

	if result.Call.Name != "panic" {
		t.Fatalf("call name = %q, want %q", result.Call.Name, "panic")
	}

	if !strings.Contains(result.Error, "recovered: suprize mfch") {
		t.Fatalf("error = %q, want panic recovery message", result.Error)
	}
}

type panicTool struct{}

func (panicTool) Definition() kit.ToolDefinition {
	return kit.ToolDefinition{Name: "panic", Description: "PanicAaaaa"}
}

func (panicTool) Execute(context.Context, map[string]any) (string, error) {
	panic("suprize mfch")
}

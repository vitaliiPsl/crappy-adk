package agent

import (
	"context"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/vitaliiPsl/crappy-adk/kit"
	"github.com/vitaliiPsl/crappy-adk/kittest"
)

func TestToolRunner_RunSequential_AppliesHooksAndYieldsMessages(t *testing.T) {
	readTool := kittest.NewTool(t, "read", "Read file",
		kittest.ToolResponse{Result: "contents"},
	)

	runner := toolRunner{
		tools: map[string]kit.Tool{
			"read": readTool,
		},
		config: &Config{ToolExecution: ToolExecutionSequential},
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

	msgs, ok := runner.run(context.Background(), []kit.ToolCall{
		{ID: "call-1", Name: "read", Arguments: map[string]any{"path": "main.go"}},
	}, func(event kit.Event, err error) bool {
		if err != nil {
			t.Fatalf("unexpected yield error: %v", err)
		}

		gotEvents = append(gotEvents, event)

		return true
	})
	if !ok {
		t.Fatal("run returned ok=false")
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
		kit.NewToolResultEvent(kit.ToolResult{
			Call: kit.ToolCall{
				ID:        "call-1",
				Name:      "read",
				Arguments: map[string]any{"path": "MAIN.GO"},
			},
			Content: "contents [hooked]",
		}),
		kit.NewMessageEvent(wantMsgs[0]),
	}
	if !reflect.DeepEqual(gotEvents, wantEvents) {
		t.Fatalf("events = %#v, want %#v", gotEvents, wantEvents)
	}
}

func TestToolRunner_RunParallel_YieldsAllResults(t *testing.T) {
	searchTool := kittest.NewTool(t, "search", "Search",
		kittest.ToolResponse{Result: "result A"},
	)
	readTool := kittest.NewTool(t, "read", "Read",
		kittest.ToolResponse{Result: "result B"},
	)

	runner := toolRunner{
		tools: map[string]kit.Tool{
			"search": searchTool,
			"read":   readTool,
		},
		config: &Config{ToolExecution: ToolExecutionParallel},
		hooks:  &hooks{},
	}

	var gotEvents []kit.Event

	msgs, ok := runner.run(context.Background(), []kit.ToolCall{
		{ID: "call-1", Name: "search", Arguments: map[string]any{"q": "A"}},
		{ID: "call-2", Name: "read", Arguments: map[string]any{"path": "b.txt"}},
	}, func(event kit.Event, err error) bool {
		if err != nil {
			t.Fatalf("unexpected yield error: %v", err)
		}

		gotEvents = append(gotEvents, event)

		return true
	})
	if !ok {
		t.Fatal("run returned ok=false")
	}

	searchTool.AssertCalledWith(t, 0, map[string]any{"q": "A"})
	readTool.AssertCalledWith(t, 0, map[string]any{"path": "b.txt"})

	if len(msgs) != 2 {
		t.Fatalf("message count = %d, want %d", len(msgs), 2)
	}

	gotMsgIDs := []string{msgs[0].ToolCallID, msgs[1].ToolCallID}
	slices.Sort(gotMsgIDs)

	if !reflect.DeepEqual(gotMsgIDs, []string{"call-1", "call-2"}) {
		t.Fatalf("tool call ids = %#v, want %#v", gotMsgIDs, []string{"call-1", "call-2"})
	}

	if len(gotEvents) != 4 {
		t.Fatalf("event count = %d, want %d", len(gotEvents), 4)
	}

	var (
		resultIDs  []string
		messageIDs []string
	)

	for _, event := range gotEvents {
		switch event.Type {
		case kit.EventToolResult:
			resultIDs = append(resultIDs, event.ToolResult.Call.ID)
		case kit.EventMessage:
			messageIDs = append(messageIDs, event.Message.ToolCallID)
		default:
			t.Fatalf("unexpected event type: %v", event.Type)
		}
	}

	slices.Sort(resultIDs)
	slices.Sort(messageIDs)

	if !reflect.DeepEqual(resultIDs, []string{"call-1", "call-2"}) {
		t.Fatalf("tool result ids = %#v, want %#v", resultIDs, []string{"call-1", "call-2"})
	}

	if !reflect.DeepEqual(messageIDs, []string{"call-1", "call-2"}) {
		t.Fatalf("message ids = %#v, want %#v", messageIDs, []string{"call-1", "call-2"})
	}
}

func TestToolRunner_Call_RecoversPanic(t *testing.T) {
	runner := toolRunner{
		tools: map[string]kit.Tool{
			"panic": panicTool{},
		},
		hooks:  &hooks{},
		config: &Config{},
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

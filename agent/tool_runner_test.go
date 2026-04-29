package agent

import (
	"context"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/vitaliiPsl/crappy-adk/kit"
	"github.com/vitaliiPsl/crappy-adk/kittest"
	"github.com/vitaliiPsl/crappy-adk/x/stream"
)

func TestToolRunner_RunSequential_AppliesHooksAndYieldsMessages(t *testing.T) {
	readTool := kittest.NewTool(t, "read", "Read file",
		kittest.ToolResponse{Result: "contents"},
	)

	runner := toolRunner{
		tools: map[string]kit.Tool{
			"read": readTool,
		},
		config: &kit.AgentConfig{ToolExecution: kit.ToolExecutionSequential},
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
	}, stream.NewEmitter[kit.Event](func(event kit.Event) bool {
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
		kit.NewContentPartDeltaEvent(kit.ContentTypeToolResult, "contents [hooked]"),
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
		config: &kit.AgentConfig{ToolExecution: kit.ToolExecutionParallel},
		hooks:  &hooks{},
	}

	var gotEvents []kit.Event

	msgs, err := runner.run(context.Background(), []kit.ToolCall{
		{ID: "call-1", Name: "search", Arguments: map[string]any{"q": "A"}},
		{ID: "call-2", Name: "read", Arguments: map[string]any{"path": "b.txt"}},
	}, stream.NewEmitter[kit.Event](func(event kit.Event) bool {
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

	gotMsgIDs := []string{part0.ID, part1.ID}
	slices.Sort(gotMsgIDs)

	if !reflect.DeepEqual(gotMsgIDs, []string{"call-1", "call-2"}) {
		t.Fatalf("tool call ids = %#v, want %#v", gotMsgIDs, []string{"call-1", "call-2"})
	}

	if len(gotEvents) != 8 {
		t.Fatalf("event count = %d, want %d", len(gotEvents), 8)
	}

	var (
		resultIDs  []string
		messageIDs []string
	)

	for _, event := range gotEvents {
		switch event.Type {
		case kit.EventContentPartStarted, kit.EventContentPartDelta:
			continue
		case kit.EventContentPartDone:
			if event.ContentPart == nil || event.ContentPart.Type != kit.ContentTypeToolResult {
				continue
			}

			resultIDs = append(resultIDs, event.ContentPart.ID)
		case kit.EventMessage:
			part, ok := event.Message.ToolResult()
			if !ok {
				t.Fatal("expected tool result content part on message event")
			}

			messageIDs = append(messageIDs, part.ID)
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

func TestToolRunner_RunParallel_ReturnsMessagesInToolCallOrder(t *testing.T) {
	searchStarted := make(chan struct{})
	releaseSearch := make(chan struct{})

	runner := toolRunner{
		tools: map[string]kit.Tool{
			"search": blockingTool{
				result:  "result A",
				started: searchStarted,
				release: releaseSearch,
			},
			"read": blockingTool{
				result: "result B",
			},
		},
		config: &kit.AgentConfig{ToolExecution: kit.ToolExecutionParallel},
		hooks:  &hooks{},
	}

	msgs, err := runner.run(context.Background(), []kit.ToolCall{
		{ID: "call-1", Name: "search", Arguments: map[string]any{"q": "A"}},
		{ID: "call-2", Name: "read", Arguments: map[string]any{"path": "b.txt"}},
	}, stream.NewEmitter[kit.Event](func(event kit.Event) bool {
		if event.Type == kit.EventMessage {
			part, ok := event.Message.ToolResult()
			if !ok {
				t.Fatal("expected tool result content part on message event")
			}

			if part.ID == "call-2" {
				close(releaseSearch)
			}
		}

		return true
	}))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	select {
	case <-searchStarted:
	default:
		t.Fatal("expected slow search tool to start")
	}

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
}

func TestToolRunner_RunParallel_ConsumerStopCancelsBlockingTools(t *testing.T) {
	slowStarted := make(chan struct{})
	slowCanceled := make(chan struct{})
	releaseFast := make(chan struct{})
	close(releaseFast)

	runner := toolRunner{
		tools: map[string]kit.Tool{
			"fast": blockingTool{
				result:  "result A",
				release: releaseFast,
			},
			"slow": blockingTool{
				result:   "result B",
				started:  slowStarted,
				release:  make(chan struct{}),
				canceled: slowCanceled,
			},
		},
		config: &kit.AgentConfig{ToolExecution: kit.ToolExecutionParallel},
		hooks:  &hooks{},
	}

	_, err := runner.run(context.Background(), []kit.ToolCall{
		{ID: "call-1", Name: "fast", Arguments: map[string]any{}},
		{ID: "call-2", Name: "slow", Arguments: map[string]any{}},
	}, stream.NewEmitter[kit.Event](func(event kit.Event) bool {
		return event.Type != kit.EventMessage
	}))
	if !stream.IsStopped(err) {
		t.Fatalf("run error = %v, want stream stopped", err)
	}

	select {
	case <-slowStarted:
	case <-time.After(time.Second):
		t.Fatal("expected slow tool to start")
	}

	select {
	case <-slowCanceled:
	case <-time.After(time.Second):
		t.Fatal("expected slow tool to observe context cancellation")
	}
}

func TestToolRunner_Call_RecoversPanic(t *testing.T) {
	runner := toolRunner{
		tools: map[string]kit.Tool{
			"panic": panicTool{},
		},
		hooks:  &hooks{},
		config: &kit.AgentConfig{},
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

type blockingTool struct {
	result   string
	started  chan struct{}
	release  chan struct{}
	canceled chan struct{}
}

func (t blockingTool) Definition() kit.ToolDefinition {
	return kit.ToolDefinition{Name: "blocking", Description: "Block until released"}
}

func (t blockingTool) Execute(ctx context.Context, _ map[string]any) (string, error) {
	if t.started != nil {
		close(t.started)
	}

	if t.release != nil {
		select {
		case <-t.release:
		case <-ctx.Done():
			if t.canceled != nil {
				close(t.canceled)
			}

			return "", ctx.Err()
		}
	}

	return t.result, nil
}

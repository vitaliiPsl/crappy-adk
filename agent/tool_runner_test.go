package agent

import (
	"context"
	"errors"
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
		config: &Config{ToolExecution: ToolExecutionParallel},
		hooks:  &hooks{},
	}

	msgs, ok := runner.run(context.Background(), []kit.ToolCall{
		{ID: "call-1", Name: "search", Arguments: map[string]any{"q": "A"}},
		{ID: "call-2", Name: "read", Arguments: map[string]any{"path": "b.txt"}},
	}, func(event kit.Event, err error) bool {
		if err != nil {
			t.Fatalf("unexpected yield error: %v", err)
		}

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
	})
	if !ok {
		t.Fatal("run returned ok=false")
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

func TestToolRunner_Run_LoopDetected_Sequential(t *testing.T) {
	searchTool := kittest.NewTool(t, "search", "Search",
		kittest.ToolResponse{Result: "result"},
		kittest.ToolResponse{Result: "result"},
		kittest.ToolResponse{Result: "result"},
	)

	runner := toolRunner{
		tools:  map[string]kit.Tool{"search": searchTool},
		config: &Config{ToolExecution: ToolExecutionSequential, ToolLoopMaxRepetitions: 2, ToolLoopWindow: 10},
		hooks:  &hooks{},
	}

	call := kit.ToolCall{ID: "c", Name: "search", Arguments: map[string]any{"q": "x"}}

	for i := range 2 {
		_, ok := runner.run(context.Background(), []kit.ToolCall{call}, func(_ kit.Event, err error) bool {
			if err != nil {
				t.Fatalf("turn %d: unexpected error: %v", i+1, err)
			}

			return true
		})
		if !ok {
			t.Fatalf("turn %d: run returned ok=false, want true", i+1)
		}
	}

	var gotErr error

	_, ok := runner.run(context.Background(), []kit.ToolCall{call}, func(_ kit.Event, err error) bool {
		if err != nil {
			gotErr = err
		}

		return true
	})

	if ok {
		t.Fatal("third run returned ok=true, want false")
	}

	if !errors.Is(gotErr, kit.ErrToolLoop) {
		t.Errorf("error = %v, want kit.ErrToolLoop", gotErr)
	}

	searchTool.AssertCallCount(t, 2)
}

func TestToolRunner_Run_LoopDetected_Parallel(t *testing.T) {
	searchTool := kittest.NewTool(t, "search", "Search",
		kittest.ToolResponse{Result: "result"},
		kittest.ToolResponse{Result: "result"},
		kittest.ToolResponse{Result: "result"},
	)

	runner := toolRunner{
		tools:  map[string]kit.Tool{"search": searchTool},
		config: &Config{ToolExecution: ToolExecutionParallel, ToolLoopMaxRepetitions: 2, ToolLoopWindow: 10},
		hooks:  &hooks{},
	}

	call := kit.ToolCall{ID: "c", Name: "search", Arguments: map[string]any{"q": "x"}}

	for i := range 2 {
		_, ok := runner.run(context.Background(), []kit.ToolCall{call}, func(_ kit.Event, err error) bool {
			if err != nil {
				t.Fatalf("turn %d: unexpected error: %v", i+1, err)
			}

			return true
		})
		if !ok {
			t.Fatalf("turn %d: run returned ok=false, want true", i+1)
		}
	}

	var gotErr error

	_, ok := runner.run(context.Background(), []kit.ToolCall{call}, func(_ kit.Event, err error) bool {
		if err != nil {
			gotErr = err
		}

		return true
	})

	if ok {
		t.Fatal("third run returned ok=true, want false")
	}

	if !errors.Is(gotErr, kit.ErrToolLoop) {
		t.Errorf("error = %v, want kit.ErrToolLoop", gotErr)
	}

	searchTool.AssertCallCount(t, 2)
}

func TestToolRunner_Run_WindowEvictsOldCalls(t *testing.T) {
	// Window=3, max=2: A A B A — after the interleaving B, the oldest A falls
	// out of the window so the 4th call (A again) should pass where a global
	// counter would have blocked it.
	searchTool := kittest.NewTool(t, "search", "Search",
		kittest.ToolResponse{Result: "r"},
		kittest.ToolResponse{Result: "r"},
		kittest.ToolResponse{Result: "r"},
		kittest.ToolResponse{Result: "r"},
	)

	runner := toolRunner{
		tools:  map[string]kit.Tool{"search": searchTool},
		config: &Config{ToolExecution: ToolExecutionSequential, ToolLoopMaxRepetitions: 2, ToolLoopWindow: 3},
		hooks:  &hooks{},
	}

	callA := kit.ToolCall{ID: "a", Name: "search", Arguments: map[string]any{"q": "a"}}
	callB := kit.ToolCall{ID: "b", Name: "search", Arguments: map[string]any{"q": "b"}}

	// A, A → window=[A,A], count(A)=2 ≤ 2
	// B    → window=[A,A,B], count(B)=1 ≤ 2
	// A    → window=[A,B,A] (oldest A evicted), count(A)=2 ≤ 2 — passes
	for i, call := range []kit.ToolCall{callA, callA, callB, callA} {
		_, ok := runner.run(context.Background(), []kit.ToolCall{call}, func(_ kit.Event, err error) bool {
			if err != nil {
				t.Fatalf("call %d: unexpected error: %v", i+1, err)
			}

			return true
		})
		if !ok {
			t.Fatalf("call %d: run returned ok=false, want true", i+1)
		}
	}

	searchTool.AssertCallCount(t, 4)
}

func TestToolRunner_Run_DifferentArgsDontTriggerLoop(t *testing.T) {
	searchTool := kittest.NewTool(t, "search", "Search",
		kittest.ToolResponse{Result: "a"},
		kittest.ToolResponse{Result: "b"},
		kittest.ToolResponse{Result: "c"},
	)

	runner := toolRunner{
		tools:  map[string]kit.Tool{"search": searchTool},
		config: &Config{ToolExecution: ToolExecutionSequential, ToolLoopMaxRepetitions: 1, ToolLoopWindow: 10},
		hooks:  &hooks{},
	}

	calls := []kit.ToolCall{
		{ID: "c1", Name: "search", Arguments: map[string]any{"q": "a"}},
		{ID: "c2", Name: "search", Arguments: map[string]any{"q": "b"}},
		{ID: "c3", Name: "search", Arguments: map[string]any{"q": "c"}},
	}

	for i, call := range calls {
		_, ok := runner.run(context.Background(), []kit.ToolCall{call}, func(_ kit.Event, err error) bool {
			if err != nil {
				t.Fatalf("call %d: unexpected error: %v", i+1, err)
			}

			return true
		})
		if !ok {
			t.Fatalf("call %d: run returned ok=false, want true", i+1)
		}
	}

	searchTool.AssertCallCount(t, 3)
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

type blockingTool struct {
	result  string
	started chan struct{}
	release chan struct{}
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
			return "", ctx.Err()
		}
	}

	return t.result, nil
}

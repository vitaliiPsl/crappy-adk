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
	"github.com/vitaliiPsl/crappy-adk/x/history"
)

func TestNew_RejectsDuplicateToolNames(t *testing.T) {
	first := kittest.NewTool(t, "lookup", "first")
	second := kittest.NewTool(t, "lookup", "second")

	if _, err := New(nil, history.New(), WithTools(first), WithTools(second)); err == nil || !strings.Contains(err.Error(), `"lookup"`) {
		t.Fatalf("New error = %v, want duplicate tool error", err)
	}
}

func TestContext_ReturnsDefinitionAndHistoryWithoutCallingModel(t *testing.T) {
	ctx := context.Background()
	model := kittest.NewModel(t)
	tool := kittest.NewTool(t, "lookup", "Lookup information")
	message := kit.NewUserMessage(kit.NewTextContent("hello"))

	a, err := New(model, newTestHistory(message), WithInstructions("be brief"), WithTools(tool))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	agentContext, err := a.Context(ctx)
	if err != nil {
		t.Fatalf("Context: %v", err)
	}

	if agentContext.InstructionsText() != "be brief" ||
		!reflect.DeepEqual(agentContext.Tools, []kit.Tool{tool}) ||
		!reflect.DeepEqual(agentContext.Messages, []kit.Message{message}) {
		t.Fatalf("Context() = %+v", agentContext)
	}

	agentContext.Instructions[0].Text = "changed"
	agentContext.Tools[0] = nil

	reread, err := a.Context(ctx)
	if err != nil {
		t.Fatalf("Context: %v", err)
	}

	if reread.Instructions[0].Text != "be brief" || reread.Tools[0] == nil {
		t.Fatalf("Context exposed the agent definition: %+v", reread)
	}

	model.AssertNeverCalled(t)
}

func TestRun_BuildsRequestFromDefinitionAndHistory(t *testing.T) {
	prior := kit.NewUserMessage(kit.NewTextContent("prior work"))
	input := kit.NewUserMessage(kit.NewTextContent("continue"))
	summary := kit.NewUserMessage(kit.NewSummaryContent("summary"))
	model := doneModel(t)
	tool := kittest.NewTool(t, "lookup", "Lookup information")
	reads := 0

	h := newTestHistory(prior)
	h.read = func(messages []kit.Message) ([]kit.Message, error) {
		reads++

		return append([]kit.Message{summary}, messages...), nil
	}

	a, err := New(model, h,
		WithInstructions("stable guidance"),
		WithInstructionsSource("task", "dynamic guidance"),
		WithTools(tool),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	response, err := a.Run(context.Background(), input)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	request := model.CallAt(0)
	if request.Instructions != "stable guidance\n\n# task\n\ndynamic guidance" ||
		!reflect.DeepEqual(request.Tools, []kit.Tool{tool}) ||
		!reflect.DeepEqual(request.Messages, []kit.Message{summary, prior, input}) {
		t.Fatalf("request = %+v", request)
	}

	if reads != 1 {
		t.Fatalf("reads = %d, want 1", reads)
	}

	if want := append([]kit.Message{prior, input}, response.Messages...); !reflect.DeepEqual(h.messages, want) {
		t.Fatalf("history messages = %+v, want prior, input and response without the summary", h.messages)
	}
}

func TestRun_ContinuesFromHistoryWithoutInput(t *testing.T) {
	for _, mode := range []string{"run", "stream"} {
		t.Run(mode, func(t *testing.T) {
			prior := kit.NewUserMessage(kit.NewTextContent("unfinished work"))
			h := newTestHistory(prior)
			model := doneModel(t)

			a, err := New(model, h)
			if err != nil {
				t.Fatal(err)
			}

			if mode == "stream" {
				_, err = a.Stream(context.Background()).Result()
			} else {
				_, err = a.Run(context.Background())
			}

			if err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(model.CallAt(0).Messages, []kit.Message{prior}) {
				t.Fatalf("continued request = %+v", model.CallAt(0).Messages)
			}

			if len(h.messages) != 2 || !reflect.DeepEqual(h.messages[0], prior) {
				t.Fatalf("continued history = %+v", h.messages)
			}
		})
	}
}

func TestRun_HistoryReadFailureStopsRun(t *testing.T) {
	failure := errors.New("cannot read history")
	model := kittest.NewModel(t)

	h := newTestHistory()
	h.read = func([]kit.Message) ([]kit.Message, error) {
		return nil, failure
	}

	a, err := New(model, h)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if _, err := a.Run(context.Background(), kit.NewUserMessage(kit.NewTextContent("hello"))); !errors.Is(err, failure) {
		t.Fatalf("Run error = %v, want read failure", err)
	}

	model.AssertNeverCalled(t)
}

func TestRun_ReturnsFinalResponse(t *testing.T) {
	tool := kittest.NewTool(t, "unused", "unused tool")

	model := kittest.NewModel(t, kittest.ModelResult{
		Response: kit.ModelResponse{
			Message:      kit.NewModelMessage(kit.NewTextContent("done")),
			FinishReason: kit.FinishReasonStop,
			Usage:        kit.Usage{InputTokens: 3, OutputTokens: 2},
		},
	})

	messages := []kit.Message{
		kit.NewUserMessage(kit.NewTextContent("hello")),
	}

	a, err := New(model, history.New(), WithInstructions("be brief"), WithTools(tool))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	response, err := a.Run(context.Background(), messages[0])
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if response.Output.Text != "done" {
		t.Fatalf("Output = %q, want done", response.Output.Text)
	}

	if response.Usage.InputTokens != 3 || response.Usage.OutputTokens != 2 {
		t.Fatalf("Usage = %+v, want input 3 output 2", response.Usage)
	}

	if response.FinishReason != kit.FinishReasonStop {
		t.Fatalf("FinishReason = %q, want %q", response.FinishReason, kit.FinishReasonStop)
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

func TestRun_RejectsToolCallFinishWithoutCalls(t *testing.T) {
	model := kittest.NewModel(t, kittest.ModelResult{
		Response: kit.ModelResponse{
			Message:      kit.NewModelMessage(kit.NewTextContent("working on it")),
			FinishReason: kit.FinishReasonToolCall,
		},
	})

	a, err := New(model, history.New())
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, err = a.Run(
		context.Background(),
		kit.NewUserMessage(kit.NewTextContent("hi")),
	)
	if !errors.Is(err, kit.ErrInvalidOutput) {
		t.Fatalf("Run error = %v, want %v", err, kit.ErrInvalidOutput)
	}

	model.AssertCallCount(t, 1)
}

func TestRun_ReturnsCanceledContextBeforeModelCall(t *testing.T) {
	model := kittest.NewModel(t)

	input := kit.NewUserMessage(kit.NewTextContent("hello"))

	a, err := New(model, history.New(), WithInstructions("be brief"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	resp, err := a.Run(ctx, input)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run error = %v, want context canceled", err)
	}

	if len(resp.Messages) != 0 {
		t.Fatalf("len(Messages) = %d, want 0", len(resp.Messages))
	}

	model.AssertNeverCalled(t)
}

func TestRun_ReturnsModelError(t *testing.T) {
	wantErr := errors.New("model failed")

	model := kittest.NewModel(t, kittest.ModelResult{Error: wantErr})

	input := kit.NewUserMessage(kit.NewTextContent("hello"))

	a, err := New(model, history.New(), WithInstructions("be brief"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, err = a.Run(context.Background(), input)
	if !errors.Is(err, wantErr) {
		t.Fatalf("Run error = %v, want %v", err, wantErr)
	}

	model.AssertCallCount(t, 1)
}

func TestRun_ModelErrorHookRebuildsRequest(t *testing.T) {
	model := kittest.NewModel(t,
		kittest.ModelResult{Error: kit.ErrContextLength},
		kittest.ModelResult{Response: kit.ModelResponse{
			Message:      kit.NewModelMessage(kit.NewTextContent("done")),
			FinishReason: kit.FinishReasonStop,
		}},
	)
	compacted := kit.NewUserMessage(kit.NewSummaryContent("compacted"))

	var hookErr error

	h := newTestHistory()

	a, err := New(model, h, WithOnModelError(
		func(_ *kit.RunContext, _ kit.ModelRequest, err error) error {
			hookErr = err

			h.replace(compacted)

			return nil
		},
	))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if _, err := a.Run(context.Background(), kit.NewUserMessage(kit.NewTextContent("hello"))); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if !errors.Is(hookErr, kit.ErrContextLength) {
		t.Fatalf("hook error = %v, want ErrContextLength", hookErr)
	}

	model.AssertCallCount(t, 2)

	if got := kit.ContentsText(model.CallAt(1).Messages[0].Content); got != "compacted" {
		t.Fatalf("retried request message = %q, want compacted", got)
	}
}

func TestRun_ModelErrorHookRetriesOnce(t *testing.T) {
	retryErr := errors.New("retry failed")
	model := kittest.NewModel(t,
		kittest.ModelResult{Error: kit.ErrContextLength},
		kittest.ModelResult{Error: retryErr},
	)
	calls := 0

	a, err := New(model, history.New(), WithOnModelError(
		func(_ *kit.RunContext, _ kit.ModelRequest, _ error) error {
			calls++

			return nil
		},
	))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, err = a.Run(context.Background(), kit.NewUserMessage(kit.NewTextContent("hello")))
	if !errors.Is(err, retryErr) {
		t.Fatalf("Run error = %v, want %v", err, retryErr)
	}

	if calls != 1 {
		t.Fatalf("hook calls = %d, want 1", calls)
	}

	model.AssertCallCount(t, 2)
}

func TestRun_ModelErrorHooksPassErrorsAlong(t *testing.T) {
	wrapped := errors.New("wrapped")
	model := kittest.NewModel(t, kittest.ModelResult{Error: kit.ErrContextLength})

	var received error

	a, err := New(model, history.New(),
		WithOnModelError(func(_ *kit.RunContext, _ kit.ModelRequest, err error) error {
			return errors.Join(wrapped, err)
		}),
		WithOnModelError(func(_ *kit.RunContext, _ kit.ModelRequest, err error) error {
			received = err

			return err
		}),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, err = a.Run(context.Background(), kit.NewUserMessage(kit.NewTextContent("hello")))
	if !errors.Is(err, wrapped) || !errors.Is(err, kit.ErrContextLength) {
		t.Fatalf("Run error = %v, want wrapped ErrContextLength", err)
	}

	if !errors.Is(received, wrapped) {
		t.Fatalf("second hook received %v, want error from first hook", received)
	}

	model.AssertCallCount(t, 1)
}

func TestRun_RequestReadyRebuildsBeforeNextHook(t *testing.T) {
	model := doneModel(t)
	builds, readyCalls := 0, 0
	prefix := kit.NewUserMessage(kit.NewTextContent("transient"))
	compacted := kit.NewUserMessage(kit.NewSummaryContent("compacted"))

	h := newTestHistory()
	h.read = func(messages []kit.Message) ([]kit.Message, error) {
		builds++

		return append([]kit.Message{prefix}, messages...), nil
	}

	a, err := New(model, h,
		WithInstructions("static"),
		WithOnModelRequestReady(func(_ *kit.RunContext, req kit.ModelRequest) (bool, error) {
			readyCalls++

			if req.Instructions != "static" || len(req.Messages) != 2 {
				t.Fatalf("ready hook received an unconstructed request: %+v", req)
			}

			h.replace(compacted)

			return true, nil
		}),
		WithOnModelRequestReady(func(_ *kit.RunContext, req kit.ModelRequest) (bool, error) {
			readyCalls++

			if len(req.Messages) != 2 || kit.ContentsText(req.Messages[1].Content) != "compacted" {
				t.Fatalf("next ready hook received stale context: %+v", req.Messages)
			}

			return false, nil
		}),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if _, err := a.Run(context.Background(), kit.NewUserMessage(kit.NewTextContent("original"))); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if builds != 2 || readyCalls != 2 {
		t.Fatalf("construction calls = %d, ready calls = %d, want 2/2", builds, readyCalls)
	}

	request := model.CallAt(0)
	if request.Instructions != "static" || len(request.Messages) != 2 {
		t.Fatalf("rebuilding duplicated request parts: %+v", request)
	}
}

func TestRun_RetryRebuildsRequestWithoutRepeatingReadyHooks(t *testing.T) {
	model := kittest.NewModel(t,
		kittest.ModelResult{Error: kit.ErrContextLength},
		kittest.ModelResult{Response: kit.ModelResponse{
			Message: kit.NewModelMessage(kit.NewTextContent("done")), FinishReason: kit.FinishReasonStop,
		}},
	)
	builds, readyCalls := 0, 0
	prefix := kit.NewUserMessage(kit.NewTextContent("transient"))

	h := newTestHistory()
	h.read = func(messages []kit.Message) ([]kit.Message, error) {
		builds++

		return append([]kit.Message{prefix}, messages...), nil
	}

	a, err := New(model, h,
		WithInstructions("static"),
		WithOnModelRequestReady(func(*kit.RunContext, kit.ModelRequest) (bool, error) {
			readyCalls++

			return false, nil
		}),
		WithOnModelError(func(_ *kit.RunContext, _ kit.ModelRequest, err error) error {
			if !errors.Is(err, kit.ErrContextLength) {
				return err
			}

			h.replace(kit.NewUserMessage(kit.NewSummaryContent("compacted")))

			return nil
		}),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if _, err := a.Run(context.Background(), kit.NewUserMessage(kit.NewTextContent("original"))); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if builds != 2 || readyCalls != 1 {
		t.Fatalf("construction calls = %d, ready calls = %d, want 2/1", builds, readyCalls)
	}

	request := model.CallAt(1)
	if request.Instructions != "static" || len(request.Messages) != 2 || kit.ContentsText(request.Messages[1].Content) != "compacted" {
		t.Fatalf("retried request = %+v", request)
	}
}

func TestRun_RequestRebuildFailureStopsModelCall(t *testing.T) {
	for _, retry := range []bool{false, true} {
		name := "ready"

		var results []kittest.ModelResult
		if retry {
			name = "retry"
			results = []kittest.ModelResult{{Error: kit.ErrContextLength}}
		}

		t.Run(name, func(t *testing.T) {
			failure := errors.New("cannot rebuild request")
			model := kittest.NewModel(t, results...)
			builds := 0

			h := newTestHistory()
			h.read = func(messages []kit.Message) ([]kit.Message, error) {
				builds++
				if builds > 1 {
					return nil, failure
				}

				return messages, nil
			}

			a, err := New(model, h,
				WithOnModelRequestReady(func(*kit.RunContext, kit.ModelRequest) (bool, error) {
					return !retry, nil
				}),
				WithOnModelError(func(*kit.RunContext, kit.ModelRequest, error) error { return nil }),
			)
			if err != nil {
				t.Fatalf("New: %v", err)
			}

			if _, err := a.Run(context.Background(), kit.NewUserMessage(kit.NewTextContent("original"))); !errors.Is(err, failure) {
				t.Fatalf("Run error = %v, want construction error", err)
			}

			model.AssertCallCount(t, len(results))
		})
	}
}

func TestRun_ExecutesToolCallAndContinues(t *testing.T) {
	call := kit.NewToolCall("call-1", "add", map[string]any{"a": 3, "b": 4})
	toolCallMessage := kit.NewModelMessage(kit.NewToolCallContent(call))
	finalMessage := kit.NewModelMessage(kit.NewTextContent("3 + 4 = 7"))

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
		kit.NewUserMessage(kit.NewTextContent("add 3 and 4")),
	}

	a, err := New(model, history.New(), WithInstructions("use tools"), WithTools(tool))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	response, err := a.Run(context.Background(), messages[0])
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if response.Output.Text != "3 + 4 = 7" {
		t.Fatalf("Output = %q, want final text", response.Output.Text)
	}

	if response.Usage.InputTokens != 12 || response.Usage.OutputTokens != 5 {
		t.Fatalf("Usage = %+v, want input 12 output 5", response.Usage)
	}

	if response.FinishReason != kit.FinishReasonStop {
		t.Fatalf("FinishReason = %q, want final %q", response.FinishReason, kit.FinishReasonStop)
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

	recordedToolCall := toolCallMessage
	recordedToolCall.Usage = kit.Usage{InputTokens: 5, OutputTokens: 2}

	model.AssertCalledWith(t, 1, kit.ModelRequest{
		Instructions: "use tools",
		Messages: append(messages,
			recordedToolCall,
			kit.NewToolMessage(
				kit.NewToolResultContent(kit.NewToolResult(call, kit.NewToolOutput(kit.NewTextContent("7")), nil))),
		),
		Tools: []kit.Tool{tool},
	})
}

func TestRun_RecordsToolResultsIndividually(t *testing.T) {
	firstCall := kit.NewToolCall("call-1", "first", nil)
	secondCall := kit.NewToolCall("call-2", "second", nil)
	firstTool := kittest.NewTool(t, "first", "first tool", kittest.ToolResult{Result: "one"})
	secondTool := kittest.NewTool(t, "second", "second tool", kittest.ToolResult{Result: "two"})
	model := kittest.NewModel(t,
		kittest.ModelResult{Response: kit.ModelResponse{
			Message: kit.NewModelMessage(
				kit.NewToolCallContent(firstCall),
				kit.NewToolCallContent(secondCall),
			),
			FinishReason: kit.FinishReasonToolCall,
		}},
		kittest.ModelResult{Response: kit.ModelResponse{
			Message:      kit.NewModelMessage(kit.NewTextContent("done")),
			FinishReason: kit.FinishReasonStop,
		}},
	)
	h := newTestHistory()

	a, err := New(model, h, WithTools(firstTool, secondTool))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if _, err := a.Run(context.Background(), kit.NewUserMessage(kit.NewTextContent("add"))); err != nil {
		t.Fatalf("Run: %v", err)
	}

	wantBatchSizes := []int{1, 1, 1, 1, 1}
	if len(h.appends) != len(wantBatchSizes) {
		t.Fatalf("len(Append calls) = %d, want %d", len(h.appends), len(wantBatchSizes))
	}

	for index, want := range wantBatchSizes {
		if got := len(h.appends[index]); got != want {
			t.Fatalf("len(Append call %d) = %d, want %d", index, got, want)
		}
	}

	if got := len(h.appends[1][0].ToolCalls()); got != 2 {
		t.Fatalf("tool-call record has %d tool calls, want 2", got)
	}

	if got := len(h.appends[2][0].ToolResults()); got != 1 {
		t.Fatalf("first tool-result record has %d tool results, want 1", got)
	}

	if got := len(h.appends[3][0].ToolResults()); got != 1 {
		t.Fatalf("second tool-result record has %d tool results, want 1", got)
	}

	requestMessages := model.CallAt(1).Messages

	firstResults := requestMessages[len(requestMessages)-2].ToolResults()
	if len(firstResults) != 1 || firstResults[0].Call.ID != firstCall.ID {
		t.Fatalf("first model-request results = %+v, want first tool result", firstResults)
	}

	secondResults := requestMessages[len(requestMessages)-1].ToolResults()
	if len(secondResults) != 1 || secondResults[0].Call.ID != secondCall.ID {
		t.Fatalf("second model-request results = %+v, want second tool result", secondResults)
	}
}

func TestRun_SendsToolErrorBackToModel(t *testing.T) {
	toolErr := errors.New("tool failed")
	call := kit.NewToolCall("call-1", "fail", map[string]any{"input": "x"})
	toolCallMessage := kit.NewModelMessage(kit.NewToolCallContent(call))

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
				Message:      kit.NewModelMessage(kit.NewTextContent("handled")),
				FinishReason: kit.FinishReasonStop,
			},
		},
	)

	input := kit.NewUserMessage(kit.NewTextContent("try failing tool"))

	a, err := New(model, history.New(), WithInstructions("use tools"), WithTools(tool))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, err = a.Run(context.Background(), input)
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

	if got := kit.ContentsText(results[0].Output.Content); got != "" {
		t.Fatalf("tool result content = %q, want empty", got)
	}
}

func TestRun_SendsMissingToolErrorBackToModel(t *testing.T) {
	call := kit.NewToolCall("call-1", "missing", map[string]any{"input": "x"})

	model := kittest.NewModel(t,
		kittest.ModelResult{
			Response: kit.ModelResponse{
				Message:      kit.NewModelMessage(kit.NewToolCallContent(call)),
				FinishReason: kit.FinishReasonToolCall,
			},
		},
		kittest.ModelResult{
			Response: kit.ModelResponse{
				Message:      kit.NewModelMessage(kit.NewTextContent("handled")),
				FinishReason: kit.FinishReasonStop,
			},
		},
	)

	input := kit.NewUserMessage(kit.NewTextContent("try missing tool"))

	a, err := New(model, history.New(), WithInstructions("use tools"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, err = a.Run(context.Background(), input)
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

func TestRun_ToolHookErrorIsSentBackToModel(t *testing.T) {
	call := kit.NewToolCall("call-1", "search", map[string]any{"q": "go"})

	tool := kittest.NewTool(t, "search", "search tool")

	model := kittest.NewModel(t,
		kittest.ModelResult{
			Response: kit.ModelResponse{
				Message:      kit.NewModelMessage(kit.NewToolCallContent(call)),
				FinishReason: kit.FinishReasonToolCall,
			},
		},
		kittest.ModelResult{
			Response: kit.ModelResponse{
				Message:      kit.NewModelMessage(kit.NewTextContent("recovered")),
				FinishReason: kit.FinishReasonStop,
			},
		},
	)

	input := kit.NewUserMessage(kit.NewTextContent("search go"))

	a, err := New(model, history.New(), WithTools(tool),
		WithOnToolCall(func(_ *kit.RunContext, _ kit.ToolCall) (kit.ToolCall, error) {
			return kit.ToolCall{}, errors.New("policy denied")
		}),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	resp, err := a.Run(context.Background(), input)
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

func TestRun_ContextCanceledDuringToolAbortsRun(t *testing.T) {
	call := kit.NewToolCall("call-1", "cancel", nil)
	toolCallMessage := kit.NewModelMessage(kit.NewToolCallContent(call))

	model := kittest.NewModel(t, kittest.ModelResult{
		Response: kit.ModelResponse{
			Message:      toolCallMessage,
			FinishReason: kit.FinishReasonToolCall,
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	tool := &cancelTool{cancel: cancel}

	input := kit.NewUserMessage(kit.NewTextContent("cancel during tool"))

	h := newTestHistory()

	a, err := New(model, h, WithTools(tool))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	resp, err := a.Run(ctx, input)
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

	if len(h.appends) != 2 {
		t.Fatalf("len(Append calls) = %d, want input and tool call", len(h.appends))
	}

	if len(h.appends[0]) != 1 || h.appends[0][0].Role != kit.RoleUser {
		t.Fatalf("first Append call = %+v, want user input", h.appends[0])
	}

	if got := len(h.appends[1][0].ToolCalls()); got != 1 {
		t.Fatalf("second Append call has %d tool calls, want 1", got)
	}
}

func TestRun_RecordsCompletedToolBeforeNextToolIsCanceled(t *testing.T) {
	firstCall := kit.NewToolCall("call-1", "ok", nil)
	secondCall := kit.NewToolCall("call-2", "cancel", nil)
	toolCallMessage := kit.NewModelMessage(
		kit.NewToolCallContent(firstCall),
		kit.NewToolCallContent(secondCall),
	)

	model := kittest.NewModel(t, kittest.ModelResult{
		Response: kit.ModelResponse{
			Message:      toolCallMessage,
			FinishReason: kit.FinishReasonToolCall,
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	firstTool := kittest.NewTool(t, "ok", "ok tool", kittest.ToolResult{Result: "done"})
	secondTool := &cancelTool{cancel: cancel}
	h := newTestHistory()

	a, err := New(model, h, WithTools(firstTool, secondTool))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	response, err := a.Run(ctx, kit.NewUserMessage(kit.NewTextContent("run tools")))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run error = %v, want context canceled", err)
	}

	firstTool.AssertCallCount(t, 1)

	if !secondTool.called {
		t.Fatal("second tool was not called")
	}

	if len(h.appends) != 3 {
		t.Fatalf("len(Append calls) = %d, want input, tool call, and first result", len(h.appends))
	}

	results := h.appends[2][0].ToolResults()
	if len(results) != 1 || results[0].Call.ID != firstCall.ID {
		t.Fatalf("persisted results = %+v, want only first tool result", results)
	}

	if len(response.Messages) != 2 {
		t.Fatalf("len(Messages) = %d, want tool call and first result", len(response.Messages))
	}
}

func TestStream_ReturnsEventsAndResult(t *testing.T) {
	model := kittest.NewModel(t, kittest.ModelResult{
		Events: []kit.ModelEvent{
			kit.NewModelContentStartedEvent(kit.NewTextContent("done")),
			kit.NewModelContentDoneEvent(kit.NewTextContent("done")),
		},
		Response: kit.ModelResponse{
			Message:      kit.NewModelMessage(kit.NewTextContent("done")),
			FinishReason: kit.FinishReasonStop,
			Usage:        kit.Usage{InputTokens: 3, OutputTokens: 2},
		},
	})

	input := kit.NewUserMessage(kit.NewTextContent("hello"))

	a, err := New(model, history.New(), WithInstructions("be brief"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	stream := a.Stream(context.Background(), input)

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
	toolCallMessage := kit.NewModelMessage(kit.NewToolCallContent(call))
	finalMessage := kit.NewModelMessage(kit.NewTextContent("3 + 4 = 7"))

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

	input := kit.NewUserMessage(kit.NewTextContent("add 3 and 4"))

	a, err := New(model, history.New(), WithInstructions("use tools"), WithTools(tool))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	stream := a.Stream(context.Background(), input)

	var sawToolResult bool

	for event := range stream.Iter() {
		if event.ToolResult != nil {
			sawToolResult = true

			if got := kit.ContentsText(event.ToolResult.Output.Content); got != "7" {
				t.Fatalf("ToolResult.Content = %q, want 7", got)
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
			Message:      kit.NewModelMessage(kit.NewTextContent("done")),
			FinishReason: kit.FinishReasonStop,
			Usage:        kit.Usage{InputTokens: 3, OutputTokens: 2},
		},
	})

	input := kit.NewUserMessage(kit.NewTextContent("hello"))

	a, err := New(model, history.New(), WithInstructions("be brief"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	stream := a.Stream(context.Background(), input)

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

func TestExecuteTool_RecoversPanic(t *testing.T) {
	tool := panicTool{name: "panic"}

	a, err := New(nil, history.New(), WithTools(tool))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	result, err := a.executeTool(&kit.RunContext{Context: context.Background()}, kit.NewToolCall("call-1", "panic", nil))
	if err != nil {
		t.Fatalf("executeTool: %v", err)
	}

	if got := kit.ContentsText(result.Output.Content); got != "" {
		t.Fatalf("Content = %q, want empty", got)
	}

	if !strings.Contains(result.Error, `tool "panic" panicked: bad things`) {
		t.Fatalf("Error = %q, want panic recovery error", result.Error)
	}
}

func TestExecuteTool_OnToolCallHookErrorBecomesResult(t *testing.T) {
	tool := kittest.NewTool(t, "ok", "ok tool")

	hookErr := errors.New("policy denied")

	a, err := New(nil, history.New(), WithTools(tool),
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

	if got := kit.ContentsText(result.Output.Content); got != "" {
		t.Fatalf("Content = %q, want empty (tool must not run)", got)
	}

	tool.AssertNeverCalled(t)
}

func TestExecuteTool_OnToolResultHookErrorBecomesResult(t *testing.T) {
	tool := kittest.NewTool(t, "ok", "ok tool", kittest.ToolResult{Result: "raw output"})

	hookErr := errors.New("redaction failed")

	a, err := New(nil, history.New(), WithTools(tool),
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

	if got := kit.ContentsText(result.Output.Content); got != "" {
		t.Fatalf("Content = %q, want empty (output replaced by hook error)", got)
	}

	tool.AssertCallCount(t, 1)
}

type testHistory struct {
	messages []kit.Message

	read    func([]kit.Message) ([]kit.Message, error)
	appends [][]kit.Message
}

func newTestHistory(messages ...kit.Message) *testHistory {
	return &testHistory{messages: messages}
}

func (h *testHistory) Messages(ctx context.Context) ([]kit.Message, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	messages := slices.Clone(h.messages)
	if h.read == nil {
		return messages, nil
	}

	return h.read(messages)
}

func (h *testHistory) Append(_ context.Context, messages ...kit.Message) error {
	h.appends = append(h.appends, slices.Clone(messages))
	h.messages = append(h.messages, messages...)

	return nil
}

func (h *testHistory) replace(messages ...kit.Message) {
	h.messages = messages
}

type panicTool struct {
	name string
}

func (p panicTool) Definition() kit.ToolDefinition {
	return kit.ToolDefinition{Name: p.name}
}

func (p panicTool) Execute(*kit.RunContext, kit.ToolCall) (kit.ToolOutput, error) {
	panic("bad things")
}

type cancelTool struct {
	cancel context.CancelFunc
	called bool
}

func (c *cancelTool) Definition() kit.ToolDefinition {
	return kit.ToolDefinition{Name: "cancel"}
}

func (c *cancelTool) Execute(rc *kit.RunContext, _ kit.ToolCall) (kit.ToolOutput, error) {
	c.called = true
	c.cancel()

	return kit.ToolOutput{}, rc.Err()
}

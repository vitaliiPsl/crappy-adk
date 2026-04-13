package agent

import (
	"context"
	"fmt"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

const (
	defaultCompactionThreshold = 0.8
)

// Config holds configuration for an [Agent].
type Config struct {
	// Generation controls generation parameters used on every model request.
	Generation kit.GenerationConfig

	// ToolExecution controls whether tool calls run in parallel or sequentially.
	// Defaults to ToolExecutionParallel.
	ToolExecution ToolExecutionMode

	// CompactionThreshold is the fraction of the context window that triggers
	// compaction. Defaults to 0.8 when zero.
	CompactionThreshold float64
}

// Agent runs a ReAct loop: it calls the model, executes any requested tool
// calls, and feeds the results back until the model returns a final response.
type Agent struct {
	config       Config
	instructions []kit.Instruction

	model kit.Model
	tools map[string]kit.Tool

	compactor kit.Compactor
	hooks     hooks
}

// New creates an agent backed by the given model. Options are applied in order.
func New(model kit.Model, options ...Option) (*Agent, error) {
	agent := &Agent{
		model: model,
		tools: make(map[string]kit.Tool),
		config: Config{
			CompactionThreshold: defaultCompactionThreshold,
		},
	}

	for _, opt := range options {
		if err := opt(agent); err != nil {
			return nil, err
		}
	}

	return agent, nil
}

// Run executes the ReAct loop and returns the accumulated [kit.Result] once the
// agent reaches a final answer. Use [Agent.Stream] instead to receive
// incremental events as the agent works.
func (a *Agent) Run(ctx context.Context, messages []kit.Message) (kit.Result, error) {
	s, err := a.Stream(ctx, messages)
	if err != nil {
		return kit.Result{}, err
	}

	return s.Result()
}

// Stream executes the ReAct loop and returns a [kit.Stream] that emits
// incremental [kit.Event] values — text deltas, thinking deltas, tool calls,
// and tool results — as the agent works. Call [kit.Stream.Result] after
// iteration to retrieve the accumulated [kit.Result].
func (a *Agent) Stream(ctx context.Context, msgs []kit.Message) (*kit.Stream[kit.Event, kit.Result], error) {
	instruction, err := kit.ComposeInstructions(ctx, "\n\n", a.instructions...)
	if err != nil {
		return nil, err
	}

	ctx, msgs, err = a.hooks.onRunStart(ctx, msgs)
	if err != nil {
		return nil, err
	}

	return kit.NewStream(func(yield func(kit.Event, error) bool) kit.Result {
		response, runErr := a.runLoop(ctx, instruction, msgs, yield)

		if _, hookErr := a.hooks.onRunEnd(ctx, response, runErr); hookErr != nil {
			yield(kit.Event{}, hookErr)

			return response
		}

		if runErr != nil {
			yield(kit.Event{}, runErr)
		}

		return response
	}), nil
}

func (a *Agent) runLoop(
	ctx context.Context,
	instruction string,
	msgs []kit.Message,
	yield func(kit.Event, error) bool,
) (response kit.Result, err error) {
	for {
		if err := ctx.Err(); err != nil {
			return response, err
		}

		ctx, msgs, err = a.hooks.onTurnStart(ctx, msgs)
		if err != nil {
			return response, err
		}

		assistantMsg, usage, err := a.callModel(ctx, instruction, msgs, yield)
		if err != nil {
			return response, err
		}

		msgs = append(msgs, assistantMsg)
		response.Messages = append(response.Messages, assistantMsg)

		response.Usage.Add(usage)
		response.LastUsage = usage

		if !yield(kit.NewMessageEvent(assistantMsg), nil) {
			return response, nil
		}

		if len(assistantMsg.ToolCalls) == 0 {
			if len(assistantMsg.Content) > 0 {
				response.Output = assistantMsg.Content[0]
			}

			return response, nil
		}

		toolMsgs, ok := a.callTools(ctx, assistantMsg.ToolCalls, yield)
		if !ok {
			return response, nil
		}

		msgs = append(msgs, toolMsgs...)
		response.Messages = append(response.Messages, toolMsgs...)

		if a.needsCompaction(usage) {
			compacted, summaryMsg, ok := a.compact(ctx, msgs, yield)
			if !ok {
				return response, nil
			}

			msgs = compacted

			if summaryMsg != nil {
				response.Messages = append(response.Messages, *summaryMsg)
			}
		}

		ctx, err = a.hooks.onTurnEnd(ctx, msgs)
		if err != nil {
			return response, err
		}
	}
}

func (a *Agent) callModel(
	ctx context.Context,
	instruction string,
	msgs []kit.Message,
	yield func(kit.Event, error) bool,
) (kit.Message, kit.Usage, error) {
	req := kit.ModelRequest{
		Instruction: instruction,
		Messages:    msgs,
		Tools:       a.toolDefinitions(),
		Config:      a.config.Generation,
	}

	ctx, req, err := a.hooks.onModelRequest(ctx, req)
	if err != nil {
		return kit.Message{}, kit.Usage{}, fmt.Errorf("model request hook failed: %w", err)
	}

	stream, err := a.model.GenerateStream(ctx, req)
	if err != nil {
		return kit.Message{}, kit.Usage{}, err
	}

	if err := a.forwardModelEvents(stream, yield); err != nil {
		return kit.Message{}, kit.Usage{}, err
	}

	modelResp, streamErr := stream.Result()
	if streamErr != nil {
		return kit.Message{}, kit.Usage{}, streamErr
	}

	_, resp, err := a.hooks.onModelResponse(ctx, modelResp)
	if err != nil {
		return kit.Message{}, kit.Usage{}, fmt.Errorf("model response hook failed: %w", err)
	}

	return resp.Message, resp.Usage, nil
}

func (a *Agent) forwardModelEvents(
	stream *kit.Stream[kit.ModelEvent, kit.ModelResponse],
	yield func(kit.Event, error) bool,
) error {
	for ev, err := range stream.Iter() {
		if err != nil {
			return err
		}

		event, ok := modelEventToAgentEvent(ev)
		if !ok {
			continue
		}

		if !yield(event, nil) {
			return nil
		}
	}

	return nil
}

func modelEventToAgentEvent(ev kit.ModelEvent) (kit.Event, bool) {
	switch ev.Type {
	case kit.ModelEventThinkingStarted:
		return kit.NewThinkingStartedEvent(), true
	case kit.ModelEventThinkingDelta:
		return kit.NewThinkingDeltaEvent(ev.Text), true
	case kit.ModelEventThinkingDone:
		return kit.NewThinkingDoneEvent(ev.Thinking), true
	case kit.ModelEventContentPartStarted:
		return kit.NewContentPartStartedEvent(ev.ContentPartType), true
	case kit.ModelEventContentPartDelta:
		return kit.NewContentPartDeltaEvent(ev.ContentPartType, ev.Text), true
	case kit.ModelEventContentPartDone:
		if ev.ContentPart == nil {
			return kit.Event{}, false
		}

		return kit.NewContentPartDoneEvent(*ev.ContentPart), true
	case kit.ModelEventToolCallStarted:
		if ev.ToolCall == nil {
			return kit.Event{}, false
		}

		return kit.NewToolCallStartedEvent(*ev.ToolCall), true
	case kit.ModelEventToolCallDone:
		if ev.ToolCall == nil {
			return kit.Event{}, false
		}

		return kit.NewToolCallDoneEvent(*ev.ToolCall), true
	default:
		return kit.Event{}, false
	}
}

func (a *Agent) toolDefinitions() []kit.ToolDefinition {
	defs := make([]kit.ToolDefinition, 0, len(a.tools))
	for _, tool := range a.tools {
		defs = append(defs, tool.Definition())
	}

	return defs
}

func (a *Agent) callTools(
	ctx context.Context,
	toolCalls []kit.ToolCall,
	yield func(kit.Event, error) bool,
) ([]kit.Message, bool) {
	if a.config.ToolExecution == ToolExecutionSequential {
		return a.callToolsSequential(ctx, toolCalls, yield)
	}

	return a.callToolsParallel(ctx, toolCalls, yield)
}

func (a *Agent) callToolsSequential(
	ctx context.Context,
	toolCalls []kit.ToolCall,
	yield func(kit.Event, error) bool,
) ([]kit.Message, bool) {
	msgs := make([]kit.Message, 0, len(toolCalls))

	for _, toolCall := range toolCalls {
		result := a.callTool(ctx, toolCall)

		if !yield(kit.NewToolResultEvent(result), nil) {
			return msgs, false
		}

		toolMsg := kit.NewToolMessage(result.Content, result.Call)

		if !yield(kit.NewMessageEvent(toolMsg), nil) {
			return msgs, false
		}

		msgs = append(msgs, toolMsg)
	}

	return msgs, true
}

func (a *Agent) callToolsParallel(
	ctx context.Context,
	toolCalls []kit.ToolCall,
	yield func(kit.Event, error) bool,
) ([]kit.Message, bool) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	msgs := make([]kit.Message, 0, len(toolCalls))

	results := make(chan kit.ToolResult, len(toolCalls))

	for _, toolCall := range toolCalls {
		go func() {
			results <- a.callTool(ctx, toolCall)
		}()
	}

	for range len(toolCalls) {
		var result kit.ToolResult

		select {
		case result = <-results:
		case <-ctx.Done():
			return msgs, false
		}

		if !yield(kit.NewToolResultEvent(result), nil) {
			return msgs, false
		}

		toolMsg := kit.NewToolMessage(result.Content, result.Call)

		if !yield(kit.NewMessageEvent(toolMsg), nil) {
			return msgs, false
		}

		msgs = append(msgs, toolMsg)
	}

	return msgs, true
}

func (a *Agent) callTool(ctx context.Context, toolCall kit.ToolCall) (result kit.ToolResult) {
	defer func() {
		if recovered := recover(); recovered != nil {
			result = kit.NewErrorToolResult(toolCall, fmt.Errorf("recovered: %v", recovered))
		}
	}()

	ctx, toolCall, err := a.hooks.onToolCall(ctx, toolCall)
	if err != nil {
		return kit.NewErrorToolResult(toolCall, err)
	}

	tool, ok := a.tools[toolCall.Name]
	if !ok {
		return kit.NewErrorToolResult(toolCall, fmt.Errorf("tool not found: %s", toolCall.Name))
	}

	content, err := tool.Execute(ctx, toolCall.Arguments)
	if err != nil {
		return kit.NewErrorToolResult(toolCall, err)
	}

	result = kit.ToolResult{Call: toolCall, Content: content}

	_, result, err = a.hooks.onToolResult(ctx, result)
	if err != nil {
		return kit.NewErrorToolResult(toolCall, err)
	}

	return result
}

func (a *Agent) needsCompaction(lastUsage kit.Usage) bool {
	if a.compactor == nil {
		return false
	}

	inputLimit := a.model.Config().InputLimit
	if inputLimit <= 0 {
		return false
	}

	used := int64(lastUsage.InputTokens) + int64(lastUsage.OutputTokens)

	return used > int64(float64(inputLimit)*a.config.CompactionThreshold)
}

func (a *Agent) compact(
	ctx context.Context,
	msgs []kit.Message,
	yield func(kit.Event, error) bool,
) ([]kit.Message, *kit.Message, bool) {
	compacted, summary, err := a.compactor.Compact(ctx, msgs)
	if err != nil {
		yield(kit.Event{}, fmt.Errorf("compactor failed: %w", err))

		return msgs, nil, false
	}

	if summary == "" {
		return msgs, nil, true
	}

	summaryMsg := kit.NewSummaryMessage(summary)

	if !yield(kit.NewCompactionDoneEvent(summary), nil) {
		return msgs, nil, false
	}

	if !yield(kit.NewMessageEvent(summaryMsg), nil) {
		return msgs, nil, false
	}

	return compacted, &summaryMsg, true
}

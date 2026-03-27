package kit

import (
	"context"
	"fmt"
)

const (
	defaultCompactionThreshold = 0.8
)

// AgentConfig holds configuration for an [Agent].
type AgentConfig struct {
	// Generation controls generation parameters used on every model request.
	Generation GenerationConfig

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
	config       AgentConfig
	instructions []Instruction

	model Model
	tools map[string]Tool

	compactor Compactor
	hooks     hooks
}

// NewAgent creates an agent backed by the given model. Options are applied in order.
func NewAgent(model Model, options ...AgentOption) (*Agent, error) {
	agent := &Agent{
		model: model,
		tools: make(map[string]Tool),
		config: AgentConfig{
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

// Run executes the ReAct loop and returns the accumulated [Response] once the
// agent reaches a final answer. Use [Agent.Stream] instead to receive
// incremental events as the agent works.
func (a *Agent) Run(ctx context.Context, messages []Message) (Response, error) {
	s, err := a.Stream(ctx, messages)
	if err != nil {
		return Response{}, err
	}

	return s.Result()
}

// Stream executes the ReAct loop and returns a [Stream] that emits incremental
// [Event] values — text deltas, thinking deltas, tool calls, and tool results —
// as the agent works. Call [Stream.Result] after iteration to retrieve the
// accumulated [Response].
func (a *Agent) Stream(ctx context.Context, msgs []Message) (*Stream, error) {
	instruction, err := ComposeInstructions(ctx, "\n\n", a.instructions...)
	if err != nil {
		return nil, err
	}

	ctx, msgs, err = a.hooks.onRunStart(ctx, msgs)
	if err != nil {
		return nil, err
	}

	return NewStream(func(yield func(Event, error) bool) Response {
		response, runErr := a.runLoop(ctx, instruction, msgs, yield)

		if _, hookErr := a.hooks.onRunEnd(ctx, response, runErr); hookErr != nil {
			yield(Event{}, hookErr)

			return response
		}

		if runErr != nil {
			yield(Event{}, runErr)
		}

		return response
	}), nil
}

func (a *Agent) runLoop(
	ctx context.Context,
	instruction string,
	msgs []Message,
	yield func(Event, error) bool,
) (response Response, err error) {
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

		if !yield(NewMessageEvent(assistantMsg), nil) {
			return response, nil
		}

		if len(assistantMsg.ToolCalls) == 0 {
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
	msgs []Message,
	yield func(Event, error) bool,
) (Message, Usage, error) {
	req := ModelRequest{
		Instruction: instruction,
		Messages:    msgs,
		Tools:       a.toolDefinitions(),
		Config:      a.config.Generation,
	}

	ctx, req, err := a.hooks.onModelRequest(ctx, req)
	if err != nil {
		return Message{}, Usage{}, fmt.Errorf("model request hook failed: %w", err)
	}

	stream, err := a.model.GenerateStream(ctx, req)
	if err != nil {
		return Message{}, Usage{}, err
	}

	for chunk, err := range stream.Iter() {
		if err != nil {
			return Message{}, Usage{}, err
		}

		var event Event

		switch chunk.Type {
		case ChunkTypeThinking:
			event = NewThinkingDeltaEvent(chunk.Thinking)
		case ChunkTypeText:
			event = NewTextDeltaEvent(chunk.Text)
		case ChunkTypeToolCall:
			event = NewToolCallEvent(*chunk.ToolCall)
		default:
			continue
		}

		if !yield(event, nil) {
			break
		}
	}

	modelResp, streamErr := stream.Response()
	if streamErr != nil {
		return Message{}, Usage{}, streamErr
	}

	_, resp, err := a.hooks.onModelResponse(ctx, modelResp)
	if err != nil {
		return Message{}, Usage{}, fmt.Errorf("model response hook failed: %w", err)
	}

	return resp.Message, resp.Usage, nil
}

func (a *Agent) toolDefinitions() []ToolDefinition {
	defs := make([]ToolDefinition, 0, len(a.tools))
	for _, tool := range a.tools {
		defs = append(defs, tool.Definition())
	}

	return defs
}

func (a *Agent) callTools(
	ctx context.Context,
	toolCalls []ToolCall,
	yield func(Event, error) bool,
) ([]Message, bool) {
	if a.config.ToolExecution == ToolExecutionSequential {
		return a.callToolsSequential(ctx, toolCalls, yield)
	}

	return a.callToolsParallel(ctx, toolCalls, yield)
}

func (a *Agent) callToolsSequential(
	ctx context.Context,
	toolCalls []ToolCall,
	yield func(Event, error) bool,
) ([]Message, bool) {
	msgs := make([]Message, 0, len(toolCalls))

	for _, toolCall := range toolCalls {
		result := a.callTool(ctx, toolCall)

		if !yield(NewToolResultEvent(result), nil) {
			return msgs, false
		}

		toolMsg := NewToolMessage(result.Content, result.ToolCall)

		if !yield(NewMessageEvent(toolMsg), nil) {
			return msgs, false
		}

		msgs = append(msgs, toolMsg)
	}

	return msgs, true
}

func (a *Agent) callToolsParallel(
	ctx context.Context,
	toolCalls []ToolCall,
	yield func(Event, error) bool,
) ([]Message, bool) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	msgs := make([]Message, 0, len(toolCalls))

	results := make(chan ToolResult, len(toolCalls))

	for _, toolCall := range toolCalls {
		go func() {
			results <- a.callTool(ctx, toolCall)
		}()
	}

	for range len(toolCalls) {
		var result ToolResult

		select {
		case result = <-results:
		case <-ctx.Done():
			return msgs, false
		}

		if !yield(NewToolResultEvent(result), nil) {
			return msgs, false
		}

		toolMsg := NewToolMessage(result.Content, result.ToolCall)

		if !yield(NewMessageEvent(toolMsg), nil) {
			return msgs, false
		}

		msgs = append(msgs, toolMsg)
	}

	return msgs, true
}

func (a *Agent) callTool(ctx context.Context, toolCall ToolCall) (result ToolResult) {
	defer func() {
		if recovered := recover(); recovered != nil {
			result = NewErrorToolResult(toolCall, fmt.Errorf("recovered: %v", recovered))
		}
	}()

	ctx, toolCall, err := a.hooks.onToolCall(ctx, toolCall)
	if err != nil {
		return NewErrorToolResult(toolCall, err)
	}

	tool, ok := a.tools[toolCall.Name]
	if !ok {
		return NewErrorToolResult(toolCall, fmt.Errorf("tool not found: %s", toolCall.Name))
	}

	content, err := tool.Execute(ctx, toolCall.Arguments)
	if err != nil {
		return NewErrorToolResult(toolCall, err)
	}

	result = ToolResult{ToolCall: toolCall, Content: content}

	_, result, err = a.hooks.onToolResult(ctx, result)
	if err != nil {
		return NewErrorToolResult(toolCall, err)
	}

	return result
}

func (a *Agent) needsCompaction(lastUsage Usage) bool {
	if a.compactor == nil {
		return false
	}

	contextWindow := a.model.Config().ContextWindow
	if contextWindow <= 0 {
		return false
	}

	used := int64(lastUsage.InputTokens) + int64(lastUsage.OutputTokens)

	return used > int64(float64(contextWindow)*a.config.CompactionThreshold)
}

func (a *Agent) compact(
	ctx context.Context,
	msgs []Message,
	yield func(Event, error) bool,
) ([]Message, *Message, bool) {
	compacted, summary, err := a.compactor.Compact(ctx, msgs)
	if err != nil {
		yield(Event{}, fmt.Errorf("compactor failed: %w", err))

		return msgs, nil, false
	}

	if summary == "" {
		return msgs, nil, true
	}

	summaryMsg := NewSummaryMessage(summary)

	if !yield(NewContextSummaryEvent(summary), nil) {
		return msgs, nil, false
	}

	if !yield(NewMessageEvent(summaryMsg), nil) {
		return msgs, nil, false
	}

	return compacted, &summaryMsg, true
}

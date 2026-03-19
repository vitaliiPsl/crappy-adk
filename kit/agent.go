package kit

import (
	"context"
	"fmt"
	"slices"
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
			CompactionThreshold: 0.8,
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

	return s.Result(), nil
}

// Stream executes the ReAct loop and returns a [Stream] that emits incremental
// [Event] values — text deltas, thinking deltas, tool calls, and tool results —
// as the agent works. Call [Stream.Result] after iteration to retrieve the
// accumulated [Response].
func (a *Agent) Stream(ctx context.Context, messages []Message) (*Stream, error) {
	instruction, err := ComposeInstructions(ctx, "\n\n", a.instructions...)
	if err != nil {
		return nil, err
	}

	msgs := slices.Clone(messages)

	ctx, msgs, err = a.hooks.onRunStart(ctx, msgs)
	if err != nil {
		return nil, err
	}

	s := &Stream{}

	s.iter = func(yield func(Event, error) bool) {
		var runErr error

		defer func() { s.done = true }()

		defer func() {
			if _, err := a.hooks.onRunEnd(ctx, s.response, runErr); err != nil {
				yield(Event{}, err)
			}
		}()

		for {
			if err := ctx.Err(); err != nil {
				runErr = err
				yield(Event{}, err)

				return
			}

			ctx, msgs, err = a.hooks.onTurnStart(ctx, msgs)
			if err != nil {
				runErr = err
				yield(Event{}, err)

				return
			}

			assistantMsg, usage, err := a.callModel(ctx, instruction, msgs, yield)
			if err != nil {
				runErr = err
				yield(Event{}, err)

				return
			}

			s.response.Usage.Add(usage)

			msgs = append(msgs, assistantMsg)
			s.response.Messages = append(s.response.Messages, assistantMsg)

			if len(assistantMsg.ToolCalls) == 0 {
				return
			}

			toolMsgs, ok := a.callTools(ctx, assistantMsg.ToolCalls, yield)
			if !ok {
				return
			}

			msgs = append(msgs, toolMsgs...)
			s.response.Messages = append(s.response.Messages, toolMsgs...)

			if a.needsCompaction(usage) {
				compacted, summary, compactErr := a.compactor.Compact(ctx, msgs)
				if compactErr != nil {
					runErr = fmt.Errorf("compactor failed: %w", compactErr)
					yield(Event{}, runErr)

					return
				}

				if summary != "" {
					msgs = compacted

					summaryMsg := NewSummaryMessage(summary)
					s.response.Messages = append(s.response.Messages, summaryMsg)

					if !yield(NewContextSummaryEvent(summary), nil) {
						return
					}
				}
			}

			ctx, err = a.hooks.onTurnEnd(ctx, msgs)
			if err != nil {
				runErr = err
				yield(Event{}, err)

				return
			}
		}
	}

	return s, nil
}

func (a *Agent) callModel(ctx context.Context, instruction string, msgs []Message, yield func(Event, error) bool) (Message, Usage, error) {
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

		if chunk.Type == ChunkTypeThinking {
			if !yield(NewThinkingDeltaEvent(chunk.Thinking), nil) {
				break
			}
		}

		if chunk.Type == ChunkTypeText {
			if !yield(NewTextDeltaEvent(chunk.Text), nil) {
				break
			}
		}
	}

	_, resp, err := a.hooks.onModelResponse(ctx, stream.Response())
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

func (a *Agent) callTools(ctx context.Context, toolCalls []ToolCall, yield func(Event, error) bool) ([]Message, bool) {
	if a.config.ToolExecution == ToolExecutionSequential {
		return a.callToolsSequential(ctx, toolCalls, yield)
	}

	return a.callToolsParallel(ctx, toolCalls, yield)
}

func (a *Agent) callToolsSequential(ctx context.Context, toolCalls []ToolCall, yield func(Event, error) bool) ([]Message, bool) {
	msgs := make([]Message, 0, len(toolCalls))

	for _, toolCall := range toolCalls {
		if !yield(NewToolCallEvent(toolCall), nil) {
			return msgs, false
		}

		result := a.callTool(ctx, toolCall)

		if !yield(NewToolResultEvent(result), nil) {
			return msgs, false
		}

		msgs = append(msgs, NewToolMessage(result.Content, result.ToolCall))
	}

	return msgs, true
}

func (a *Agent) callToolsParallel(ctx context.Context, toolCalls []ToolCall, yield func(Event, error) bool) ([]Message, bool) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	msgs := make([]Message, 0, len(toolCalls))

	results := make(chan ToolResult, len(toolCalls))

	for _, toolCall := range toolCalls {
		if !yield(NewToolCallEvent(toolCall), nil) {
			return nil, false
		}

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

		msgs = append(msgs, NewToolMessage(result.Content, result.ToolCall))
	}

	return msgs, true
}

func (a *Agent) callTool(ctx context.Context, toolCall ToolCall) (result ToolResult) {
	result.ToolCall = toolCall

	defer func() {
		if recovered := recover(); recovered != nil {
			result.Error = fmt.Sprintf("recovered. Error: %v", recovered)
			result.Content = result.Error
		}
	}()

	ctx, toolCall, err := a.hooks.onToolCall(ctx, toolCall)
	result.ToolCall = toolCall

	if err != nil {
		result.Error = err.Error()
		result.Content = result.Error

		return result
	}

	tool, ok := a.tools[toolCall.Name]
	if !ok {
		result.Error = fmt.Sprintf("tool not found: %s", toolCall.Name)
		result.Content = result.Error

		return result
	}

	result.Content, err = tool.Execute(ctx, toolCall.Arguments)
	if err != nil {
		result.Error = err.Error()
		result.Content = result.Error
	}

	_, result, err = a.hooks.onToolResult(ctx, result)
	if err != nil {
		result.Error = err.Error()
		result.Content = result.Error
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

	used := lastUsage.InputTokens + lastUsage.OutputTokens

	return used > int32(float64(contextWindow)*a.config.CompactionThreshold)
}

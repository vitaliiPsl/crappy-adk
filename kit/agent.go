package kit

import (
	"context"
	"fmt"
	"slices"
)

// Agent runs a ReAct loop: it calls the model, executes any requested tool
// calls, and feeds the results back until the model returns a final response.
type Agent struct {
	instructions []Instruction

	model Model
	tools map[string]Tool

	generationConfig GenerationConfig
	hooks            hooks
}

// NewAgent creates an agent backed by the given model. Options are applied in order.
func NewAgent(model Model, options ...AgentOptions) (*Agent, error) {
	agent := &Agent{
		model: model,
		tools: make(map[string]Tool),
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
	s := &Stream{}

	s.iter = func(yield func(Event, error) bool) {
		defer func() { s.done = true }()

		for {
			assistantMsg, usage, err := a.callModel(ctx, instruction, msgs, yield)
			if err != nil {
				yield(Event{}, err)

				return
			}

			s.response.Usage.InputTokens += usage.InputTokens
			s.response.Usage.OutputTokens += usage.OutputTokens

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
		}
	}

	return s, nil
}

func (a *Agent) callModel(ctx context.Context, instruction string, msgs []Message, yield func(Event, error) bool) (Message, Usage, error) {
	req := ModelRequest{
		Instruction: instruction,
		Messages:    msgs,
		Tools:       a.toolDefinitions(),
		Config:      a.generationConfig,
	}

	if err := a.hooks.onModelRequest(ctx, req); err != nil {
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

	resp := stream.Response()
	if err := a.hooks.onModelResponse(ctx, resp); err != nil {
		return Message{}, Usage{}, fmt.Errorf("model response hook failed: %w", err)
	}

	return resp.Message, resp.Usage, nil
}

func (a *Agent) callTools(ctx context.Context, toolCalls []ToolCall, yield func(Event, error) bool) ([]Message, bool) {
	var msgs []Message

	for _, tc := range toolCalls {
		if !yield(NewToolCallEvent(tc), nil) {
			return msgs, false
		}

		content, err := a.callTool(ctx, tc)

		if !yield(NewToolResultEvent(ToolResult{ToolCallID: tc.ID, Content: content, IsError: err != nil}), nil) {
			return msgs, false
		}

		msgs = append(msgs, NewToolMessage(content, tc))
	}

	return msgs, true
}

func (a *Agent) callTool(ctx context.Context, toolCall ToolCall) (string, error) {
	if err := a.hooks.onToolCall(ctx, toolCall); err != nil {
		return "", err
	}

	t, ok := a.tools[toolCall.Name]
	if !ok {
		return "", fmt.Errorf("tool not found: %s", toolCall.Name)
	}

	result, err := t.Execute(ctx, toolCall.Arguments)
	if err != nil {
		result = err.Error()
	}

	if hookErr := a.hooks.onToolResult(ctx, toolCall, result, err); hookErr != nil {
		return "", hookErr
	}

	return result, err
}

func (a *Agent) toolDefinitions() []ToolDefinition {
	defs := make([]ToolDefinition, 0, len(a.tools))
	for _, tool := range a.tools {
		defs = append(defs, tool.Definition())
	}

	return defs
}

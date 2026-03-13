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

func (a *Agent) Run(ctx context.Context, messages []Message) (*Stream, error) {
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

			for _, tc := range assistantMsg.ToolCalls {
				if !yield(Event{Type: EventToolCall, ToolCall: tc}, nil) {
					return
				}

				content, isError := a.callTool(ctx, tc)
				toolResult := ToolResult{
					ToolCallID: tc.ID,
					Content:    content,
					IsError:    isError,
				}

				if !yield(Event{Type: EventToolResult, ToolResult: toolResult}, nil) {
					return
				}

				toolMsg := NewToolMessage(content, tc)
				msgs = append(msgs, toolMsg)
				s.response.Messages = append(s.response.Messages, toolMsg)
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
			if !yield(Event{Type: EventThinkingDelta, Text: chunk.Thinking}, nil) {
				break
			}
		}

		if chunk.Type == ChunkTypeText {
			if !yield(Event{Type: EventTextDelta, Text: chunk.Text}, nil) {
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

func (a *Agent) callTool(ctx context.Context, toolCall ToolCall) (string, bool) {
	if err := a.hooks.onToolCall(ctx, toolCall); err != nil {
		return fmt.Sprintf("error: %v", err), true
	}

	t, ok := a.tools[toolCall.Name]
	if !ok {
		return fmt.Sprintf("error: tool not found: %s", toolCall.Name), true
	}

	result, err := t.Execute(ctx, toolCall.Arguments)
	isError := err != nil

	if err != nil {
		result = fmt.Sprintf("error: %v", err)
	}

	if hookErr := a.hooks.onToolResult(ctx, toolCall, result, err); hookErr != nil {
		return fmt.Sprintf("error: %v", hookErr), true
	}

	return result, isError
}

func (a *Agent) toolDefinitions() []ToolDefinition {
	defs := make([]ToolDefinition, 0, len(a.tools))
	for _, tool := range a.tools {
		defs = append(defs, tool.Definition())
	}

	return defs
}

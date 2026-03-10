package kit

import (
	"context"
	"fmt"
	"slices"
)

type Agent struct {
	instructions string
	model        Model
	tools        map[string]Tool
}

func NewAgent(model Model, options ...AgentOptions) *Agent {
	agent := &Agent{
		model: model,
		tools: make(map[string]Tool),
	}

	for _, opt := range options {
		opt(agent)
	}

	return agent
}

func (a *Agent) Run(ctx context.Context, messages []Message) (Message, error) {
	msgs := slices.Clone(messages)

	for {
		req := ModelRequest{
			Instructions: a.instructions,
			Messages:     msgs,
			Tools:        a.toolDefinitions(),
		}

		resp, err := a.model.Generate(ctx, req)
		if err != nil {
			return Message{}, err
		}

		assistantMsg := NewAssistantMessage(resp.Content, resp.ToolCalls)
		msgs = append(msgs, assistantMsg)

		if len(resp.ToolCalls) == 0 {
			return assistantMsg, nil
		}

		toolMsgs := a.callTools(ctx, resp.ToolCalls)
		msgs = append(msgs, toolMsgs...)
	}
}

func (a *Agent) callTools(ctx context.Context, toolCalls []ToolCall) []Message {
	messages := make([]Message, 0, len(toolCalls))
	for _, toolCall := range toolCalls {
		result := a.CallTool(ctx, toolCall)
		messages = append(messages, NewToolMessage(result, toolCall))
	}

	return messages
}

func (a *Agent) CallTool(ctx context.Context, toolCall ToolCall) string {
	t, ok := a.tools[toolCall.Name]
	if !ok {
		return fmt.Sprintf("error: tool not found: %s", toolCall.Name)
	}

	result, err := t.Execute(ctx, toolCall.Arguments)
	if err != nil {
		result = fmt.Sprintf("error: %v", err)
	}

	return result
}

func (a *Agent) toolDefinitions() []ToolDefinition {
	defs := make([]ToolDefinition, 0, len(a.tools))
	for _, tool := range a.tools {
		defs = append(defs, tool.Definition())
	}

	return defs
}

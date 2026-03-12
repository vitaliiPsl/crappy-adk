package kit

import (
	"context"
	"fmt"
	"iter"
	"slices"
	"strings"
)

// Agent runs a ReAct loop: it calls the model, executes any requested tool
// calls, and feeds the results back until the model returns a final response.
type Agent struct {
	instructions string
	model        Model
	tools        map[string]Tool

	generationConfig GenerationConfig
}

// NewAgent creates an agent backed by the given model. Options are applied in order.
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

// Run executes the ReAct loop and streams the response as a sequence of [Event]
// values. Delta events carry incremental text tokens. A Message event is emitted
// each time a complete message is ready. Tool calls are handled transparently.
func (a *Agent) Run(ctx context.Context, messages []Message) iter.Seq2[Event, error] {
	return func(yield func(Event, error) bool) {
		msgs := slices.Clone(messages)

		for {
			req := ModelRequest{
				Instructions: a.instructions,
				Messages:     msgs,
				Tools:        a.toolDefinitions(),
				Config:       a.generationConfig,
			}

			assistantMsg, err := a.callModel(ctx, req, yield)
			if err != nil {
				yield(Event{}, err)

				return
			}

			msgs = append(msgs, assistantMsg)
			if !yield(newMessageEvent(&assistantMsg), nil) {
				return
			}

			if len(assistantMsg.ToolCalls) == 0 {
				return
			}

			toolMsgs := a.callTools(ctx, assistantMsg.ToolCalls)
			for i := range toolMsgs {
				if !yield(newMessageEvent(&toolMsgs[i]), nil) {
					return
				}
			}

			msgs = append(msgs, toolMsgs...)
		}
	}
}

// callModel streams a single model turn, yielding Delta events for each text
// or thinking token, and returns the assembled assistant message.
func (a *Agent) callModel(ctx context.Context, req ModelRequest, yield func(Event, error) bool) (Message, error) {
	var content, thinking strings.Builder
	var toolCalls []ToolCall

	for chunk, err := range a.model.GenerateStream(ctx, req) {
		if err != nil {
			return Message{}, err
		}

		if chunk.Thinking != "" {
			if !yield(newDeltaEvent("", chunk.Thinking), nil) {
				break
			}
			thinking.WriteString(chunk.Thinking)
		}

		if chunk.Text != "" {
			if !yield(newDeltaEvent(chunk.Text, ""), nil) {
				break
			}
			content.WriteString(chunk.Text)
		}

		toolCalls = append(toolCalls, chunk.ToolCalls...)
	}

	return NewAssistantMessage(content.String(), thinking.String(), toolCalls), nil
}

func (a *Agent) callTools(ctx context.Context, toolCalls []ToolCall) []Message {
	messages := make([]Message, 0, len(toolCalls))
	for _, toolCall := range toolCalls {
		result := a.callTool(ctx, toolCall)
		messages = append(messages, NewToolMessage(result, toolCall))
	}

	return messages
}

func (a *Agent) callTool(ctx context.Context, toolCall ToolCall) string {
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

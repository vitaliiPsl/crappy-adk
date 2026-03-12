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

// Run executes the ReAct loop and streams the response as a sequence of [Event]
// values. Delta events carry incremental text tokens. A Message event is emitted
// each time a complete message is ready. Tool calls are handled transparently.
func (a *Agent) Run(ctx context.Context, messages []Message) iter.Seq2[Event, error] {
	return func(yield func(Event, error) bool) {
		instruction, err := ComposeInstructions(ctx, "\n\n", a.instructions...)
		if err != nil {
			yield(Event{}, err)

			return
		}

		msgs := slices.Clone(messages)

		for {
			req := ModelRequest{
				Instruction: instruction,
				Messages:    msgs,
				Tools:       a.toolDefinitions(),
				Config:      a.generationConfig,
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
	if err := a.hooks.onModelRequest(ctx, req); err != nil {
		return Message{}, fmt.Errorf("model request hook failed: %w", err)
	}

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

	if err := a.hooks.onModelResponse(ctx, ModelResponse{
		Content:   content.String(),
		Thinking:  thinking.String(),
		ToolCalls: toolCalls,
	}); err != nil {
		return Message{}, fmt.Errorf("model response hook failed: %w", err)
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
	if err := a.hooks.onToolCall(ctx, toolCall); err != nil {
		return fmt.Sprintf("error: %v", err)
	}

	t, ok := a.tools[toolCall.Name]
	if !ok {
		return fmt.Sprintf("error: tool not found: %s", toolCall.Name)
	}

	result, err := t.Execute(ctx, toolCall.Arguments)
	if err != nil {
		result = fmt.Sprintf("error: %v", err)
	}

	if hookErr := a.hooks.onToolResult(ctx, toolCall, result, err); hookErr != nil {
		return fmt.Sprintf("error: %v", hookErr)
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

package agent

import (
	"context"
	"errors"
	"fmt"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

var _ kit.Agent = (*Agent)(nil)

// Agent runs a multi-turn conversation with a model, executing tools until
// the model produces a final response.
type Agent struct {
	config kit.AgentConfig
	output outputContract

	model  kit.Model
	memory kit.Memory
	tools  kit.ToolSet

	hooks hooks
}

// New creates a new Agent with the given model, memory, tools, and options.
func New(model kit.Model, memory kit.Memory, tools kit.ToolSet, options ...Option) (*Agent, error) {
	agent := &Agent{
		memory: memory,
		model:  model,
		tools:  tools,
	}

	for _, opt := range options {
		if err := opt(agent); err != nil {
			return nil, err
		}
	}

	return agent, nil
}

// Run executes the agentic loop and returns agent reponse when run finishes.
func (a *Agent) Run(ctx context.Context, input kit.Message) (kit.AgentResponse, error) {
	return a.run(ctx, input, kit.NoopEmitter[kit.AgentEvent]{})
}

// Stream executes the agentic loop and yields events as the run progresses.
func (a *Agent) Stream(ctx context.Context, input kit.Message) *kit.Stream[kit.AgentEvent, kit.AgentResponse] {
	return kit.NewStream(func(emit kit.Emitter[kit.AgentEvent]) (kit.AgentResponse, error) {
		return a.run(ctx, input, emit)
	})
}

func (a *Agent) run(
	ctx context.Context,
	input kit.Message,
	events kit.Emitter[kit.AgentEvent],
) (kit.AgentResponse, error) {
	if err := ctx.Err(); err != nil {
		return kit.AgentResponse{}, err
	}

	rc := &kit.RunContext{
		Context: ctx,
		Memory:  a.memory,
		Events:  events,
	}

	if err := a.memory.Record(ctx, input); err != nil {
		return rc.Response(), err
	}

	for {
		if err := rc.Err(); err != nil {
			return rc.Response(), err
		}

		if err := a.hooks.onTurnStart(rc); err != nil {
			return rc.Response(), err
		}

		resp, err := a.callModel(rc)
		if err != nil {
			return rc.Response(), err
		}

		rc.Append(resp.Message)
		rc.RecordUsage(resp.Usage)

		toolCalls := resp.Message.ToolCalls()

		if resp.FinishReason == kit.FinishReasonToolCall && len(toolCalls) == 0 {
			return rc.Response(), fmt.Errorf(
				"%w: finish reason requested tool calls but none were returned",
				kit.ErrInvalidOutput,
			)
		}

		if resp.FinishReason != kit.FinishReasonToolCall {
			if err := a.output.capture(rc, resp.Message); err != nil {
				return rc.Response(), err
			}

			if err := a.memory.Record(rc.Context, resp.Message); err != nil {
				return rc.Response(), err
			}

			if err := rc.Emit(kit.NewAgentMessageEvent(resp.Message)); err != nil {
				return rc.Response(), err
			}

			if err := a.hooks.onTurnEnd(rc); err != nil {
				return rc.Response(), err
			}

			return rc.Response(), nil
		}

		if err := rc.Emit(kit.NewAgentMessageEvent(resp.Message)); err != nil {
			return rc.Response(), err
		}

		results, err := a.executeTools(rc, toolCalls)
		if err != nil {
			return rc.Response(), err
		}

		toolMessage := kit.NewToolMessage(results...)
		rc.Append(toolMessage)

		if err := a.memory.Record(rc.Context, resp.Message, toolMessage); err != nil {
			return rc.Response(), err
		}

		if err := rc.Emit(kit.NewAgentMessageEvent(toolMessage)); err != nil {
			return rc.Response(), err
		}

		if err := a.hooks.onTurnEnd(rc); err != nil {
			return rc.Response(), err
		}
	}
}

func (a *Agent) callModel(rc *kit.RunContext) (kit.ModelResponse, error) {
	messages, err := a.memory.Context(rc.Context)
	if err != nil {
		return kit.ModelResponse{}, err
	}

	req := kit.ModelRequest{
		Instructions: a.config.Instructions,
		Messages:     messages,
		Tools:        a.tools.List(),
		OutputSchema: a.config.OutputSchema,
		Config: kit.GenerationConfig{
			Temperature:     a.config.Temperature,
			MaxOutputTokens: a.config.MaxOutputTokens,
			Thinking:        a.config.Thinking,
		},
	}

	req, err = a.hooks.onModelRequest(rc, req)
	if err != nil {
		return kit.ModelResponse{}, err
	}

	resp, err := a.streamModel(rc, req)
	if err != nil {
		return kit.ModelResponse{}, err
	}

	resp, err = a.hooks.onModelResponse(rc, resp)
	if err != nil {
		return kit.ModelResponse{}, err
	}

	return resp, nil
}

func (a *Agent) streamModel(rc *kit.RunContext, req kit.ModelRequest) (kit.ModelResponse, error) {
	stream := a.model.Stream(rc.Context, req)
	for event := range stream.Iter() {
		agentEvent, ok := kit.AgentEventFromModel(event)
		if !ok {
			continue
		}

		if err := rc.Emit(agentEvent); err != nil {
			return kit.ModelResponse{}, err
		}
	}

	return stream.Result()
}

func (a *Agent) executeTools(rc *kit.RunContext, toolCalls []kit.ToolCall) ([]kit.Content, error) {
	results := make([]kit.Content, 0, len(toolCalls))
	for _, call := range toolCalls {
		result, err := a.executeTool(rc, call)
		if err != nil {
			return nil, err
		}

		if err := rc.Emit(kit.NewAgentToolResultEvent(result)); err != nil {
			return nil, err
		}

		results = append(results, kit.NewToolResultContent(result))
	}

	return results, nil
}

func (a *Agent) executeTool(rc *kit.RunContext, call kit.ToolCall) (result kit.ToolResult, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			result = kit.NewToolResult(call, kit.ToolOutput{}, fmt.Errorf("tool %q panicked: %v", call.Name, recovered))
			err = nil
		}
	}()

	tool, ok := a.tools.Get(call.Name)
	if !ok {
		result = kit.NewToolResult(call, kit.ToolOutput{}, fmt.Errorf("tool %q not found", call.Name))

		return
	}

	hookedCall, hookErr := a.hooks.onToolCall(rc, call)
	if hookErr != nil {
		result = kit.NewToolResult(call, kit.ToolOutput{}, hookErr)

		return
	}

	call = hookedCall

	output, execErr := tool.Execute(rc, call)
	if errors.Is(execErr, context.Canceled) || errors.Is(execErr, context.DeadlineExceeded) {
		return kit.ToolResult{}, execErr
	}

	result = kit.NewToolResult(call, output, execErr)

	hookedResult, hookErr := a.hooks.onToolResult(rc, result)
	if hookErr != nil {
		result = kit.NewToolResult(call, kit.ToolOutput{}, hookErr)

		return
	}

	result = hookedResult

	return
}

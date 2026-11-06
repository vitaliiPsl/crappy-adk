package agent

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

var _ kit.Agent = (*Agent)(nil)

// Agent runs a multi-turn conversation with a model, executing tools until
// the model produces a final response.
type Agent struct {
	config kit.AgentConfig
	output outputContract

	model   kit.Model
	history kit.History

	instructions []kit.Instruction
	tools        []kit.Tool
	toolsByName  map[string]kit.Tool

	hooks hooks
}

// New creates an Agent that appends every message it produces to history.
// It returns an error if two tools share a name.
func New(model kit.Model, history kit.History, options ...Option) (*Agent, error) {
	agent := &Agent{
		model:   model,
		history: history,
	}

	for _, opt := range options {
		if err := opt(agent); err != nil {
			return nil, err
		}
	}

	agent.toolsByName = make(map[string]kit.Tool, len(agent.tools))
	for _, tool := range agent.tools {
		name := tool.Definition().Name
		if _, exists := agent.toolsByName[name]; exists {
			return nil, fmt.Errorf("duplicate tool %q", name)
		}

		agent.toolsByName[name] = tool
	}

	return agent, nil
}

// Context returns what the next model request would be built from.
func (a *Agent) Context(ctx context.Context) (kit.AgentContext, error) {
	messages, err := a.history.Messages(ctx)
	if err != nil {
		return kit.AgentContext{}, err
	}

	return kit.AgentContext{
		Instructions: append(kit.NewInstructions("", a.config.Instructions), a.instructions...),
		Messages:     messages,
		Tools:        slices.Clone(a.tools),
	}, nil
}

// Run appends input and runs the agent loop. With no input, it continues from history.
func (a *Agent) Run(ctx context.Context, input ...kit.Message) (kit.AgentResponse, error) {
	return a.run(ctx, input, kit.NoopEmitter[kit.AgentEvent]{})
}

// Stream runs the agent loop with events. With no input, it continues from history.
func (a *Agent) Stream(ctx context.Context, input ...kit.Message) *kit.Stream[kit.AgentEvent, kit.AgentResponse] {
	return kit.NewStream(func(emit kit.Emitter[kit.AgentEvent]) (kit.AgentResponse, error) {
		return a.run(ctx, input, emit)
	})
}

func (a *Agent) run(
	ctx context.Context,
	input []kit.Message,
	events kit.Emitter[kit.AgentEvent],
) (kit.AgentResponse, error) {
	if err := ctx.Err(); err != nil {
		return kit.AgentResponse{}, err
	}

	rc := &kit.RunContext{
		Context: ctx,
		Events:  events,
	}

	if len(input) > 0 {
		if err := a.history.Append(ctx, input...); err != nil {
			return rc.Response(), err
		}
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

		rc.RecordUsage(resp.Usage)
		rc.FinishReason = resp.FinishReason
		resp.Message.Usage = resp.Usage

		if err := a.record(rc, resp.Message, kit.NewAgentMessageEvent(resp.Message)); err != nil {
			return rc.Response(), err
		}

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

			if err := a.hooks.onTurnEnd(rc); err != nil {
				return rc.Response(), err
			}

			return rc.Response(), nil
		}

		if err := a.executeTools(rc, toolCalls); err != nil {
			return rc.Response(), err
		}

		if err := a.hooks.onTurnEnd(rc); err != nil {
			return rc.Response(), err
		}
	}
}

func (a *Agent) callModel(rc *kit.RunContext) (kit.ModelResponse, error) {
	req, err := a.prepareModelRequest(rc)
	if err != nil {
		return kit.ModelResponse{}, err
	}

	resp, err := a.streamModel(rc, req)
	if err != nil {
		if err := a.hooks.onModelError(rc, req, err); err != nil {
			return kit.ModelResponse{}, err
		}

		req, err = a.buildModelRequest(rc.Context)
		if err != nil {
			return kit.ModelResponse{}, err
		}

		resp, err = a.streamModel(rc, req)
	}

	if err != nil {
		return kit.ModelResponse{}, err
	}

	return a.hooks.onModelResponse(rc, resp)
}

func (a *Agent) prepareModelRequest(rc *kit.RunContext) (kit.ModelRequest, error) {
	req, err := a.buildModelRequest(rc.Context)
	if err != nil {
		return kit.ModelRequest{}, err
	}

	for _, hook := range a.hooks.modelRequestReady {
		rebuild, err := hook(rc, req)
		if err != nil {
			return kit.ModelRequest{}, err
		}

		if rebuild {
			req, err = a.buildModelRequest(rc.Context)
			if err != nil {
				return kit.ModelRequest{}, err
			}
		}
	}

	return req, nil
}

func (a *Agent) buildModelRequest(ctx context.Context) (kit.ModelRequest, error) {
	agentContext, err := a.Context(ctx)
	if err != nil {
		return kit.ModelRequest{}, err
	}

	return kit.ModelRequest{
		Instructions: agentContext.InstructionsText(),
		Messages:     agentContext.Messages,
		Tools:        agentContext.Tools,
		OutputSchema: a.config.OutputSchema,
		Config: kit.GenerationConfig{
			Temperature:     a.config.Temperature,
			MaxOutputTokens: a.config.MaxOutputTokens,
			Thinking:        a.config.Thinking,
		},
	}, nil
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

func (a *Agent) executeTools(rc *kit.RunContext, toolCalls []kit.ToolCall) error {
	for _, call := range toolCalls {
		result, err := a.executeTool(rc, call)
		if err != nil {
			return err
		}

		message := kit.NewToolMessage(kit.NewToolResultContent(result))
		if err := a.record(rc, message, kit.NewAgentToolResultEvent(result), kit.NewAgentMessageEvent(message)); err != nil {
			return err
		}
	}

	return nil
}

func (a *Agent) executeTool(rc *kit.RunContext, call kit.ToolCall) (result kit.ToolResult, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			result = kit.NewToolResult(call, kit.ToolOutput{}, fmt.Errorf("tool %q panicked: %v", call.Name, recovered))
			err = nil
		}
	}()

	tool, ok := a.toolsByName[call.Name]
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

func (a *Agent) record(rc *kit.RunContext, message kit.Message, events ...kit.AgentEvent) error {
	rc.Append(message)

	if err := a.history.Append(rc.Context, message); err != nil {
		return err
	}

	for _, event := range events {
		if err := rc.Emit(event); err != nil {
			return err
		}
	}

	return nil
}

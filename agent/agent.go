package agent

import (
	"context"
	"fmt"
	"slices"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

var _ kit.Agent = (*Agent)(nil)

// Agent runs a multi-turn conversation with a model, executing tools until
// the model produces a final response.
type Agent struct {
	config kit.AgentConfig

	model kit.Model

	tools     []kit.Tool
	toolIndex map[string]kit.Tool

	hooks hooks
}

// New creates a new Agent with the given model and options.
func New(model kit.Model, options ...Option) (*Agent, error) {
	agent := &Agent{
		model:     model,
		toolIndex: make(map[string]kit.Tool),
	}

	for _, opt := range options {
		if err := opt(agent); err != nil {
			return nil, err
		}
	}

	return agent, nil
}

// Run executes the agentic loop starting with the given messages.
// It calls the model repeatedly, executing any requested tools, until the model
// returns a finish reason other than FinishReasonToolCall.
func (a *Agent) Run(ctx context.Context, messages []kit.Message) (kit.AgentResponse, error) {
	return a.run(ctx, messages, kit.NoopEmitter[kit.AgentEvent]{})
}

// Stream executes the agentic loop and yields events as the run progresses.
func (a *Agent) Stream(ctx context.Context, messages []kit.Message) *kit.Stream[kit.AgentEvent, kit.AgentResponse] {
	return kit.NewStream(func(emit kit.Emitter[kit.AgentEvent]) (kit.AgentResponse, error) {
		return a.run(ctx, messages, emit)
	})
}

func (a *Agent) run(
	ctx context.Context,
	messages []kit.Message,
	events kit.Emitter[kit.AgentEvent],
) (kit.AgentResponse, error) {
	rc := &kit.RunContext{
		Context:  ctx,
		Messages: slices.Clone(messages),
		Events:   events,
	}

	for {
		if err := a.hooks.onTurnStart(rc); err != nil {
			return rc.Response(), err
		}

		resp, err := a.callModel(rc)
		if err != nil {
			return rc.Response(), err
		}

		rc.RecordUsage(resp.Usage)
		rc.Append(resp.Message)

		if resp.FinishReason != kit.FinishReasonToolCall {
			if err := a.hooks.onTurnEnd(rc); err != nil {
				return rc.Response(), err
			}

			return rc.Response(), nil
		}

		results, err := a.executeTools(rc, resp.Message.ToolCalls())
		if err != nil {
			return rc.Response(), err
		}

		toolMessage := kit.NewToolMessage(results)
		rc.Append(toolMessage)

		if err := rc.Emit(kit.NewAgentMessageEvent(toolMessage)); err != nil {
			return rc.Response(), err
		}

		if err := a.hooks.onTurnEnd(rc); err != nil {
			return rc.Response(), err
		}
	}
}

func (a *Agent) callModel(rc *kit.RunContext) (kit.ModelResponse, error) {
	req := kit.ModelRequest{
		Instructions: a.config.Instructions,
		Messages:     rc.Messages,
		Tools:        a.tools,
		Config: kit.GenerationConfig{
			Temperature:     a.config.Temperature,
			MaxOutputTokens: a.config.MaxOutputTokens,
			Thinking:        a.config.Thinking,
		},
	}

	req, err := a.hooks.onModelRequest(rc, req)
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
			result = kit.NewToolResult(call, "", fmt.Errorf("tool %q panicked: %v", call.Name, recovered))
			err = nil
		}
	}()

	tool, ok := a.toolIndex[call.Name]
	if !ok {
		result = kit.NewToolResult(call, "", fmt.Errorf("tool %q not found", call.Name))

		return
	}

	hookedCall, hookErr := a.hooks.onToolCall(rc, call)
	if hookErr != nil {
		result = kit.NewToolResult(call, "", hookErr)

		return
	}

	call = hookedCall

	output, execErr := tool.Execute(rc.Context, call.Arguments)
	result = kit.NewToolResult(call, output, execErr)

	hookedResult, hookErr := a.hooks.onToolResult(rc, result)
	if hookErr != nil {
		result = kit.NewToolResult(call, "", hookErr)

		return
	}

	result = hookedResult

	return
}

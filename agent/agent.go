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
	rc := &kit.RunContext{
		Context:  ctx,
		Messages: slices.Clone(messages),
	}

	for {
		rc.Turn++

		if err := a.hooks.onTurnStart(rc); err != nil {
			return kit.AgentResponse{}, err
		}

		resp, err := a.callModel(rc)
		if err != nil {
			return kit.AgentResponse{}, err
		}

		rc.RecordUsage(resp.Usage)
		rc.Append(resp.Message)

		if resp.FinishReason != kit.FinishReasonToolCall {
			if err := a.hooks.onTurnEnd(rc); err != nil {
				return kit.AgentResponse{}, err
			}

			return kit.AgentResponse{
				Messages: rc.Generated,
				Output:   resp.Message.TextContent(),
				Usage:    rc.Usage,
			}, nil
		}

		results, err := a.executeTools(rc, resp.Message.ToolCalls())
		if err != nil {
			return kit.AgentResponse{}, err
		}

		rc.Append(kit.NewToolMessage(results))

		if err := a.hooks.onTurnEnd(rc); err != nil {
			return kit.AgentResponse{}, err
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

	resp, err := a.model.Generate(rc.Context, req)
	if err != nil {
		return kit.ModelResponse{}, err
	}

	resp, err = a.hooks.onModelResponse(rc, resp)
	if err != nil {
		return kit.ModelResponse{}, err
	}

	return resp, nil
}

func (a *Agent) executeTools(rc *kit.RunContext, toolCalls []kit.ToolCall) ([]kit.Content, error) {
	results := make([]kit.Content, 0, len(toolCalls))
	for _, call := range toolCalls {
		result, err := a.executeTool(rc, call)
		if err != nil {
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

	call, err = a.hooks.onToolCall(rc, call)
	if err != nil {
		return
	}

	output, execErr := tool.Execute(rc.Context, call.Arguments)
	result = kit.NewToolResult(call, output, execErr)

	result, err = a.hooks.onToolResult(rc, result)

	return
}

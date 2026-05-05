package agent

import (
	"context"
	"fmt"

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

// New creates a new Agent with the given model, tools, and config.
// Additional options can be provided to further customize the agent's behavior.
// It would return an error if any of the options fail to apply.
func New(model kit.Model, tools []kit.Tool, config kit.AgentConfig, options ...Option) (*Agent, error) {
	idx := make(map[string]kit.Tool, len(tools))
	for _, t := range tools {
		idx[t.Definition().Name] = t
	}

	agent := &Agent{
		config:    config,
		model:     model,
		tools:     tools,
		toolIndex: idx,
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
	history := make([]kit.Message, len(messages))
	copy(history, messages)

	var usage kit.Usage

	for {
		ctx, resp, err := a.callModel(ctx, history)
		if err != nil {
			return kit.AgentResponse{}, err
		}

		usage.Add(resp.Usage)
		history = append(history, resp.Message)

		if resp.FinishReason != kit.FinishReasonToolCall {
			return kit.AgentResponse{
				Messages: history[len(messages):],
				Output:   resp.Message.TextContent(),
				Usage:    usage,
			}, nil
		}

		results, err := a.executeTools(ctx, resp.Message.ToolCalls())
		if err != nil {
			return kit.AgentResponse{}, err
		}

		history = append(history, kit.NewToolMessage(results))
	}
}

func (a *Agent) callModel(ctx context.Context, history []kit.Message) (context.Context, kit.ModelResponse, error) {
	req := kit.ModelRequest{
		Instructions: a.config.Instructions,
		Messages:     history,
		Tools:        a.tools,
		Config: kit.GenerationConfig{
			Temperature:     a.config.Temperature,
			MaxOutputTokens: a.config.MaxOutputTokens,
			Thinking:        a.config.Thinking,
		},
	}

	var err error

	ctx, req, err = a.hooks.onModelRequest(ctx, req)
	if err != nil {
		return ctx, kit.ModelResponse{}, err
	}

	resp, err := a.model.Generate(ctx, req)
	if err != nil {
		return ctx, kit.ModelResponse{}, err
	}

	ctx, resp, err = a.hooks.onModelResponse(ctx, resp)
	if err != nil {
		return ctx, kit.ModelResponse{}, err
	}

	return ctx, resp, nil
}

func (a *Agent) executeTools(ctx context.Context, toolCalls []kit.ToolCall) ([]kit.Content, error) {
	results := make([]kit.Content, 0, len(toolCalls))
	for _, call := range toolCalls {
		result, err := a.executeTool(ctx, call)
		if err != nil {
			return nil, err
		}

		results = append(results, kit.NewToolResultContent(result))
	}

	return results, nil
}

func (a *Agent) executeTool(ctx context.Context, call kit.ToolCall) (result kit.ToolResult, err error) {
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

	ctx, call, err = a.hooks.onToolCall(ctx, call)
	if err != nil {
		return
	}

	output, execErr := tool.Execute(ctx, call.Arguments)
	result = kit.NewToolResult(call, output, execErr)

	_, result, err = a.hooks.onToolResult(ctx, result)

	return
}

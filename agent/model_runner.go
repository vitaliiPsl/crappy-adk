package agent

import (
	"context"
	"fmt"

	"github.com/vitaliiPsl/crappy-adk/kit"
	"github.com/vitaliiPsl/crappy-adk/x/stream"
)

type modelRunner struct {
	model           kit.Model
	toolDefinitions []kit.ToolDefinition
	hooks           *hooks
	config          *kit.AgentConfig
}

func newModelRunner(model kit.Model, toolDefinitions []kit.ToolDefinition, hooks *hooks, config *kit.AgentConfig) *modelRunner {
	return &modelRunner{
		model:           model,
		toolDefinitions: toolDefinitions,
		hooks:           hooks,
		config:          config,
	}
}

func (r *modelRunner) run(
	ctx context.Context,
	msgs []kit.Message,
	e *stream.Emitter[kit.Event],
) (kit.ModelResponse, error) {
	req := kit.ModelRequest{
		Instruction:    r.config.SystemPrompt,
		Messages:       msgs,
		Tools:          r.toolDefinitions,
		ResponseSchema: r.config.ResponseSchema,
		Config: kit.GenerationConfig{
			Temperature:     r.config.Temperature,
			TopP:            r.config.TopP,
			MaxOutputTokens: r.config.MaxOutputTokens,
			Thinking:        r.config.Thinking,
		},
	}

	ctx, req, err := r.hooks.onModelRequest(ctx, req)
	if err != nil {
		return kit.ModelResponse{}, fmt.Errorf("model request hook failed: %w", err)
	}

	modelStream, err := r.model.GenerateStream(ctx, req)
	if err != nil {
		return kit.ModelResponse{}, err
	}

	for ev := range modelStream.Iter() {
		if err := e.Emit(ev); err != nil {
			return kit.ModelResponse{}, err
		}
	}

	modelResp, streamErr := modelStream.Result()
	if streamErr != nil {
		return kit.ModelResponse{}, streamErr
	}

	_, resp, err := r.hooks.onModelResponse(ctx, modelResp)
	if err != nil {
		return kit.ModelResponse{}, fmt.Errorf("model response hook failed: %w", err)
	}

	if err := e.Emit(kit.NewMessageEvent(resp.Message)); err != nil {
		return kit.ModelResponse{}, err
	}

	return resp, nil
}

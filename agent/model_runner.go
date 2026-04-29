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
	config          *Config
}

func newModelRunner(model kit.Model, toolDefinitions []kit.ToolDefinition, hooks *hooks, config *Config) *modelRunner {
	return &modelRunner{
		model:           model,
		toolDefinitions: toolDefinitions,
		hooks:           hooks,
		config:          config,
	}
}

func (r *modelRunner) run(
	ctx context.Context,
	instruction string,
	msgs []kit.Message,
	e *stream.Emitter[kit.Event],
) (kit.ModelResponse, error) {
	req := kit.ModelRequest{
		Instruction:    instruction,
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

	stream, err := r.model.GenerateStream(ctx, req)
	if err != nil {
		return kit.ModelResponse{}, err
	}

	if err := r.forwardEvents(stream, e); err != nil {
		return kit.ModelResponse{}, err
	}

	modelResp, streamErr := stream.Result()
	if streamErr != nil {
		return kit.ModelResponse{}, streamErr
	}

	_, resp, err := r.hooks.onModelResponse(ctx, modelResp)
	if err != nil {
		return kit.ModelResponse{}, fmt.Errorf("model response hook failed: %w", err)
	}

	return resp, nil
}

func (r *modelRunner) forwardEvents(
	modelStream *stream.Stream[kit.ModelEvent, kit.ModelResponse],
	e *stream.Emitter[kit.Event],
) error {
	for ev := range modelStream.Iter() {
		event, ok := eventFromModelEvent(ev)
		if !ok {
			continue
		}

		if err := e.Emit(event); err != nil {
			return err
		}
	}

	return nil
}

func eventFromModelEvent(ev kit.ModelEvent) (kit.Event, bool) {
	switch ev.Type {
	case kit.ModelEventContentPartStarted:
		return kit.NewContentPartStartedEvent(ev.ContentPartType), true
	case kit.ModelEventContentPartDelta:
		return kit.NewContentPartDeltaEvent(ev.ContentPartType, ev.Text), true
	case kit.ModelEventContentPartDone:
		if ev.ContentPart == nil {
			return kit.Event{}, false
		}

		return kit.NewContentPartDoneEvent(*ev.ContentPart), true
	default:
		return kit.Event{}, false
	}
}

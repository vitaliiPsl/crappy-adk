package agent

import (
	"context"
	"fmt"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

type modelRunner struct {
	model           kit.Model
	toolDefinitions []kit.ToolDefinition
	hooks           *hooks
	config          *Config
}

func (r *modelRunner) run(
	ctx context.Context,
	instruction string,
	msgs []kit.Message,
	yield func(kit.Event, error) bool,
) (kit.Message, kit.Usage, error) {
	req := kit.ModelRequest{
		Instruction: instruction,
		Messages:    msgs,
		Tools:       r.toolDefinitions,
		Config: kit.GenerationConfig{
			Temperature:     r.config.Temperature,
			TopP:            r.config.TopP,
			MaxOutputTokens: r.config.MaxOutputTokens,
			Thinking:        r.config.Thinking,
		},
	}

	ctx, req, err := r.hooks.onModelRequest(ctx, req)
	if err != nil {
		return kit.Message{}, kit.Usage{}, fmt.Errorf("model request hook failed: %w", err)
	}

	stream, err := r.model.GenerateStream(ctx, req)
	if err != nil {
		return kit.Message{}, kit.Usage{}, err
	}

	if err := r.forwardEvents(stream, yield); err != nil {
		return kit.Message{}, kit.Usage{}, err
	}

	modelResp, streamErr := stream.Result()
	if streamErr != nil {
		return kit.Message{}, kit.Usage{}, streamErr
	}

	_, resp, err := r.hooks.onModelResponse(ctx, modelResp)
	if err != nil {
		return kit.Message{}, kit.Usage{}, fmt.Errorf("model response hook failed: %w", err)
	}

	return resp.Message, resp.Usage, nil
}

func (r *modelRunner) forwardEvents(
	stream *kit.Stream[kit.ModelEvent, kit.ModelResponse],
	yield func(kit.Event, error) bool,
) error {
	for ev, err := range stream.Iter() {
		if err != nil {
			return err
		}

		event, ok := eventFromModelEvent(ev)
		if !ok {
			continue
		}

		if !yield(event, nil) {
			return nil
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

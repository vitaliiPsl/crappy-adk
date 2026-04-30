package agent

import (
	"context"
	"errors"
	"fmt"

	"github.com/vitaliiPsl/crappy-adk/kit"
	"github.com/vitaliiPsl/crappy-adk/x/stream"
)

const (
	defaultCompactionThreshold = 0.8
)

// Agent runs a ReAct loop: it calls the model, executes any requested tool
// calls, and feeds the results back until the model returns a final response.
type Agent struct {
	config kit.AgentConfig
	model  kit.Model

	registry     *toolRegistry
	toolExecutor *toolExecutor

	compactor kit.Compactor
	hooks     hooks
}

// New creates an agent backed by the given model. Options are applied in order.
func New(model kit.Model, options ...Option) (*Agent, error) {
	a := &Agent{
		model:    model,
		registry: newToolRegistry(),
		config: kit.AgentConfig{
			CompactionThreshold: defaultCompactionThreshold,
		},
	}

	for _, opt := range options {
		if err := opt(a); err != nil {
			return nil, err
		}
	}

	a.toolExecutor = newToolExecutor(a.registry, &a.hooks)

	return a, nil
}

// Name returns the agent's name.
func (a *Agent) Name() string { return a.config.Name }

// Description returns the agent's description.
func (a *Agent) Description() string { return a.config.Description }

// Run executes the ReAct loop and returns the accumulated [kit.Result] once the
// agent reaches a final answer. Use [Agent.Stream] instead to receive
// incremental events as the agent works.
func (a *Agent) Run(ctx context.Context, messages []kit.Message) (kit.Result, error) {
	s, err := a.Stream(ctx, messages)
	if err != nil {
		return kit.Result{}, err
	}

	return s.Result()
}

// Stream executes the ReAct loop and returns a [kit.Stream] that emits
// incremental [kit.Event] values — text deltas, thinking deltas, tool calls,
// and tool results — as the agent works. Call [kit.Stream.Result] after
// iteration to retrieve the accumulated [kit.Result].
func (a *Agent) Stream(ctx context.Context, msgs []kit.Message) (*stream.Stream[kit.Event, kit.Result], error) {
	return stream.New(func(e *stream.Emitter[kit.Event]) (kit.Result, error) {
		ctx, msgs, err := a.hooks.onRunStart(ctx, msgs)
		if err != nil {
			return kit.Result{}, err
		}

		result, runErr := a.runLoop(ctx, msgs, e)

		_, hookErr := a.hooks.onRunEnd(ctx, result, runErr)
		if hookErr != nil {
			return result, hookErr
		}

		return result, runErr
	}), nil
}

func (a *Agent) runLoop(
	ctx context.Context,
	msgs []kit.Message,
	e *stream.Emitter[kit.Event],
) (kit.Result, error) {
	var err error

	state := newRunState(msgs)

	for {
		if err := ctx.Err(); err != nil {
			return state.result(), err
		}

		ctx, msgs, err = a.hooks.onTurnStart(ctx, state.messages())
		if err != nil {
			return state.result(), err
		}

		state.setMessages(msgs)

		modelResp, err := a.runModelTurn(ctx, state, e)
		if err != nil {
			if errors.Is(err, kit.ErrContextLength) && a.compactor != nil {
				if err := a.compact(ctx, state, e); err != nil {
					return state.result(), err
				}

				continue
			}

			return state.result(), err
		}

		if isFinalResponse(modelResp) {
			state.recordFinalResponse(modelResp)

			return state.result(), nil
		}

		if err := a.runToolTurn(ctx, state, modelResp.Message.ToolCalls(), e); err != nil {
			return state.result(), err
		}

		if err := a.runCompactionTurn(ctx, state, modelResp.Usage, e); err != nil {
			return state.result(), err
		}

		ctx, err = a.hooks.onTurnEnd(ctx, state.messages())
		if err != nil {
			return state.result(), err
		}
	}
}

func (a *Agent) runModelTurn(
	ctx context.Context,
	state *runState,
	e *stream.Emitter[kit.Event],
) (kit.ModelResponse, error) {
	req := a.buildModelRequest(state.messages())

	ctx, req, err := a.hooks.onModelRequest(ctx, req)
	if err != nil {
		return kit.ModelResponse{}, fmt.Errorf("model request hook failed: %w", err)
	}

	modelResp, err := a.generateModelResponse(ctx, req, e)
	if err != nil {
		return kit.ModelResponse{}, err
	}

	_, modelResp, err = a.hooks.onModelResponse(ctx, modelResp)
	if err != nil {
		return kit.ModelResponse{}, fmt.Errorf("model response hook failed: %w", err)
	}

	if err := e.Emit(kit.NewMessageEvent(modelResp.Message)); err != nil {
		return kit.ModelResponse{}, err
	}

	state.recordModelResponse(modelResp)

	return modelResp, nil
}

func (a *Agent) buildModelRequest(msgs []kit.Message) kit.ModelRequest {
	return kit.ModelRequest{
		Instruction:    a.config.SystemPrompt,
		Messages:       msgs,
		Tools:          a.registry.definitions(),
		ResponseSchema: a.config.ResponseSchema,
		Config: kit.GenerationConfig{
			Temperature:     a.config.Temperature,
			TopP:            a.config.TopP,
			MaxOutputTokens: a.config.MaxOutputTokens,
			Thinking:        a.config.Thinking,
		},
	}
}

func (a *Agent) generateModelResponse(
	ctx context.Context,
	req kit.ModelRequest,
	e *stream.Emitter[kit.Event],
) (kit.ModelResponse, error) {
	modelStream, err := a.model.GenerateStream(ctx, req)
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

	return modelResp, nil
}

func (a *Agent) runToolTurn(
	ctx context.Context,
	state *runState,
	toolCalls []kit.ToolCall,
	e *stream.Emitter[kit.Event],
) error {
	toolMsgs, err := a.toolExecutor.run(ctx, toolCalls, e)
	if err != nil {
		return err
	}

	state.recordToolMessages(toolMsgs)

	return nil
}

func isFinalResponse(resp kit.ModelResponse) bool {
	return len(resp.Message.ToolCalls()) == 0
}

func (a *Agent) runCompactionTurn(
	ctx context.Context,
	state *runState,
	usage kit.Usage,
	e *stream.Emitter[kit.Event],
) error {
	if !a.needsCompaction(usage) {
		return nil
	}

	return a.compact(ctx, state, e)
}

func (a *Agent) needsCompaction(lastUsage kit.Usage) bool {
	if a.compactor == nil {
		return false
	}

	inputLimit := a.model.Config().InputLimit
	if inputLimit <= 0 {
		return false
	}

	used := int64(lastUsage.InputTokens) + int64(lastUsage.OutputTokens)

	return used > int64(float64(inputLimit)*a.config.CompactionThreshold)
}

func (a *Agent) compact(
	ctx context.Context,
	state *runState,
	e *stream.Emitter[kit.Event],
) error {
	compacted, summary, err := a.compactor.Compact(ctx, state.messages())
	if err != nil {
		return fmt.Errorf("compactor failed: %w", err)
	}

	if summary == "" {
		state.recordCompaction(compacted, nil)

		return nil
	}

	summaryMsg := kit.NewSummaryMessage(summary)

	if err := e.Emit(kit.NewCompactionDoneEvent(summary)); err != nil {
		return err
	}

	if err := e.Emit(kit.NewMessageEvent(summaryMsg)); err != nil {
		return err
	}

	state.recordCompaction(compacted, &summaryMsg)

	return nil
}

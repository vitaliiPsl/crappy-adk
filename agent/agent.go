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

	tools           map[string]kit.Tool
	toolDefinitions []kit.ToolDefinition

	compactor         kit.Compactor
	hooks             hooks
	executionStrategy toolExecutionStrategy

	modelRunner  *modelRunner
	toolExecutor *toolExecutor
}

// New creates an agent backed by the given model. Options are applied in order.
func New(model kit.Model, options ...Option) (*Agent, error) {
	a := &Agent{
		model: model,
		tools: make(map[string]kit.Tool),
		config: kit.AgentConfig{
			CompactionThreshold: defaultCompactionThreshold,
		},
	}

	for _, opt := range options {
		if err := opt(a); err != nil {
			return nil, err
		}
	}

	if a.executionStrategy == nil {
		a.executionStrategy = parallelStrategy{}
	}

	a.modelRunner = newModelRunner(a.model, a.toolDefinitions, &a.hooks, &a.config)
	a.toolExecutor = newToolExecutor(a.tools, &a.hooks, a.executionStrategy)

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

		response, runErr := a.runLoop(ctx, msgs, e)

		_, hookErr := a.hooks.onRunEnd(ctx, response, runErr)
		if hookErr != nil {
			return response, hookErr
		}

		return response, runErr
	}), nil
}

// TODO: really need to consider separate runner component with state,
// so there is no need to carry so much data through input params.
// Also don't like the response mutation.
func (a *Agent) runLoop(
	ctx context.Context,
	msgs []kit.Message,
	e *stream.Emitter[kit.Event],
) (response kit.Result, err error) {
	for {
		if err := ctx.Err(); err != nil {
			return response, err
		}

		ctx, msgs, err = a.hooks.onTurnStart(ctx, msgs)
		if err != nil {
			return response, err
		}

		modelResp, err := a.runModelTurn(ctx, msgs, &response, e)
		if err != nil {
			if errors.Is(err, kit.ErrContextLength) && a.compactor != nil {
				msgs, err = a.compact(ctx, msgs, &response, e)
				if err != nil {
					return response, err
				}

				continue
			}

			return response, err
		}

		msgs = append(msgs, modelResp.Message)

		done := a.tryExit(modelResp, &response)
		if done {
			return response, nil
		}

		toolMsgs, err := a.runToolTurn(ctx, modelResp.Message.ToolCalls(), &response, e)
		if err != nil {
			return response, err
		}

		msgs = append(msgs, toolMsgs...)

		msgs, err = a.runCompactionTurn(ctx, msgs, modelResp.Usage, &response, e)
		if err != nil {
			return response, err
		}

		ctx, err = a.hooks.onTurnEnd(ctx, msgs)
		if err != nil {
			return response, err
		}
	}
}

func (a *Agent) runModelTurn(
	ctx context.Context,
	msgs []kit.Message,
	response *kit.Result,
	e *stream.Emitter[kit.Event],
) (kit.ModelResponse, error) {
	modelResp, err := a.modelRunner.run(ctx, msgs, e)
	if err != nil {
		return kit.ModelResponse{}, err
	}

	response.Messages = append(response.Messages, modelResp.Message)
	response.Usage.Add(modelResp.Usage)
	response.LastUsage = modelResp.Usage

	return modelResp, nil
}

func (a *Agent) tryExit(modelResp kit.ModelResponse, response *kit.Result) bool {
	assistantMsg := modelResp.Message
	if len(assistantMsg.ToolCalls()) > 0 {
		return false
	}

	response.Output = assistantMsg.Output()
	response.StructuredOutput = modelResp.StructuredOutput

	return true
}

func (a *Agent) runToolTurn(
	ctx context.Context,
	toolCalls []kit.ToolCall,
	response *kit.Result,
	e *stream.Emitter[kit.Event],
) ([]kit.Message, error) {
	toolMsgs, err := a.toolExecutor.run(ctx, toolCalls, e)
	if err != nil {
		return nil, err
	}

	response.Messages = append(response.Messages, toolMsgs...)

	return toolMsgs, nil
}

func (a *Agent) runCompactionTurn(
	ctx context.Context,
	msgs []kit.Message,
	usage kit.Usage,
	response *kit.Result,
	e *stream.Emitter[kit.Event],
) ([]kit.Message, error) {
	if !a.needsCompaction(usage) {
		return msgs, nil
	}

	return a.compact(ctx, msgs, response, e)
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
	msgs []kit.Message,
	response *kit.Result,
	e *stream.Emitter[kit.Event],
) ([]kit.Message, error) {
	compacted, summary, err := a.compactor.Compact(ctx, msgs)
	if err != nil {
		return msgs, fmt.Errorf("compactor failed: %w", err)
	}

	if summary == "" {
		return msgs, nil
	}

	summaryMsg := kit.NewSummaryMessage(summary)

	if err := e.Emit(kit.NewCompactionDoneEvent(summary)); err != nil {
		return msgs, err
	}

	if err := e.Emit(kit.NewMessageEvent(summaryMsg)); err != nil {
		return msgs, err
	}

	response.Messages = append(response.Messages, summaryMsg)

	return compacted, nil
}

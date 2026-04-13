package agent

import (
	"context"
	"fmt"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

const (
	defaultCompactionThreshold = 0.8
)

// Config holds configuration for an [Agent].
type Config struct {
	// Generation controls generation parameters used on every model request.
	Generation kit.GenerationConfig

	// ToolExecution controls whether tool calls run in parallel or sequentially.
	// Defaults to ToolExecutionParallel.
	ToolExecution ToolExecutionMode

	// CompactionThreshold is the fraction of the context window that triggers
	// compaction. Defaults to 0.8 when zero.
	CompactionThreshold float64
}

// Agent runs a ReAct loop: it calls the model, executes any requested tool
// calls, and feeds the results back until the model returns a final response.
type Agent struct {
	config       Config
	instructions []kit.Instruction

	model kit.Model

	tools           map[string]kit.Tool
	toolDefinitions []kit.ToolDefinition

	compactor kit.Compactor
	hooks     hooks
}

// New creates an agent backed by the given model. Options are applied in order.
func New(model kit.Model, options ...Option) (*Agent, error) {
	agent := &Agent{
		model: model,
		tools: make(map[string]kit.Tool),
		config: Config{
			CompactionThreshold: defaultCompactionThreshold,
		},
	}

	for _, opt := range options {
		if err := opt(agent); err != nil {
			return nil, err
		}
	}

	return agent, nil
}

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
func (a *Agent) Stream(ctx context.Context, msgs []kit.Message) (*kit.Stream[kit.Event, kit.Result], error) {
	instruction, err := kit.ComposeInstructions(ctx, "\n\n", a.instructions...)
	if err != nil {
		return nil, err
	}

	ctx, msgs, err = a.hooks.onRunStart(ctx, msgs)
	if err != nil {
		return nil, err
	}

	return kit.NewStream(func(yield func(kit.Event, error) bool) kit.Result {
		response, runErr := a.runLoop(ctx, instruction, msgs, yield)

		if _, hookErr := a.hooks.onRunEnd(ctx, response, runErr); hookErr != nil {
			yield(kit.Event{}, hookErr)

			return response
		}

		if runErr != nil {
			yield(kit.Event{}, runErr)
		}

		return response
	}), nil
}

func (a *Agent) runLoop(
	ctx context.Context,
	instruction string,
	msgs []kit.Message,
	yield func(kit.Event, error) bool,
) (response kit.Result, err error) {
	for {
		if err := ctx.Err(); err != nil {
			return response, err
		}

		ctx, msgs, err = a.hooks.onTurnStart(ctx, msgs)
		if err != nil {
			return response, err
		}

		modelRunner := a.modelRunner()

		assistantMsg, usage, err := modelRunner.run(ctx, instruction, msgs, yield)
		if err != nil {
			return response, err
		}

		msgs = append(msgs, assistantMsg)
		response.Messages = append(response.Messages, assistantMsg)

		response.Usage.Add(usage)
		response.LastUsage = usage

		if !yield(kit.NewMessageEvent(assistantMsg), nil) {
			return response, nil
		}

		if len(assistantMsg.ToolCalls) == 0 {
			if len(assistantMsg.Content) > 0 {
				response.Output = assistantMsg.Content[0]
			}

			return response, nil
		}

		toolRunner := a.toolRunner()

		toolMsgs, ok := toolRunner.run(ctx, assistantMsg.ToolCalls, yield)
		if !ok {
			return response, nil
		}

		msgs = append(msgs, toolMsgs...)
		response.Messages = append(response.Messages, toolMsgs...)

		if a.needsCompaction(usage) {
			compacted, summaryMsg, ok := a.compact(ctx, msgs, yield)
			if !ok {
				return response, nil
			}

			msgs = compacted

			if summaryMsg != nil {
				response.Messages = append(response.Messages, *summaryMsg)
			}
		}

		ctx, err = a.hooks.onTurnEnd(ctx, msgs)
		if err != nil {
			return response, err
		}
	}
}

func (a *Agent) modelRunner() modelRunner {
	return modelRunner{
		model:           a.model,
		toolDefinitions: a.toolDefinitions,
		hooks:           &a.hooks,
		config:          &a.config,
	}
}

func (a *Agent) toolRunner() toolRunner {
	return toolRunner{
		tools:  a.tools,
		hooks:  &a.hooks,
		config: &a.config,
	}
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
	yield func(kit.Event, error) bool,
) ([]kit.Message, *kit.Message, bool) {
	compacted, summary, err := a.compactor.Compact(ctx, msgs)
	if err != nil {
		yield(kit.Event{}, fmt.Errorf("compactor failed: %w", err))

		return msgs, nil, false
	}

	if summary == "" {
		return msgs, nil, true
	}

	summaryMsg := kit.NewSummaryMessage(summary)

	if !yield(kit.NewCompactionDoneEvent(summary), nil) {
		return msgs, nil, false
	}

	if !yield(kit.NewMessageEvent(summaryMsg), nil) {
		return msgs, nil, false
	}

	return compacted, &summaryMsg, true
}

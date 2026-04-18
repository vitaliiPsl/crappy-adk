package agent

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

const (
	defaultCompactionThreshold = 0.8
)

// Config holds configuration for an [Agent].
type Config struct {
	// Instructions are composed into the system prompt on each run.
	Instructions []kit.Instruction

	// Temperature controls randomness. Nil uses the model default.
	Temperature *float32

	// TopP limits sampling to the smallest set of tokens whose cumulative
	// probability meets this threshold. Nil uses the model default.
	TopP *float32

	// MaxOutputTokens limits the number of tokens the model can generate.
	// Nil uses the model default.
	MaxOutputTokens *int32

	// ResponseSchema constrains the final assistant answer to JSON matching this schema.
	ResponseSchema *jsonschema.Schema

	// Thinking controls extended thinking. Empty disables it.
	Thinking kit.ThinkingLevel

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
	config Config
	model  kit.Model

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
	instruction, err := kit.ComposeInstructions(ctx, "\n\n", a.config.Instructions...)
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

// TODO: really need to consider separate runner component with state,
// so there is no need to carry so much data through input params.
// Also don't like the response mutation.
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

		modelResp, ok, err := a.runModelTurn(ctx, instruction, msgs, &response, yield)
		if err != nil {
			if errors.Is(err, kit.ErrContextLength) && a.compactor != nil {
				var compacted bool

				msgs, compacted = a.compact(ctx, msgs, &response, yield)
				if compacted {
					continue
				}
			}

			return response, err
		}

		if !ok {
			return response, nil
		}

		msgs = append(msgs, modelResp.Message)

		done := a.tryExit(modelResp, &response)
		if done {
			return response, nil
		}

		toolMsgs, ok := a.runToolTurn(ctx, modelResp.Message.ToolCalls(), &response, yield)
		if !ok {
			return response, nil
		}

		msgs = append(msgs, toolMsgs...)

		msgs, ok = a.runCompactionTurn(ctx, msgs, modelResp.Usage, &response, yield)
		if !ok {
			return response, nil
		}

		ctx, err = a.hooks.onTurnEnd(ctx, msgs)
		if err != nil {
			return response, err
		}
	}
}

func (a *Agent) runModelTurn(
	ctx context.Context,
	instruction string,
	msgs []kit.Message,
	response *kit.Result,
	yield func(kit.Event, error) bool,
) (kit.ModelResponse, bool, error) {
	modelRunner := a.modelRunner()

	modelResp, err := modelRunner.run(ctx, instruction, msgs, yield)
	if err != nil {
		return kit.ModelResponse{}, false, err
	}

	response.Messages = append(response.Messages, modelResp.Message)
	response.Usage.Add(modelResp.Usage)
	response.LastUsage = modelResp.Usage

	if !yield(kit.NewMessageEvent(modelResp.Message), nil) {
		return kit.ModelResponse{}, false, nil
	}

	return modelResp, true, nil
}

func (a *Agent) tryExit(modelResp kit.ModelResponse, response *kit.Result) bool {
	assistantMsg := modelResp.Message
	if len(assistantMsg.ToolCalls()) > 0 {
		return false
	}

	if len(assistantMsg.Content) > 0 {
		response.Output = assistantMsg.Content[0]
	}

	response.StructuredOutput = modelResp.StructuredOutput

	return true
}

func (a *Agent) runToolTurn(
	ctx context.Context,
	toolCalls []kit.ToolCall,
	response *kit.Result,
	yield func(kit.Event, error) bool,
) ([]kit.Message, bool) {
	toolRunner := a.toolRunner()

	toolMsgs, ok := toolRunner.run(ctx, toolCalls, yield)
	if !ok {
		return nil, false
	}

	response.Messages = append(response.Messages, toolMsgs...)

	return toolMsgs, true
}

func (a *Agent) runCompactionTurn(
	ctx context.Context,
	msgs []kit.Message,
	usage kit.Usage,
	response *kit.Result,
	yield func(kit.Event, error) bool,
) ([]kit.Message, bool) {
	if !a.needsCompaction(usage) {
		return msgs, true
	}

	return a.compact(ctx, msgs, response, yield)
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
	yield func(kit.Event, error) bool,
) ([]kit.Message, bool) {
	compacted, summary, err := a.compactor.Compact(ctx, msgs)
	if err != nil {
		yield(kit.Event{}, fmt.Errorf("compactor failed: %w", err))

		return msgs, false
	}

	if summary == "" {
		return msgs, true
	}

	summaryMsg := kit.NewSummaryMessage(summary)

	if !yield(kit.NewCompactionDoneEvent(summary), nil) {
		return msgs, false
	}

	if !yield(kit.NewMessageEvent(summaryMsg), nil) {
		return msgs, false
	}

	response.Messages = append(response.Messages, summaryMsg)

	return compacted, true
}

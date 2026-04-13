package agent

import (
	"context"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

// ToolExecutionMode controls how multiple tool calls in a single turn are executed.
type ToolExecutionMode string

const (
	// ToolExecutionParallel executes tool calls concurrently. This is the default.
	ToolExecutionParallel ToolExecutionMode = "parallel"
	// ToolExecutionSequential executes tool calls one at a time, in order.
	ToolExecutionSequential ToolExecutionMode = "sequential"
)

// Option is a functional option for configuring an [Agent].
type Option func(*Agent) error

// WithInstruction adds a static string to the agent's system prompt.
func WithInstruction(text string) Option {
	return WithInstructions(func(_ context.Context) (string, error) {
		return text, nil
	})
}

// WithInstructions appends one or more [kit.Instruction] values to the
// agent's system prompt. Sources are evaluated in order on each [Agent.Run].
func WithInstructions(sources ...kit.Instruction) Option {
	return func(a *Agent) error {
		a.instructions = append(a.instructions, sources...)

		return nil
	}
}

// WithParallelToolExecution sets tool execution to the parallel mode.
func WithParallelToolExecution() Option {
	return func(a *Agent) error {
		a.config.ToolExecution = ToolExecutionParallel

		return nil
	}
}

// WithSequentialToolExecution sets tool execution to the sequential mode.
func WithSequentialToolExecution() Option {
	return func(a *Agent) error {
		a.config.ToolExecution = ToolExecutionSequential

		return nil
	}
}

// WithGenerationConfig sets the generation parameters used on every model request.
func WithGenerationConfig(config kit.GenerationConfig) Option {
	return func(a *Agent) error {
		a.config.Generation = config

		return nil
	}
}

// WithTool registers a single tool with the agent.
func WithTool(tool kit.Tool) Option {
	return func(a *Agent) error {
		registerTool(a, tool)

		return nil
	}
}

// WithTools registers multiple tools with the agent.
func WithTools(tools ...kit.Tool) Option {
	return func(a *Agent) error {
		for _, tool := range tools {
			registerTool(a, tool)
		}

		return nil
	}
}

func registerTool(a *Agent, tool kit.Tool) {
	def := tool.Definition()
	a.tools[def.Name] = tool

	for i, existing := range a.toolDefinitions {
		if existing.Name == def.Name {
			a.toolDefinitions[i] = def

			return
		}
	}

	a.toolDefinitions = append(a.toolDefinitions, def)
}

// WithCompactor sets the [kit.Compactor] and optional compaction threshold.
func WithCompactor(c kit.Compactor, threshold ...float64) Option {
	return func(a *Agent) error {
		a.compactor = c

		if len(threshold) > 0 {
			a.config.CompactionThreshold = threshold[0]
		}

		return nil
	}
}

// WithOnRunStart registers a hook called once before the ReAct loop begins.
func WithOnRunStart(fn kit.OnRunStart) Option {
	return func(a *Agent) error {
		a.hooks.runStart = append(a.hooks.runStart, fn)

		return nil
	}
}

// WithOnRunEnd registers a hook called once after the ReAct loop completes.
func WithOnRunEnd(fn kit.OnRunEnd) Option {
	return func(a *Agent) error {
		a.hooks.runEnd = append(a.hooks.runEnd, fn)

		return nil
	}
}

// WithOnModelRequest registers a hook called before each model request.
func WithOnModelRequest(fn kit.OnModelRequest) Option {
	return func(a *Agent) error {
		a.hooks.modelRequest = append(a.hooks.modelRequest, fn)

		return nil
	}
}

// WithOnModelResponse registers a hook called after each model response.
func WithOnModelResponse(fn kit.OnModelResponse) Option {
	return func(a *Agent) error {
		a.hooks.modelResponse = append(a.hooks.modelResponse, fn)

		return nil
	}
}

// WithOnToolCall registers a hook called before each tool execution.
func WithOnToolCall(fn kit.OnToolCall) Option {
	return func(a *Agent) error {
		a.hooks.toolCall = append(a.hooks.toolCall, fn)

		return nil
	}
}

// WithOnToolResult registers a hook called after each tool finishes executing.
func WithOnToolResult(fn kit.OnToolResult) Option {
	return func(a *Agent) error {
		a.hooks.toolResult = append(a.hooks.toolResult, fn)

		return nil
	}
}

// WithOnTurnStart registers a hook called at the start of each ReAct loop iteration.
func WithOnTurnStart(fn kit.OnTurnStart) Option {
	return func(a *Agent) error {
		a.hooks.turnStart = append(a.hooks.turnStart, fn)

		return nil
	}
}

// WithOnTurnEnd registers a hook called at the end of each ReAct loop iteration.
func WithOnTurnEnd(fn kit.OnTurnEnd) Option {
	return func(a *Agent) error {
		a.hooks.turnEnd = append(a.hooks.turnEnd, fn)

		return nil
	}
}

// WithModelMiddleware wraps the agent's model with one or more middleware
// functions. Middlewares are applied in order, so the first middleware is the
// outermost wrapper (it intercepts calls first).
func WithModelMiddleware(middlewares ...kit.ModelMiddleware) Option {
	return func(a *Agent) error {
		for i := len(middlewares) - 1; i >= 0; i-- {
			a.model = middlewares[i](a.model)
		}

		return nil
	}
}

// WithExtension applies all options from extension to the agent.
func WithExtension(extension []Option) Option {
	return func(a *Agent) error {
		for _, opt := range extension {
			if err := opt(a); err != nil {
				return err
			}
		}

		return nil
	}
}

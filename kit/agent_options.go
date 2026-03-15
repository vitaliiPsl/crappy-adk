package kit

import "context"

// AgentOption is a functional option for configuring an [Agent].
type AgentOption func(*Agent) error

// WithInstruction adds a static string to the agent's system prompt.
func WithInstruction(text string) AgentOption {
	return WithInstructions(func(_ context.Context) (string, error) {
		return text, nil
	})
}

// WithInstructions appends one or more [Instruction] values to the
// agent's system prompt. Sources are evaluated in order on each [Agent.Run].
func WithInstructions(sources ...Instruction) AgentOption {
	return func(a *Agent) error {
		a.instructions = append(a.instructions, sources...)

		return nil
	}
}

// WithTool registers a single tool with the agent.
func WithTool(tool Tool) AgentOption {
	return func(a *Agent) error {
		a.tools[tool.Definition().Name] = tool

		return nil
	}
}

// WithTools registers multiple tools with the agent.
func WithTools(tools ...Tool) AgentOption {
	return func(a *Agent) error {
		for _, tool := range tools {
			a.tools[tool.Definition().Name] = tool
		}

		return nil
	}
}

// WithGenerationConfig sets the generation parameters used on every model request.
func WithGenerationConfig(config GenerationConfig) AgentOption {
	return func(a *Agent) error {
		a.generationConfig = config

		return nil
	}
}

// WithOnRunStart registers a hook called once before the ReAct loop begins.
func WithOnRunStart(fn OnRunStart) AgentOption {
	return func(a *Agent) error {
		a.hooks.runStart = append(a.hooks.runStart, fn)

		return nil
	}
}

// WithOnRunEnd registers a hook called once after the ReAct loop completes.
func WithOnRunEnd(fn OnRunEnd) AgentOption {
	return func(a *Agent) error {
		a.hooks.runEnd = append(a.hooks.runEnd, fn)

		return nil
	}
}

// WithOnModelRequest registers a hook called before each model request.
func WithOnModelRequest(fn OnModelRequest) AgentOption {
	return func(a *Agent) error {
		a.hooks.modelRequest = append(a.hooks.modelRequest, fn)

		return nil
	}
}

// WithOnModelResponse registers a hook called after each model response.
func WithOnModelResponse(fn OnModelResponse) AgentOption {
	return func(a *Agent) error {
		a.hooks.modelResponse = append(a.hooks.modelResponse, fn)

		return nil
	}
}

// WithOnToolCall registers a hook called before each tool execution.
func WithOnToolCall(fn OnToolCall) AgentOption {
	return func(a *Agent) error {
		a.hooks.toolCall = append(a.hooks.toolCall, fn)

		return nil
	}
}

// WithOnToolResult registers a hook called after each tool finishes executing.
func WithOnToolResult(fn OnToolResult) AgentOption {
	return func(a *Agent) error {
		a.hooks.toolResult = append(a.hooks.toolResult, fn)

		return nil
	}
}

// WithOnTurnStart registers a hook called at the start of each ReAct loop iteration.
func WithOnTurnStart(fn OnTurnStart) AgentOption {
	return func(a *Agent) error {
		a.hooks.turnStart = append(a.hooks.turnStart, fn)

		return nil
	}
}

// WithOnTurnEnd registers a hook called at the end of each ReAct loop iteration.
func WithOnTurnEnd(fn OnTurnEnd) AgentOption {
	return func(a *Agent) error {
		a.hooks.turnEnd = append(a.hooks.turnEnd, fn)

		return nil
	}
}

// WithModelMiddleware wraps the agent's model with one or more middleware
// functions. Middlewares are applied in order, so the first middleware is the
// outermost wrapper (it intercepts calls first).
func WithModelMiddleware(middlewares ...ModelMiddleware) AgentOption {
	return func(a *Agent) error {
		for i := len(middlewares) - 1; i >= 0; i-- {
			a.model = middlewares[i](a.model)
		}

		return nil
	}
}

// WithExtension applies all options from extension to the agent.
func WithExtension(extension []AgentOption) AgentOption {
	return func(a *Agent) error {
		for _, opt := range extension {
			if err := opt(a); err != nil {
				return err
			}
		}

		return nil
	}
}

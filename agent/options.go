package agent

import "github.com/vitaliiPsl/crappy-adk/kit"

// Option is a functional option for configuring an [Agent].
type Option func(*Agent) error

// WithInstructions sets the instructions describing the agent's behavior and role.
func WithInstructions(text string) Option {
	return func(a *Agent) error {
		a.config.Instructions = text

		return nil
	}
}

// WithInstructionsSource appends guidance rendered under a source heading.
func WithInstructionsSource(source string, text ...string) Option {
	instructions := kit.NewInstructions(source, text...)

	return func(a *Agent) error {
		a.instructions = append(a.instructions, instructions...)

		return nil
	}
}

// WithTools adds tools. Tool names must be unique.
func WithTools(tools ...kit.Tool) Option {
	return func(a *Agent) error {
		a.tools = append(a.tools, tools...)

		return nil
	}
}

// WithTemperature sets the sampling temperature.
func WithTemperature(v float32) Option {
	return func(a *Agent) error {
		a.config.Temperature = &v

		return nil
	}
}

// WithMaxOutputTokens limits the length of each model response.
func WithMaxOutputTokens(v int32) Option {
	return func(a *Agent) error {
		a.config.MaxOutputTokens = &v

		return nil
	}
}

// WithThinking sets the extended reasoning level.
func WithThinking(v kit.ThinkingLevel) Option {
	return func(a *Agent) error {
		a.config.Thinking = &v

		return nil
	}
}

// WithOutputSchema constrains final responses to the supplied JSON schema.
func WithOutputSchema(schema kit.OutputSchema) Option {
	return func(a *Agent) error {
		output, err := newOutputContract(schema)
		if err != nil {
			return err
		}

		a.config.OutputSchema = &schema
		a.output = output

		return nil
	}
}

// WithOnModelRequestReady registers a hook after request construction.
func WithOnModelRequestReady(fn kit.OnModelRequestReady) Option {
	return func(a *Agent) error {
		a.hooks.modelRequestReady = append(a.hooks.modelRequestReady, fn)

		return nil
	}
}

// WithOnModelResponse registers a hook that is called after each model response.
func WithOnModelResponse(fn kit.OnModelResponse) Option {
	return func(a *Agent) error {
		a.hooks.modelResponse = append(a.hooks.modelResponse, fn)

		return nil
	}
}

// WithOnModelError registers a hook that is called when a model call fails.
func WithOnModelError(fn kit.OnModelError) Option {
	return func(a *Agent) error {
		a.hooks.modelError = append(a.hooks.modelError, fn)

		return nil
	}
}

// WithOnToolCall registers a hook that is called before each tool execution.
func WithOnToolCall(fn kit.OnToolCall) Option {
	return func(a *Agent) error {
		a.hooks.toolCall = append(a.hooks.toolCall, fn)

		return nil
	}
}

// WithOnToolResult registers a hook that is called after each tool execution.
func WithOnToolResult(fn kit.OnToolResult) Option {
	return func(a *Agent) error {
		a.hooks.toolResult = append(a.hooks.toolResult, fn)

		return nil
	}
}

// WithOnTurnStart registers a hook that is called before each model call.
func WithOnTurnStart(fn kit.OnTurnStart) Option {
	return func(a *Agent) error {
		a.hooks.turnStart = append(a.hooks.turnStart, fn)

		return nil
	}
}

// WithOnTurnEnd registers a hook that is called after each turn completes.
func WithOnTurnEnd(fn kit.OnTurnEnd) Option {
	return func(a *Agent) error {
		a.hooks.turnEnd = append(a.hooks.turnEnd, fn)

		return nil
	}
}

// WithExtensions applies one or more options to the agent, allowing extensions to be added in a modular way.
func WithExtensions(extensions ...Option) Option {
	return func(a *Agent) error {
		for _, opt := range extensions {
			if err := opt(a); err != nil {
				return err
			}
		}

		return nil
	}
}

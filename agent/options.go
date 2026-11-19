package agent

import "github.com/vitaliiPsl/crappy-adk/kit"

type Options struct {
	config       kit.AgentConfig
	output       outputContract
	instructions []kit.Instruction
	hooks        hooks

	tools kit.ToolSet
}

type Option func(*Options) error

func WithInstructions(text string) Option {
	return func(a *Options) error {
		a.config.Instructions = text

		return nil
	}
}

// WithInstructionsSource appends guidance rendered under a source heading.
func WithInstructionsSource(source string, text ...string) Option {
	instructions := kit.NewInstructions(source, text...)

	return func(a *Options) error {
		a.instructions = append(a.instructions, instructions...)

		return nil
	}
}

// WithTools registers tools in the currently configured set.
func WithTools(tools ...kit.Tool) Option {
	return func(a *Options) error {
		return a.tools.Register(tools...)
	}
}

// WithToolSet supplies the registry used by subsequent WithTools options.
func WithToolSet(set kit.ToolSet) Option {
	return func(a *Options) error {
		a.tools = set

		return nil
	}
}

func WithTemperature(v float32) Option {
	return func(a *Options) error {
		a.config.Temperature = &v

		return nil
	}
}

func WithMaxOutputTokens(v int32) Option {
	return func(a *Options) error {
		a.config.MaxOutputTokens = &v

		return nil
	}
}

func WithThinking(v kit.ThinkingLevel) Option {
	return func(a *Options) error {
		a.config.Thinking = &v

		return nil
	}
}

// WithOutputSchema constrains final responses to the supplied JSON schema.
func WithOutputSchema(schema kit.OutputSchema) Option {
	return func(a *Options) error {
		output, err := newOutputContract(schema)
		if err != nil {
			return err
		}

		a.config.OutputSchema = &schema
		a.output = output

		return nil
	}
}

func WithOnModelRequestReady(fn kit.OnModelRequestReady) Option {
	return func(a *Options) error {
		a.hooks.modelRequestReady = append(a.hooks.modelRequestReady, fn)

		return nil
	}
}

func WithOnModelResponse(fn kit.OnModelResponse) Option {
	return func(a *Options) error {
		a.hooks.modelResponse = append(a.hooks.modelResponse, fn)

		return nil
	}
}

func WithOnModelError(fn kit.OnModelError) Option {
	return func(a *Options) error {
		a.hooks.modelError = append(a.hooks.modelError, fn)

		return nil
	}
}

func WithOnToolCall(fn kit.OnToolCall) Option {
	return func(a *Options) error {
		a.hooks.toolCall = append(a.hooks.toolCall, fn)

		return nil
	}
}

func WithOnToolResult(fn kit.OnToolResult) Option {
	return func(a *Options) error {
		a.hooks.toolResult = append(a.hooks.toolResult, fn)

		return nil
	}
}

func WithOnTurnStart(fn kit.OnTurnStart) Option {
	return func(a *Options) error {
		a.hooks.turnStart = append(a.hooks.turnStart, fn)

		return nil
	}
}

func WithOnTurnEnd(fn kit.OnTurnEnd) Option {
	return func(a *Options) error {
		a.hooks.turnEnd = append(a.hooks.turnEnd, fn)

		return nil
	}
}

func WithExtensions(extensions ...Option) Option {
	return func(a *Options) error {
		for _, opt := range extensions {
			if err := opt(a); err != nil {
				return err
			}
		}

		return nil
	}
}

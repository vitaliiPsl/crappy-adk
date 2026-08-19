package agent

import (
	"strings"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

// Option is a functional option for configuring an [Agent].
type Option func(*Agent) error

// WithInstructions sets or appends to the agent's system prompt.
func WithInstructions(sources ...string) Option {
	return func(a *Agent) error {
		for _, source := range sources {
			a.config.Instructions = appendInstructions(a.config.Instructions, source)
		}

		return nil
	}
}

// DynamicInstructions resolves instructions before each model request.
type DynamicInstructions func(*kit.RunContext) (string, error)

// WithDynamicInstructions appends resolved instructions before each model request.
func WithDynamicInstructions(resolvers ...DynamicInstructions) Option {
	return WithOnModelRequest(func(rc *kit.RunContext, req kit.ModelRequest) (kit.ModelRequest, error) {
		for _, resolve := range resolvers {
			instructions, err := resolve(rc)
			if err != nil {
				return kit.ModelRequest{}, err
			}

			req.Instructions = appendInstructions(req.Instructions, instructions)
		}

		return req, nil
	})
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

// WithTools adds one or more tools to the agent's tool set. The first
// registration of a given name wins; later registrations with the same name
// are ignored.
func WithTools(tools ...kit.Tool) Option {
	return func(a *Agent) error {
		a.tools.Add(tools...)

		return nil
	}
}

// WithOnModelRequest registers a hook that is called before each model request.
func WithOnModelRequest(fn kit.OnModelRequest) Option {
	return func(a *Agent) error {
		a.hooks.modelRequest = append(a.hooks.modelRequest, fn)

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

func appendInstructions(existing, addition string) string {
	addition = strings.TrimSpace(addition)
	if addition == "" {
		return existing
	}

	if strings.TrimSpace(existing) == "" {
		return addition
	}

	return existing + "\n\n" + addition
}

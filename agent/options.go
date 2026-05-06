package agent

import (
	"strings"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

// Option is a functional option for configuring an [Agent].
type Option func(*Agent) error

// WithInstructions appends one or more instructions to the
// agent's system prompt.
func WithInstructions(sources ...string) Option {
	return func(a *Agent) error {
		parts := []string{a.config.Instructions}
		parts = append(parts, sources...)

		a.config.Instructions = strings.Join(parts, "\n\n")

		return nil
	}
}

// WithTools adds one or more tools to the agent's tool set.
func WithTools(tools ...kit.Tool) Option {
	return func(a *Agent) error {
		a.tools = append(a.tools, tools...)
		for _, t := range tools {
			a.toolIndex[t.Definition().Name] = t
		}

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

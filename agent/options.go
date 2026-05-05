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

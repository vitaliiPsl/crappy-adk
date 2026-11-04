package tool

import (
	"maps"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

// Argument is exposed to the model and consumed by a wrapper.
type Argument struct {
	Name   string
	Schema map[string]any
}

// Next invokes the underlying tool after removing wrapper-owned arguments.
type Next func(*kit.RunContext, kit.ToolCall) (kit.ToolOutput, error)

// Handler receives the full call and controls whether to invoke Next.
type Handler func(*kit.RunContext, kit.ToolCall, Next) (kit.ToolOutput, error)

// Wrapper adds behavior and optional arguments to a tool.
type Wrapper struct {
	tool       kit.Tool
	definition kit.ToolDefinition
	arguments  []Argument
	handler    Handler
}

// Wrap adds a handler and optional arguments, rejecting conflicting argument names.
func Wrap(tool kit.Tool, arguments []Argument, handler Handler) (*Wrapper, error) {
	definition := tool.Definition()
	if len(arguments) > 0 {
		schema, err := addArguments(definition.Schema, arguments)
		if err != nil {
			return nil, err
		}

		definition.Schema = schema
	}

	return &Wrapper{
		tool:       tool,
		definition: definition,
		arguments:  append([]Argument(nil), arguments...),
		handler:    handler,
	}, nil
}

func (w *Wrapper) Definition() kit.ToolDefinition {
	return w.definition
}

func (w *Wrapper) Execute(run *kit.RunContext, call kit.ToolCall) (kit.ToolOutput, error) {
	next := func(run *kit.RunContext, call kit.ToolCall) (kit.ToolOutput, error) {
		call.Arguments = removeArguments(call.Arguments, w.arguments)

		return w.tool.Execute(run, call)
	}

	return w.handler(run, call, next)
}

func removeArguments(values map[string]any, arguments []Argument) map[string]any {
	if len(arguments) == 0 {
		return values
	}

	cleaned := maps.Clone(values)
	for _, argument := range arguments {
		delete(cleaned, argument.Name)
	}

	return cleaned
}

package kit

import (
	"context"

	"github.com/google/jsonschema-go/jsonschema"
)

// Tool is an action the agent can perform.
type Tool interface {
	// Definition returns the tool's metadata, used to describe it to the model.
	Definition() ToolDefinition
	// Execute runs the tool with the given arguments and returns its output.
	Execute(ctx context.Context, args map[string]any) (string, error)
}

// ToolDefinition describes a tool to the model — what it's called, what it does, and what arguments it accepts.
type ToolDefinition struct {
	// The unique name of the tool.
	Name string
	// Tells the model what the tool does and when to use it.
	Description string
	// JSON schema describing the tool's arguments.
	Schema *jsonschema.Schema
}

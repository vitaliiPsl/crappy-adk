package tool

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

var _ kit.Tool = (*FunctionTool[int])(nil)

// FunctionTool wraps a typed Go function as a [kit.Tool]. The JSON schema for
// the arguments is generated automatically from the type parameter T.
type FunctionTool[T any] struct {
	name        string
	description string
	schema      *jsonschema.Schema
	resolved    *jsonschema.Resolved
	fn          func(ctx context.Context, args T) (string, error)
}

// NewFunction creates a [FunctionTool] from a name, description, and a typed handler function.
// Returns an error if the schema for T cannot be generated.
func NewFunction[T any](
	name string,
	description string,
	fn func(ctx context.Context, args T) (string, error),
) (*FunctionTool[T], error) {
	schema, err := jsonschema.For[T](nil)
	if err != nil {
		return nil, fmt.Errorf("failed to generate schema: %w", err)
	}

	resolved, err := schema.Resolve(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve schema for type %T: %w", *new(T), err)
	}

	return &FunctionTool[T]{
		name:        name,
		description: description,
		schema:      schema,
		resolved:    resolved,
		fn:          fn,
	}, nil
}

// MustFunction wraps [NewFunction] and panics if the tool cannot be created.
func MustFunction[T any](
	name string,
	description string,
	fn func(ctx context.Context, args T) (string, error),
) *FunctionTool[T] {
	tool, err := NewFunction(name, description, fn)
	if err != nil {
		panic(fmt.Sprintf("failed to create function tool: %v", err))
	}

	return tool
}

func (f *FunctionTool[T]) Definition() kit.ToolDefinition {
	return kit.ToolDefinition{
		Name:        f.name,
		Description: f.description,
		Schema:      f.schema,
	}
}

func (f *FunctionTool[T]) Execute(ctx context.Context, args map[string]any) (string, error) {
	if err := f.resolved.Validate(args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	argsJSON, err := json.Marshal(args)
	if err != nil {
		return "", fmt.Errorf("failed to unmarshal arguments: %w", err)
	}

	var input T
	if err := json.Unmarshal(argsJSON, &input); err != nil {
		return "", fmt.Errorf("failed to unmarshal arguments: %w", err)
	}

	return f.fn(ctx, input)
}

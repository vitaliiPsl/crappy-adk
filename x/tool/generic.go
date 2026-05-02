package tool

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

var _ kit.Tool = (*Generic[any])(nil)

type Generic[T any] struct {
	name        string
	description string
	schema      map[string]any
	resolved    *jsonschema.Resolved
	fn          func(ctx context.Context, input T) (string, error)
}

// New constructs a [Generic] tool whose input schema is derived from T.
//
// T should be a struct with json tags for field names. Use jsonschema tags to add
// per-property descriptions that the model uses to understand each argument:
//
//	type SearchArgs struct {
//		Query string `json:"query" jsonschema:"The search query"`
//		Limit int    `json:"limit,omitempty" jsonschema:"Maximum number of results to return"`
//	}
//
//	t, err := tool.New("search", "Search the web", func(ctx context.Context, args SearchArgs) (string, error) {
//		...
//	})
func New[T any](
	name,
	description string,
	fn func(ctx context.Context, args T) (string, error),
) (*Generic[T], error) {
	schema, err := jsonschema.For[T](nil)
	if err != nil {
		return nil, fmt.Errorf("failed to generate schema: %w", err)
	}

	resolved, err := schema.Resolve(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve schema for type %T: %w", *new(T), err)
	}

	schemaJSON, err := json.Marshal(schema)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal schema: %w", err)
	}

	var schemaMap map[string]any
	if err := json.Unmarshal(schemaJSON, &schemaMap); err != nil {
		return nil, fmt.Errorf("failed to parse schema: %w", err)
	}

	return &Generic[T]{
		name:        name,
		description: description,
		schema:      schemaMap,
		resolved:    resolved,
		fn:          fn,
	}, nil
}

// Definition returns the tool's name, description, and the JSON schema derived from T.
func (g *Generic[T]) Definition() kit.ToolDefinition {
	return kit.ToolDefinition{
		Name:        g.name,
		Description: g.description,
		Schema:      g.schema,
	}
}

// Execute validates args against the derived schema, unmarshals them into T, and
// calls the handler. Returns an error if validation or unmarshalling fails.
func (g *Generic[T]) Execute(ctx context.Context, args map[string]any) (string, error) {
	if err := g.resolved.Validate(args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	argsJSON, err := json.Marshal(args)
	if err != nil {
		return "", fmt.Errorf("failed to marshal arguments: %w", err)
	}

	var input T

	err = json.Unmarshal(argsJSON, &input)
	if err != nil {
		return "", fmt.Errorf("failed to unmarshal input: %w", err)
	}

	return g.fn(ctx, input)
}

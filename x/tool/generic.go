package tool

import (
	"encoding/json"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

var _ kit.Tool = (*Generic[any, any])(nil)

// Generic is a type-safe tool whose input schema is derived from I and whose
// handler returns O. O is adapted into a [kit.ToolOutput].
type Generic[I, O any] struct {
	name        string
	description string
	schema      map[string]any
	resolved    *jsonschema.Resolved
	fn          func(rc *kit.RunContext, input I) (O, error)
}

// New constructs a [Generic] tool whose input schema is derived from I.
//
// I should be a struct with json tags for field names. Use jsonschema tags to add
// per-property descriptions that the model uses to understand each argument:
//
//	type SearchArgs struct {
//		Query string `json:"query" jsonschema:"The search query"`
//		Limit int    `json:"limit,omitempty" jsonschema:"Maximum number of results to return"`
//	}
//
// O is the handler's return type. Strings become text output, []kit.Content
// becomes rich content output, kit.ToolOutput is used as-is, and any other
// value becomes structured output.
//
//	t, err := tool.New("search", "Search the web", func(rc *kit.RunContext, args SearchArgs) (string, error) {
//		...
//	})
func New[I, O any](
	name,
	description string,
	fn func(rc *kit.RunContext, args I) (O, error),
) (*Generic[I, O], error) {
	schema, err := jsonschema.For[I](nil)
	if err != nil {
		return nil, fmt.Errorf("failed to generate schema: %w", err)
	}

	resolved, err := schema.Resolve(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve schema for type %T: %w", *new(I), err)
	}

	schemaJSON, err := json.Marshal(schema)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal schema: %w", err)
	}

	var schemaMap map[string]any
	if err := json.Unmarshal(schemaJSON, &schemaMap); err != nil {
		return nil, fmt.Errorf("failed to parse schema: %w", err)
	}

	return &Generic[I, O]{
		name:        name,
		description: description,
		schema:      schemaMap,
		resolved:    resolved,
		fn:          fn,
	}, nil
}

// MustNew is like [New] but panics if schema generation fails.
func MustNew[I, O any](
	name,
	description string,
	fn func(rc *kit.RunContext, args I) (O, error),
) *Generic[I, O] {
	t, err := New(name, description, fn)
	if err != nil {
		panic(fmt.Sprintf("tool.MustNew(%q): %v", name, err))
	}

	return t
}

// Definition returns the tool's name, description, and the JSON schema derived from I.
func (g *Generic[I, O]) Definition() kit.ToolDefinition {
	return kit.ToolDefinition{
		Name:        g.name,
		Description: g.description,
		Schema:      g.schema,
	}
}

// Execute validates the call arguments against the derived schema, unmarshals them into I,
// calls the handler, and adapts the result into a [kit.ToolOutput].
func (g *Generic[I, O]) Execute(rc *kit.RunContext, call kit.ToolCall) (kit.ToolOutput, error) {
	args := call.Arguments
	if err := g.resolved.Validate(args); err != nil {
		return kit.ToolOutput{}, fmt.Errorf("invalid arguments: %w", err)
	}

	input, err := inputValue[I](args)
	if err != nil {
		return kit.ToolOutput{}, err
	}

	result, err := g.fn(rc, input)
	if err != nil {
		return kit.ToolOutput{}, err
	}

	output, err := toolOutput(result)
	if err != nil {
		return kit.ToolOutput{}, err
	}

	return output, nil
}

func toolOutput(value any) (kit.ToolOutput, error) {
	switch v := value.(type) {
	case string:
		return kit.NewToolOutput(kit.NewTextContent(v)), nil
	case kit.Content:
		return kit.NewToolOutput(v), nil
	case []kit.Content:
		return kit.NewToolOutput(v...), nil
	case kit.ToolOutput:
		return v, nil
	default:
		structured, err := structuredToolValue(value)
		if err != nil {
			return kit.ToolOutput{}, err
		}

		return kit.NewStructuredToolOutput(structured), nil
	}
}

func inputValue[I any](args map[string]any) (I, error) {
	data, err := json.Marshal(args)
	if err != nil {
		return *new(I), fmt.Errorf("failed to marshal arguments: %w", err)
	}

	var input I
	if err := json.Unmarshal(data, &input); err != nil {
		return *new(I), fmt.Errorf("failed to unmarshal input: %w", err)
	}

	return input, nil
}

func structuredToolValue(value any) (any, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal tool output: %w", err)
	}

	var structured any
	if err := json.Unmarshal(data, &structured); err != nil {
		return nil, fmt.Errorf("failed to normalize tool output: %w", err)
	}

	return structured, nil
}

package output

import (
	"encoding/json"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

func For[T any](name, description string) (kit.OutputSchema, error) {
	schema, err := jsonschema.For[T](nil)
	if err != nil {
		return kit.OutputSchema{}, fmt.Errorf("generate output schema: %w", err)
	}

	data, err := json.Marshal(schema)
	if err != nil {
		return kit.OutputSchema{}, fmt.Errorf("marshal output schema: %w", err)
	}

	var schemaMap map[string]any
	if err := json.Unmarshal(data, &schemaMap); err != nil {
		return kit.OutputSchema{}, fmt.Errorf("parse output schema: %w", err)
	}

	return kit.OutputSchema{
		Name:        name,
		Description: description,
		Schema:      schemaMap,
	}, nil
}

func MustFor[T any](name, description string) kit.OutputSchema {
	schema, err := For[T](name, description)
	if err != nil {
		panic(fmt.Sprintf("output.MustFor(%q): %v", name, err))
	}

	return schema
}

func Decode[T any](response kit.AgentResponse) (T, error) {
	if len(response.StructuredOutput) == 0 {
		return *new(T), fmt.Errorf("structured output is missing")
	}

	var value T
	if err := json.Unmarshal(response.StructuredOutput, &value); err != nil {
		return *new(T), fmt.Errorf("decode structured output: %w", err)
	}

	return value, nil
}

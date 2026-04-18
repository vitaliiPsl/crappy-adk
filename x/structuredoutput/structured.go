package structuredoutput

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

// Validate parses raw JSON text and validates it against schema.
func Validate(raw string, schema *jsonschema.Schema) (*kit.StructuredOutput, error) {
	if schema == nil {
		return nil, nil
	}

	data := bytes.TrimSpace([]byte(raw))
	if len(data) == 0 {
		return nil, fmt.Errorf("structured output: empty response")
	}

	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, fmt.Errorf("structured output: parse json: %w", err)
	}

	resolved, err := schema.Resolve(nil)
	if err != nil {
		return nil, fmt.Errorf("structured output: resolve schema: %w", err)
	}

	if err := resolved.Validate(value); err != nil {
		return nil, fmt.Errorf("structured output: validate schema: %w", err)
	}

	normalized, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("structured output: marshal normalized json: %w", err)
	}

	return &kit.StructuredOutput{
		JSON: normalized,
	}, nil
}

// SchemaMap converts a jsonschema-go schema to a generic JSON object map that
// provider SDKs can embed in request payloads.
func SchemaMap(schema *jsonschema.Schema) (map[string]any, error) {
	if schema == nil {
		return nil, nil
	}

	data, err := json.Marshal(schema)
	if err != nil {
		return nil, fmt.Errorf("marshal schema: %w", err)
	}

	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("decode schema json: %w", err)
	}

	return out, nil
}

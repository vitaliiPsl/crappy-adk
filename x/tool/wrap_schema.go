package tool

import (
	"errors"
	"fmt"
	"maps"
)

func addArguments(schema map[string]any, arguments []Argument) (map[string]any, error) {
	decorated := maps.Clone(schema)
	if decorated == nil {
		decorated = map[string]any{}
	}

	if schemaType, exists := decorated["type"]; exists && schemaType != "object" {
		return nil, errors.New("tool schema must describe an object")
	}

	properties, err := schemaProperties(decorated)
	if err != nil {
		return nil, err
	}

	for _, argument := range arguments {
		if _, exists := properties[argument.Name]; exists {
			return nil, fmt.Errorf("tool schema already defines %q", argument.Name)
		}

		properties[argument.Name] = maps.Clone(argument.Schema)
	}

	decorated["type"] = "object"
	decorated["properties"] = properties

	return decorated, nil
}

func schemaProperties(schema map[string]any) (map[string]any, error) {
	value, exists := schema["properties"]
	if !exists {
		return map[string]any{}, nil
	}

	properties, ok := value.(map[string]any)
	if !ok {
		return nil, errors.New("tool schema properties must be an object")
	}

	return maps.Clone(properties), nil
}

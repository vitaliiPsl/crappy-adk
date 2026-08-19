package agent

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

type outputContract struct {
	schema *jsonschema.Resolved
}

func newOutputContract(output kit.OutputSchema) (outputContract, error) {
	if strings.TrimSpace(output.Name) == "" {
		return outputContract{}, fmt.Errorf("output schema name is required")
	}

	if output.Schema == nil {
		return outputContract{}, fmt.Errorf("output schema is required")
	}

	data, err := json.Marshal(output.Schema)
	if err != nil {
		return outputContract{}, fmt.Errorf("marshal output schema: %w", err)
	}

	var schema jsonschema.Schema
	if err := json.Unmarshal(data, &schema); err != nil {
		return outputContract{}, fmt.Errorf("parse output schema: %w", err)
	}

	resolved, err := schema.Resolve(nil)
	if err != nil {
		return outputContract{}, fmt.Errorf("resolve output schema: %w", err)
	}

	return outputContract{schema: resolved}, nil
}

func (o outputContract) capture(rc *kit.RunContext, message kit.Message) error {
	if o.schema == nil {
		return nil
	}

	structured, err := o.validate(message)
	if err != nil {
		return err
	}

	rc.StructuredOutput = structured

	return nil
}

func (o outputContract) validate(message kit.Message) (json.RawMessage, error) {
	text := message.TextContent()
	if text == nil || strings.TrimSpace(text.Text) == "" {
		return nil, fmt.Errorf("%w: final response has no text content", kit.ErrInvalidOutput)
	}

	raw := json.RawMessage(text.Text)

	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, fmt.Errorf("%w: decode JSON: %v", kit.ErrInvalidOutput, err)
	}

	if err := o.schema.Validate(value); err != nil {
		return nil, fmt.Errorf("%w: validate JSON: %v", kit.ErrInvalidOutput, err)
	}

	return append(json.RawMessage(nil), raw...), nil
}

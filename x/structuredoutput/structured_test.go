package structuredoutput

import (
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
)

func TestValidate(t *testing.T) {
	schema := &jsonschema.Schema{
		Type:     "object",
		Required: []string{"answer"},
		Properties: map[string]*jsonschema.Schema{
			"answer": {Type: "string"},
		},
	}

	out, err := Validate(`{"answer":"done"}`, schema)
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}

	if got := string(out.JSON); got != `{"answer":"done"}` {
		t.Fatalf("normalized json = %q, want %q", got, `{"answer":"done"}`)
	}
}

func TestValidate_Invalid(t *testing.T) {
	schema := &jsonschema.Schema{
		Type:     "object",
		Required: []string{"answer"},
		Properties: map[string]*jsonschema.Schema{
			"answer": {Type: "string"},
		},
	}

	if _, err := Validate(`{"answer":3}`, schema); err == nil {
		t.Fatal("expected validation error, got nil")
	}
}

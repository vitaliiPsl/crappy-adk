package output

import (
	"testing"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

type result struct {
	Status string `json:"status" jsonschema:"Investigation status"`
}

func TestForAndDecode(t *testing.T) {
	schema, err := For[result]("result", "Final result")
	if err != nil {
		t.Fatalf("For: %v", err)
	}

	if schema.Name != "result" || schema.Description != "Final result" {
		t.Fatalf("schema metadata = %+v", schema)
	}

	if schema.Schema["type"] != "object" {
		t.Fatalf("schema type = %v, want object", schema.Schema["type"])
	}

	got, err := Decode[result](kit.AgentResponse{StructuredOutput: []byte(`{"status":"resolved"}`)})
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}

	if got.Status != "resolved" {
		t.Fatalf("Status = %q, want resolved", got.Status)
	}
}

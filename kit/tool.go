package kit

import (
	"context"

	"github.com/google/jsonschema-go/jsonschema"
)

type Tool interface {
	Definition() *ToolDefinition
	Execute(ctx context.Context, args map[string]any) (string, error)
}

type ToolDefinition struct {
	Name        string
	Description string
	Schema      *jsonschema.Schema
}

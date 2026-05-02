package tool

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

type addArgs struct {
	A int `json:"a"`
	B int `json:"b"`
}

func newAddTool(t *testing.T) *Generic[addArgs] {
	t.Helper()

	tool, err := New("add", "adds two numbers", func(_ context.Context, args addArgs) (string, error) {
		return fmt.Sprintf("%d", args.A+args.B), nil
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	return tool
}

func TestNew_Definition(t *testing.T) {
	tool := newAddTool(t)

	def := tool.Definition()
	if def.Name != "add" {
		t.Errorf("Name = %q, want %q", def.Name, "add")
	}

	if def.Description != "adds two numbers" {
		t.Errorf("Description = %q, want %q", def.Description, "adds two numbers")
	}

	if def.Schema == nil {
		t.Fatal("Schema is nil")
	}

	props, ok := def.Schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("Schema.properties missing or wrong type: %T", def.Schema["properties"])
	}

	for _, field := range []string{"a", "b"} {
		if _, ok := props[field]; !ok {
			t.Errorf("Schema.properties missing field %q", field)
		}
	}
}

func TestNew_Execute(t *testing.T) {
	tool := newAddTool(t)

	tests := []struct {
		name string
		a, b int
		want string
	}{
		{"positive", 3, 4, "7"},
		{"zero values", 0, 0, "0"},
		{"negative operands", -5, 3, "-2"},
		{"negative result", 1, -10, "-9"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Execute(context.Background(), map[string]any{"a": tt.a, "b": tt.b})
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}

			if result != tt.want {
				t.Errorf("result = %q, want %q", result, tt.want)
			}
		})
	}
}

func TestNew_Execute_InvalidArgs(t *testing.T) {
	tool := newAddTool(t)

	_, err := tool.Execute(context.Background(), map[string]any{"a": "not-a-number", "b": 1})
	if err == nil {
		t.Fatal("expected error for invalid argument type")
	}
}

func TestNew_Execute_FnError(t *testing.T) {
	tool, err := New("fail", "always fails", func(_ context.Context, _ addArgs) (string, error) {
		return "", errors.New("boom")
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, err = tool.Execute(context.Background(), map[string]any{"a": 1, "b": 2})
	if err == nil {
		t.Fatal("expected error from fn")
	}
}

func TestNew_Execute_ContextPropagation(t *testing.T) {
	type ctxKey struct{}

	const want = "sentinel"

	tool, err := New("ctx-check", "checks context value", func(ctx context.Context, _ addArgs) (string, error) {
		val, ok := ctx.Value(ctxKey{}).(string)
		if !ok || val != want {
			return "", fmt.Errorf("unexpected context value: %v", ctx.Value(ctxKey{}))
		}

		return val, nil
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx := context.WithValue(context.Background(), ctxKey{}, want)

	result, err := tool.Execute(ctx, map[string]any{"a": 0, "b": 0})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if result != want {
		t.Errorf("result = %q, want %q", result, want)
	}
}

type annotatedArgs struct {
	Query string `json:"query"         jsonschema:"The search query"`
	Limit int    `json:"limit,omitempty" jsonschema:"Maximum number of results"`
}

func TestNew_Schema_Descriptions(t *testing.T) {
	tool, err := New("search", "search things", func(_ context.Context, args annotatedArgs) (string, error) {
		return args.Query, nil
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	def := tool.Definition()

	props, ok := def.Schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("Schema.properties missing or wrong type: %T", def.Schema["properties"])
	}

	queryProp, ok := props["query"].(map[string]any)
	if !ok {
		t.Fatalf("query property missing or wrong type: %T", props["query"])
	}

	desc, _ := queryProp["description"].(string)
	if desc == "" {
		t.Error("query property: expected non-empty description from jsonschema tag")
	}
}

func TestNew_OptionalField(t *testing.T) {
	tool, err := New("search", "search things", func(_ context.Context, args annotatedArgs) (string, error) {
		return fmt.Sprintf("%s:%d", args.Query, args.Limit), nil
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	result, err := tool.Execute(context.Background(), map[string]any{"query": "hello"})
	if err != nil {
		t.Fatalf("Execute without optional field: %v", err)
	}

	if result != "hello:0" {
		t.Errorf("result = %q, want %q", result, "hello:0")
	}
}

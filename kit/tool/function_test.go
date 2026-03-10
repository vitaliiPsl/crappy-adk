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

func newAddTool(t *testing.T) *FunctionTool[addArgs] {
	t.Helper()

	tool, err := NewFunction("add", "adds two numbers", func(_ context.Context, args addArgs) (string, error) {
		return fmt.Sprintf("%d", args.A+args.B), nil
	})
	if err != nil {
		t.Fatalf("NewFunction: %v", err)
	}

	return tool
}

func TestNewFunction_Definition(t *testing.T) {
	tool := newAddTool(t)

	def := tool.Definition()
	if def.Name != "add" {
		t.Errorf("Name = %q, want %q", def.Name, "add")
	}

	if def.Description != "adds two numbers" {
		t.Errorf("Description = %q, want %q", def.Description, "adds two numbers")
	}

	if def.Schema == nil {
		t.Error("Schema is nil")
	}
}

func TestNewFunction_Execute(t *testing.T) {
	tool := newAddTool(t)

	result, err := tool.Execute(context.Background(), map[string]any{"a": 3, "b": 4})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if result != "7" {
		t.Errorf("result = %q, want %q", result, "7")
	}
}

func TestNewFunction_Execute_InvalidArgs(t *testing.T) {
	tool := newAddTool(t)

	_, err := tool.Execute(context.Background(), map[string]any{"a": "not-a-number", "b": 1})
	if err == nil {
		t.Fatal("expected error for invalid argument type")
	}
}

func TestNewFunction_Execute_FnError(t *testing.T) {
	tool, err := NewFunction("fail", "always fails", func(_ context.Context, _ addArgs) (string, error) {
		return "", errors.New("boom")
	})
	if err != nil {
		t.Fatalf("NewFunction: %v", err)
	}

	_, err = tool.Execute(context.Background(), map[string]any{"a": 1, "b": 2})
	if err == nil {
		t.Fatal("expected error from fn")
	}
}

func TestMustFunction_Panics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected MustFunction to panic for unsupported type")
		}
	}()

	type bad struct {
		Ch chan int `json:"ch"`
	}
	MustFunction("bad", "bad tool", func(_ context.Context, _ bad) (string, error) {
		return "", nil
	})
}

func TestMustFunction_DoesNotPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("unexpected panic: %v", r)
		}
	}()

	MustFunction("add", "adds two numbers", func(_ context.Context, _ addArgs) (string, error) {
		return "", nil
	})
}

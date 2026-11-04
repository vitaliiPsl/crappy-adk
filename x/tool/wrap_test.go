package tool_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/vitaliiPsl/crappy-adk/kit"
	"github.com/vitaliiPsl/crappy-adk/x/tool"
)

type echoInput struct {
	Value string `json:"value"`
}

func TestWrapConsumesItsArgumentsWithoutMutatingTheToolOrCall(t *testing.T) {
	t.Parallel()

	underlying := tool.MustNew("echo", "Echo a value", func(_ *kit.RunContext, input echoInput) (string, error) {
		return input.Value, nil
	})

	wrapped, err := tool.Wrap(underlying,
		[]tool.Argument{{Name: "extra", Schema: map[string]any{"type": "string"}}},
		func(run *kit.RunContext, call kit.ToolCall, next tool.Next) (kit.ToolOutput, error) {
			if call.Arguments["extra"] != "wrapper value" {
				t.Fatal("handler did not receive its argument")
			}

			return next(run, call)
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	definition := wrapped.Definition()
	if definition.Name != "echo" || definition.Description != "Echo a value" {
		t.Fatalf("tool identity changed: %+v", definition)
	}

	properties := definition.Schema["properties"].(map[string]any)
	if !reflect.DeepEqual(properties["extra"], map[string]any{"type": "string"}) {
		t.Fatalf("wrapper argument schema = %#v", properties["extra"])
	}

	originalProperties := underlying.Definition().Schema["properties"].(map[string]any)
	if _, found := originalProperties["extra"]; found {
		t.Fatal("wrapping changed the underlying schema")
	}

	call := kit.NewToolCall("call", "echo", map[string]any{"value": "done", "extra": "wrapper value"})

	output, err := wrapped.Execute(kit.NewRunContext(context.Background()), call)
	if err != nil || kit.ContentsText(output.Content) != "done" {
		t.Fatalf("wrapped tool execution = %+v, %v", output, err)
	}

	if call.Arguments["extra"] != "wrapper value" {
		t.Fatal("wrapper consumed the original call's argument")
	}
}

func TestWrapComposesHandlersAndArgumentOwnership(t *testing.T) {
	t.Parallel()

	var order []string

	underlying := tool.MustNew("echo", "", func(_ *kit.RunContext, input echoInput) (string, error) {
		order = append(order, "tool")

		return input.Value, nil
	})

	inner, err := tool.Wrap(underlying,
		[]tool.Argument{{Name: "inner", Schema: map[string]any{"type": "string"}}},
		func(run *kit.RunContext, call kit.ToolCall, next tool.Next) (kit.ToolOutput, error) {
			if _, found := call.Arguments["outer"]; found {
				t.Fatal("outer argument reached the inner wrapper")
			}

			if call.Arguments["inner"] != "inner value" {
				t.Fatal("inner wrapper lost its argument")
			}

			order = append(order, "inner before")
			output, err := next(run, call)

			order = append(order, "inner after")

			return output, err
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	outer, err := tool.Wrap(inner,
		[]tool.Argument{{Name: "outer", Schema: map[string]any{"type": "string"}}},
		func(run *kit.RunContext, call kit.ToolCall, next tool.Next) (kit.ToolOutput, error) {
			if call.Arguments["outer"] != "outer value" {
				t.Fatal("outer wrapper lost its argument")
			}

			order = append(order, "outer before")
			output, err := next(run, call)

			order = append(order, "outer after")

			return output, err
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	output, err := outer.Execute(kit.NewRunContext(context.Background()), kit.NewToolCall("call", "echo", map[string]any{
		"value": "done", "inner": "inner value", "outer": "outer value",
	}))
	if err != nil || kit.ContentsText(output.Content) != "done" {
		t.Fatalf("composed tool execution = %+v, %v", output, err)
	}

	if want := []string{"outer before", "inner before", "tool", "inner after", "outer after"}; !reflect.DeepEqual(order, want) {
		t.Fatalf("handler order = %v, want %v", order, want)
	}
}

func TestWrapRejectsConflictingArguments(t *testing.T) {
	t.Parallel()

	underlying := tool.MustNew("echo", "", func(_ *kit.RunContext, input echoInput) (string, error) {
		return input.Value, nil
	})
	handler := func(run *kit.RunContext, call kit.ToolCall, next tool.Next) (kit.ToolOutput, error) {
		return next(run, call)
	}

	for _, arguments := range [][]tool.Argument{
		{{Name: "value", Schema: map[string]any{"type": "string"}}},
		{{Name: "extra"}, {Name: "extra"}},
	} {
		if _, err := tool.Wrap(underlying, arguments, handler); err == nil {
			t.Fatalf("Wrap(%v) accepted conflicting arguments", arguments)
		}
	}
}

func TestWrapWithoutArgumentsPreservesScalarSchemaAndCanShortCircuit(t *testing.T) {
	t.Parallel()

	underlying := tool.MustNew("scalar", "Scalar input", func(*kit.RunContext, string) (string, error) {
		t.Fatal("underlying tool ran after the handler rejected the call")

		return "", nil
	})
	blocked := errors.New("blocked")

	wrapped, err := tool.Wrap(underlying, nil, func(*kit.RunContext, kit.ToolCall, tool.Next) (kit.ToolOutput, error) {
		return kit.ToolOutput{}, blocked
	})
	if err != nil {
		t.Fatal(err)
	}

	if got := wrapped.Definition(); !reflect.DeepEqual(got, underlying.Definition()) {
		t.Fatalf("wrapper changed scalar schema: %+v", got)
	}

	if _, err := wrapped.Execute(kit.NewRunContext(context.Background()), kit.NewToolCall("call", "scalar", nil)); !errors.Is(err, blocked) {
		t.Fatalf("handler error = %v, want %v", err, blocked)
	}

	if _, err := tool.Wrap(underlying, []tool.Argument{{Name: "extra"}}, func(run *kit.RunContext, call kit.ToolCall, next tool.Next) (kit.ToolOutput, error) {
		return next(run, call)
	}); err == nil {
		t.Fatal("wrapper added object arguments to a scalar schema")
	}
}

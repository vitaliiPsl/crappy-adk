package kittest

import (
	"context"
	"reflect"
	"testing"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

// ToolResponse describes what a tool should return for a single call.
type ToolResponse struct {
	Result string
	Error  error
}

// Tool is a programmable test double for [kit.Tool]. Callers describe a
// sequence of [ToolResponse] values. Each Execute call consumes one response.
// If more calls are made than responses provided, the test fails immediately.
type Tool struct {
	t          *testing.T
	definition kit.ToolDefinition
	responses  []ToolResponse
	calls      []map[string]any
}

// NewTool creates a Tool that will play through the given responses in order.
func NewTool(t *testing.T, name, description string, responses ...ToolResponse) *Tool {
	return &Tool{
		t: t,
		definition: kit.ToolDefinition{
			Name:        name,
			Description: description,
		},
		responses: responses,
	}
}

// Definition returns the tool's metadata.
func (tool *Tool) Definition() kit.ToolDefinition {
	return tool.definition
}

// Execute records the call and returns the next queued response.
func (tool *Tool) Execute(_ context.Context, args map[string]any) (string, error) {
	tool.calls = append(tool.calls, cloneMap(args))

	resp := tool.response()

	return resp.Result, resp.Error
}

func (tool *Tool) response() ToolResponse {
	idx := len(tool.calls) - 1
	if idx < 0 {
		tool.t.Fatalf("kittest.Tool %q: no responses configured", tool.definition.Name)
	}

	if idx >= len(tool.responses) {
		tool.t.Fatalf("kittest.Tool %q: unexpected call %d (configured %d response(s))", tool.definition.Name, idx+1, len(tool.responses))
	}

	return tool.responses[idx]
}

// Assertion helpers

// CallCount returns the number of times the tool was called.
func (tool *Tool) CallCount() int {
	return len(tool.calls)
}

// CallAt returns the arguments from the call at the given index.
func (tool *Tool) CallAt(index int) map[string]any {
	tool.t.Helper()

	if index >= len(tool.calls) {
		tool.t.Fatalf("kittest.Tool %q: call index %d out of range (got %d calls)", tool.definition.Name, index, len(tool.calls))
	}

	return cloneMap(tool.calls[index])
}

// AssertCallCount fails the test if the tool was not called exactly n times.
func (tool *Tool) AssertCallCount(t *testing.T, expected int) {
	t.Helper()

	if len(tool.calls) != expected {
		t.Errorf("tool %q call count = %d, want %d", tool.definition.Name, len(tool.calls), expected)
	}
}

// AssertCalledWith fails the test if the call at the given index does not
// match the expected arguments.
func (tool *Tool) AssertCalledWith(t *testing.T, index int, expected map[string]any) {
	t.Helper()

	if index >= len(tool.calls) {
		t.Fatalf("tool %q: call index %d out of range (got %d calls)", tool.definition.Name, index, len(tool.calls))
	}

	if !reflect.DeepEqual(tool.calls[index], expected) {
		t.Errorf("tool %q call[%d] args = %v, want %v", tool.definition.Name, index, tool.calls[index], expected)
	}
}

// AssertNeverCalled fails the test if the tool was called at least once.
func (tool *Tool) AssertNeverCalled(t *testing.T) {
	t.Helper()

	if len(tool.calls) > 0 {
		t.Errorf("tool %q was called %d time(s), want 0", tool.definition.Name, len(tool.calls))
	}
}

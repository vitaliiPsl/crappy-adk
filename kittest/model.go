package kittest

import (
	"context"
	"reflect"
	"testing"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

var _ kit.Model = (*Model)(nil)

// ModelResult describes what a model should return for a single call.
// Events are emitted by Stream; Response and Error are returned by both Generate and Stream.
type ModelResult struct {
	Events   []kit.ModelEvent
	Response kit.ModelResponse
	Error    error
}

// Model is a programmable test double for [kit.Model]. Callers describe a
// sequence of [ModelResult] values. Each Generate call consumes one response.
// If more calls are made than responses provided, the test fails immediately.
type Model struct {
	t        testing.TB
	results  []ModelResult
	requests []kit.ModelRequest
}

// NewModel creates a Model that will play through the given results in order.
func NewModel(t testing.TB, results ...ModelResult) *Model {
	t.Helper()

	return &Model{
		t:       t,
		results: results,
	}
}

// Generate records the request and returns the next queued result.
func (model *Model) Generate(_ context.Context, request kit.ModelRequest) (kit.ModelResponse, error) {
	model.requests = append(model.requests, request)

	result := model.result()

	return result.Response, result.Error
}

// Stream records the request and streams the next queued result.
func (model *Model) Stream(_ context.Context, request kit.ModelRequest) *kit.Stream[kit.ModelEvent, kit.ModelResponse] {
	model.requests = append(model.requests, request)
	result := model.result()

	return kit.NewStream(func(emit kit.Emitter[kit.ModelEvent]) (kit.ModelResponse, error) {
		if result.Error != nil {
			return kit.ModelResponse{}, result.Error
		}

		for _, event := range result.Events {
			if err := emit.Emit(event); err != nil {
				return result.Response, err
			}
		}

		return result.Response, nil
	})
}

func (model *Model) result() ModelResult {
	idx := len(model.requests) - 1
	if idx < 0 {
		model.t.Fatalf("kittest.Model: no results configured")
	}

	if idx >= len(model.results) {
		model.t.Fatalf("kittest.Model: unexpected call %d (configured %d result(s))", idx+1, len(model.results))
	}

	return model.results[idx]
}

// CallCount returns the number of times the model was called.
func (model *Model) CallCount() int {
	return len(model.requests)
}

// CallAt returns the request from the call at the given index.
func (model *Model) CallAt(index int) kit.ModelRequest {
	model.t.Helper()

	if index < 0 || index >= len(model.requests) {
		model.t.Fatalf("kittest.Model: call index %d out of range (got %d calls)", index, len(model.requests))
	}

	return model.requests[index]
}

// AssertCallCount fails the test if the model was not called exactly n times.
func (model *Model) AssertCallCount(t testing.TB, expected int) {
	t.Helper()

	if len(model.requests) != expected {
		t.Errorf("model call count = %d, want %d", len(model.requests), expected)
	}
}

// AssertCalledWith fails the test if the call at the given index does not
// match the expected request.
func (model *Model) AssertCalledWith(t testing.TB, index int, expected kit.ModelRequest) {
	t.Helper()

	if index < 0 || index >= len(model.requests) {
		t.Fatalf("model: call index %d out of range (got %d calls)", index, len(model.requests))
	}

	if !reflect.DeepEqual(model.requests[index], expected) {
		t.Errorf("model call[%d] request = %v, want %v", index, model.requests[index], expected)
	}
}

// AssertNeverCalled fails the test if the model was called at least once.
func (model *Model) AssertNeverCalled(t testing.TB) {
	t.Helper()

	if len(model.requests) > 0 {
		t.Errorf("model was called %d time(s), want 0", len(model.requests))
	}
}

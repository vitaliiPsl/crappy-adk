package kit

import (
	"context"
	"errors"
	"testing"
)

func TestStream_IsLazyAndReturnsResult(t *testing.T) {
	called := false
	stream := NewStream(func(emit Emitter[int]) (string, error) {
		called = true

		if err := emit.Emit(1); err != nil {
			return "partial", err
		}

		if err := emit.Emit(2); err != nil {
			return "partial", err
		}

		return "done", nil
	})

	if called {
		t.Fatal("stream function ran before consumption")
	}

	var events []int
	for event := range stream.Iter() {
		events = append(events, event)
	}

	if !called {
		t.Fatal("stream function did not run during consumption")
	}

	if len(events) != 2 || events[0] != 1 || events[1] != 2 {
		t.Fatalf("events = %v, want [1 2]", events)
	}

	result, err := stream.Result()
	if err != nil {
		t.Fatalf("Result error = %v", err)
	}

	if result != "done" {
		t.Fatalf("Result = %q, want done", result)
	}
}

func TestStream_ResultDrainsUnstartedStream(t *testing.T) {
	stream := NewStream(func(emit Emitter[int]) (int, error) {
		if err := emit.Emit(1); err != nil {
			return 0, err
		}

		return 42, nil
	})

	result, err := stream.Result()
	if err != nil {
		t.Fatalf("Result error = %v", err)
	}

	if result != 42 {
		t.Fatalf("Result = %d, want 42", result)
	}
}

func TestStream_EarlyStopReturnsPartialResult(t *testing.T) {
	stream := NewStream(func(emit Emitter[int]) (int, error) {
		if err := emit.Emit(1); err != nil {
			return 1, err
		}

		if err := emit.Emit(2); err != nil {
			return 2, err
		}

		return 3, nil
	})

	for event := range stream.Iter() {
		if event == 1 {
			break
		}
	}

	result, err := stream.Result()
	if err != nil {
		t.Fatalf("Result error = %v", err)
	}

	if result != 1 {
		t.Fatalf("Result = %d, want partial 1", result)
	}
}

func TestStream_PanicsOnDoubleConsumption(t *testing.T) {
	stream := NewStream(func(Emitter[int]) (int, error) {
		return 1, nil
	})

	for range stream.Iter() {
		_ = struct{}{}
	}

	defer func() {
		if recovered := recover(); recovered == nil {
			t.Fatal("expected panic on double consumption")
		}
	}()

	for range stream.Iter() {
		_ = struct{}{}
	}
}

func TestStreamFromGenerate_EmitsContent(t *testing.T) {
	msg := NewModelMessage(
		NewTextContent("hello"),
		NewTextContent(" world"),
	)

	stream := StreamFromGenerate(
		context.Background(),
		ModelRequest{},
		func(context.Context, ModelRequest) (ModelResponse, error) {
			return ModelResponse{
				Message:      msg,
				FinishReason: FinishReasonStop,
				Usage:        Usage{InputTokens: 3, OutputTokens: 2},
			}, nil
		},
	)

	var events []ModelEvent
	for event := range stream.Iter() {
		events = append(events, event)
	}

	if len(events) != 4 {
		t.Fatalf("len(events) = %d, want 4", len(events))
	}

	if events[0].Type != EventContentStarted || events[1].Type != EventContentDone {
		t.Fatalf("first part events = %q/%q, want started/done", events[0].Type, events[1].Type)
	}

	if events[2].Type != EventContentStarted || events[3].Type != EventContentDone {
		t.Fatalf("second part events = %q/%q, want started/done", events[2].Type, events[3].Type)
	}

	result, err := stream.Result()
	if err != nil {
		t.Fatalf("Result error = %v", err)
	}

	if result.Message.TextContent().Text != "hello" {
		t.Fatalf("result text = %q, want hello", result.Message.TextContent().Text)
	}
}

func TestStreamFromGenerate_ReturnsGenerateError(t *testing.T) {
	wantErr := errors.New("model down")
	stream := StreamFromGenerate(
		context.Background(),
		ModelRequest{},
		func(context.Context, ModelRequest) (ModelResponse, error) {
			return ModelResponse{}, wantErr
		},
	)

	_, err := stream.Result()
	if !errors.Is(err, wantErr) {
		t.Fatalf("Result error = %v, want %v", err, wantErr)
	}
}

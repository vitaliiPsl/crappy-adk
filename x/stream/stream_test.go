package stream

import (
	"errors"
	"testing"
)

func TestStream_ResultDrainsUnstartedStream(t *testing.T) {
	calls := 0

	stream := New(func(e *Emitter[string]) (int, error) {
		calls++

		total := 0
		for _, n := range []int{1, 2, 3} {
			total += n
			if err := e.Emit(string(rune('0' + n))); err != nil {
				return total, nil
			}
		}

		return total, nil
	})

	got, err := stream.Result()
	if err != nil {
		t.Fatalf("Result: %v", err)
	}

	if got != 6 {
		t.Fatalf("Result = %d, want 6", got)
	}

	if calls != 1 {
		t.Fatalf("stream producer called %d times, want 1", calls)
	}
}

func TestStream_ResultAfterPartialIterationReturnsPartialState(t *testing.T) {
	stream := New(func(e *Emitter[string]) (int, error) {
		total := 0

		for _, n := range []int{1, 2, 3} {
			total += n
			if err := e.Emit(string(rune('0' + n))); err != nil {
				return total, nil
			}
		}

		return total, nil
	})

	count := 0
	for range stream.Iter() {
		count++
		if count == 1 {
			break
		}
	}

	got, err := stream.Result()
	if err != nil {
		t.Fatalf("Result: %v", err)
	}

	if got != 1 {
		t.Fatalf("Result = %d, want 1", got)
	}
}

func TestStream_IterCollectsEventsAndResultReturnsFinalState(t *testing.T) {
	stream := New(func(e *Emitter[string]) (int, error) {
		total := 0

		for _, n := range []int{1, 2, 3} {
			total += n
			if err := e.Emit(string(rune('0' + n))); err != nil {
				return total, nil
			}
		}

		return total, nil
	})

	var events []string
	for event := range stream.Iter() {
		events = append(events, event)
	}

	if len(events) != 3 || events[0] != "1" || events[1] != "2" || events[2] != "3" {
		t.Fatalf("events = %#v, want []string{\"1\", \"2\", \"3\"}", events)
	}

	got, err := stream.Result()
	if err != nil {
		t.Fatalf("Result: %v", err)
	}

	if got != 6 {
		t.Fatalf("Result = %d, want 6", got)
	}
}

func TestStream_ResultReturnsStreamError(t *testing.T) {
	wantErr := errors.New("boom")

	stream := New(func(e *Emitter[string]) (int, error) {
		if err := e.Emit("1"); err != nil {
			return 1, nil
		}

		return 1, wantErr
	})

	got, err := stream.Result()
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}

	if got != 1 {
		t.Fatalf("Result = %d, want 1", got)
	}
}

func TestStream_ResultAfterIterationReturnsClosureError(t *testing.T) {
	wantErr := errors.New("boom")

	stream := New(func(e *Emitter[string]) (int, error) {
		_ = e.Emit("1")

		return 1, wantErr
	})

	for range stream.Iter() {
		_ = ""
	}

	if _, err := stream.Result(); !errors.Is(err, wantErr) {
		t.Fatalf("Result error = %v, want %v", err, wantErr)
	}
}

func TestStream_ResultAfterConsumerBreakHasNoError(t *testing.T) {
	stream := New(func(e *Emitter[string]) (int, error) {
		if err := e.Emit("1"); err != nil {
			return 1, nil
		}

		return 2, nil
	})

	for range stream.Iter() {
		break
	}

	if _, err := stream.Result(); err != nil {
		t.Fatalf("Result error = %v, want nil", err)
	}
}

func TestStream_ResultSwallowsStoppedEmitterError(t *testing.T) {
	stream := New(func(e *Emitter[string]) (int, error) {
		if err := e.Emit("1"); err != nil {
			return 1, err
		}

		return 2, nil
	})

	for range stream.Iter() {
		break
	}

	got, err := stream.Result()
	if err != nil {
		t.Fatalf("Result error = %v, want nil", err)
	}

	if got != 1 {
		t.Fatalf("Result = %d, want 1", got)
	}
}

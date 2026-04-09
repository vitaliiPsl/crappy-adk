package kit

import (
	"errors"
	"testing"
)

func TestStream_ResultDrainsUnstartedStream(t *testing.T) {
	calls := 0

	stream := NewStream(func(yield func(string, error) bool) int {
		calls++

		total := 0
		for _, n := range []int{1, 2, 3} {
			total += n
			if !yield(string(rune('0'+n)), nil) {
				return total
			}
		}

		return total
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
	stream := NewStream(func(yield func(string, error) bool) int {
		total := 0

		for _, n := range []int{1, 2, 3} {
			total += n
			if !yield(string(rune('0'+n)), nil) {
				return total
			}
		}

		return total
	})

	count := 0
	for _, err := range stream.Iter() {
		if err != nil {
			t.Fatalf("Iter: %v", err)
		}

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
	stream := NewStream(func(yield func(string, error) bool) int {
		total := 0

		for _, n := range []int{1, 2, 3} {
			total += n
			if !yield(string(rune('0'+n)), nil) {
				return total
			}
		}

		return total
	})

	var events []string
	for event, err := range stream.Iter() {
		if err != nil {
			t.Fatalf("Iter: %v", err)
		}

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

	stream := NewStream(func(yield func(string, error) bool) int {
		if !yield("1", nil) {
			return 1
		}

		yield("", wantErr)

		return 1
	})

	got, err := stream.Result()
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}

	if got != 1 {
		t.Fatalf("Result = %d, want 1", got)
	}
}

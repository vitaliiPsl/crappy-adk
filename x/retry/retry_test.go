package retry

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

// fakeModel plays a scripted sequence of attempts, emitting each entry's
// events before returning its error.
type fakeModel struct {
	attempts []attempt
	calls    int
}

type attempt struct {
	events []kit.ModelEvent
	err    error
}

func (m *fakeModel) Config() kit.ModelConfig { return kit.ModelConfig{ID: "fake"} }

func (m *fakeModel) next() attempt {
	if m.calls >= len(m.attempts) {
		return attempt{}
	}

	current := m.attempts[m.calls]
	m.calls++

	return current
}

func (m *fakeModel) Generate(context.Context, kit.ModelRequest) (kit.ModelResponse, error) {
	current := m.next()
	if current.err != nil {
		return kit.ModelResponse{}, current.err
	}

	return kit.ModelResponse{
		Message:      kit.NewModelMessage(kit.NewTextContent("done")),
		FinishReason: kit.FinishReasonStop,
	}, nil
}

func (m *fakeModel) Stream(context.Context, kit.ModelRequest) *kit.Stream[kit.ModelEvent, kit.ModelResponse] {
	return kit.NewStream(func(emit kit.Emitter[kit.ModelEvent]) (kit.ModelResponse, error) {
		current := m.next()

		for _, event := range current.events {
			if err := emit.Emit(event); err != nil {
				return kit.ModelResponse{}, err
			}
		}

		if current.err != nil {
			return kit.ModelResponse{}, current.err
		}

		return kit.ModelResponse{
			Message:      kit.NewModelMessage(kit.NewTextContent("done")),
			FinishReason: kit.FinishReasonStop,
		}, nil
	})
}

func testConfig() Config {
	config := DefaultConfig()
	config.BaseDelay = time.Millisecond
	config.MaxDelay = 2 * time.Millisecond

	return config
}

func TestGenerateRetriesTransientFailures(t *testing.T) {
	fake := &fakeModel{attempts: []attempt{
		{err: fmt.Errorf("%w: overloaded", kit.ErrServerError)},
		{err: fmt.Errorf("%w: slow down", kit.ErrRateLimit)},
		{},
	}}

	if _, err := Wrap(fake, testConfig()).Generate(
		context.Background(),
		kit.ModelRequest{},
	); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	if fake.calls != 3 {
		t.Fatalf("calls = %d, want 3", fake.calls)
	}
}

func TestGenerateDoesNotRetryPermanentFailures(t *testing.T) {
	tests := []error{
		kit.ErrInvalidOutput,
		kit.ErrAuthentication,
		kit.ErrInvalidRequest,
		kit.ErrContextLength,
	}

	for _, want := range tests {
		t.Run(want.Error(), func(t *testing.T) {
			fake := &fakeModel{attempts: []attempt{
				{err: fmt.Errorf("%w: nope", want)},
				{},
			}}

			if _, err := Wrap(fake, testConfig()).Generate(
				context.Background(),
				kit.ModelRequest{},
			); !errors.Is(err, want) {
				t.Fatalf("Generate() error = %v, want %v", err, want)
			}

			if fake.calls != 1 {
				t.Fatalf("calls = %d, want 1", fake.calls)
			}
		})
	}
}

func TestGenerateStopsAtMaxAttempts(t *testing.T) {
	fake := &fakeModel{attempts: []attempt{
		{err: kit.ErrServerError},
		{err: kit.ErrServerError},
		{err: kit.ErrServerError},
		{},
	}}

	config := testConfig()
	config.MaxAttempts = 3

	if _, err := Wrap(fake, config).Generate(
		context.Background(),
		kit.ModelRequest{},
	); !errors.Is(err, kit.ErrServerError) {
		t.Fatalf("Generate() error = %v, want %v", err, kit.ErrServerError)
	}

	if fake.calls != 3 {
		t.Fatalf("calls = %d, want 3", fake.calls)
	}
}

func TestGenerateStopsAtBudget(t *testing.T) {
	fake := &fakeModel{attempts: []attempt{
		{err: kit.ErrServerError},
		{err: kit.ErrServerError},
		{},
	}}

	config := testConfig()
	config.MaxAttempts = 10
	config.BaseDelay = 50 * time.Millisecond
	config.MaxDelay = 50 * time.Millisecond
	config.Budget = 10 * time.Millisecond

	started := time.Now()

	if _, err := Wrap(fake, config).Generate(
		context.Background(),
		kit.ModelRequest{},
	); !errors.Is(err, kit.ErrServerError) {
		t.Fatalf("Generate() error = %v, want %v", err, kit.ErrServerError)
	}

	if elapsed := time.Since(started); elapsed > config.BaseDelay {
		t.Fatalf("waited %v, want the budget to stop it before a delay", elapsed)
	}

	if fake.calls != 1 {
		t.Fatalf("calls = %d, want 1", fake.calls)
	}
}

func TestStreamRetriesBeforeAnyEvent(t *testing.T) {
	fake := &fakeModel{attempts: []attempt{
		{err: fmt.Errorf("%w: overloaded", kit.ErrServerError)},
		{events: []kit.ModelEvent{
			kit.NewModelContentDeltaEvent(kit.NewTextContent("hi")),
		}},
	}}

	stream := Wrap(fake, testConfig()).Stream(context.Background(), kit.ModelRequest{})

	var events int
	for range stream.Iter() {
		events++
	}

	if _, err := stream.Result(); err != nil {
		t.Fatalf("Result() error = %v", err)
	}

	if fake.calls != 2 {
		t.Fatalf("calls = %d, want 2", fake.calls)
	}

	if events != 1 {
		t.Fatalf("events = %d, want 1", events)
	}
}

func TestStreamDoesNotRetryAfterEmitting(t *testing.T) {
	fake := &fakeModel{attempts: []attempt{
		{
			events: []kit.ModelEvent{
				kit.NewModelContentDeltaEvent(kit.NewTextContent("partial")),
			},
			err: fmt.Errorf("%w: dropped", kit.ErrServerError),
		},
		{},
	}}

	stream := Wrap(fake, testConfig()).Stream(context.Background(), kit.ModelRequest{})

	var events int
	for range stream.Iter() {
		events++
	}

	if _, err := stream.Result(); !errors.Is(err, kit.ErrServerError) {
		t.Fatalf("Result() error = %v, want %v", err, kit.ErrServerError)
	}

	if fake.calls != 1 {
		t.Fatalf("calls = %d, want 1", fake.calls)
	}

	if events != 1 {
		t.Fatalf("events = %d, want 1 (no replay)", events)
	}
}

func TestRetryStopsOnCancelledContext(t *testing.T) {
	fake := &fakeModel{attempts: []attempt{
		{err: kit.ErrServerError},
		{},
	}}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := Wrap(fake, testConfig()).Generate(ctx, kit.ModelRequest{}); err == nil {
		t.Fatal("Generate() error = nil, want a failure")
	}

	if fake.calls != 1 {
		t.Fatalf("calls = %d, want 1", fake.calls)
	}
}

func TestOnRetryObservesEachAttempt(t *testing.T) {
	fake := &fakeModel{attempts: []attempt{
		{err: kit.ErrServerError},
		{err: kit.ErrRateLimit},
		{},
	}}

	var observed []Attempt

	observe := WithOnRetry(func(a Attempt) { observed = append(observed, a) })

	if _, err := Wrap(fake, testConfig(), observe).Generate(
		context.Background(),
		kit.ModelRequest{},
	); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	if len(observed) != 2 {
		t.Fatalf("observed = %d retries, want 2", len(observed))
	}

	if observed[0].Number != 1 || !errors.Is(observed[0].Err, kit.ErrServerError) {
		t.Fatalf("first retry = %+v, want attempt 1 with a server error", observed[0])
	}
}

func TestWrapReturnsModelUnchangedWhenDisabled(t *testing.T) {
	fake := &fakeModel{}

	if got := Wrap(fake, Config{}); got != kit.Model(fake) {
		t.Fatalf("Wrap() = %T, want the model unchanged", got)
	}
}

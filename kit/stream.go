package kit

import (
	"context"
	"errors"
	"iter"
)

var errStopped = errors.New("stream stopped")

// IsStopped reports whether err indicates that stream consumption stopped early.
func IsStopped(err error) bool {
	return errors.Is(err, errStopped)
}

// Emitter emits stream events. Implementations may return a stopped error when
// the consumer stops iterating.
type Emitter[E any] interface {
	Emit(event E) error
}

// NoopEmitter discards all emitted events.
type NoopEmitter[E any] struct{}

// Emit discards event.
func (NoopEmitter[E]) Emit(E) error {
	return nil
}

type seqEmitter[E any] struct {
	yield func(E) bool
}

func (e seqEmitter[E]) Emit(event E) error {
	if !e.yield(event) {
		return errStopped
	}

	return nil
}

// Stream is a lazy, single-consumption iterator that yields events of type E
// and accumulates a final result of type R.
//
// If iteration stops early, [Stream.Result] returns the partial result seen so
// far; it does not resume iteration.
type Stream[E, R any] struct {
	iter iter.Seq[E]

	result R
	err    error

	started bool
	done    bool
}

// NewStream constructs a Stream from fn, invoked lazily on first consumption.
func NewStream[E, R any](fn func(Emitter[E]) (R, error)) *Stream[E, R] {
	s := &Stream[E, R]{}
	s.iter = func(yield func(E) bool) {
		s.result, s.err = fn(seqEmitter[E]{yield: yield})
		if IsStopped(s.err) {
			s.err = nil
		}
	}

	return s
}

// Iter returns the stream iterator. Range over it at most once.
func (s *Stream[E, R]) Iter() iter.Seq[E] {
	return func(yield func(E) bool) {
		if s.started {
			panic("stream: iterator already consumed")
		}

		s.started = true
		defer func() { s.done = true }()

		for event := range s.iter {
			if !yield(event) {
				return
			}
		}
	}
}

// Result returns the accumulated result and any stream error.
// If iteration has not started yet, it drains the stream first.
func (s *Stream[E, R]) Result() (R, error) {
	if !s.done {
		for range s.Iter() {
			_ = struct{}{}
		}
	}

	return s.result, s.err
}

// StreamFromGenerate adapts a Generate-style model function into a model stream.
func StreamFromGenerate(
	ctx context.Context,
	request ModelRequest,
	generate func(context.Context, ModelRequest) (ModelResponse, error),
) *Stream[ModelEvent, ModelResponse] {
	return NewStream(func(emit Emitter[ModelEvent]) (ModelResponse, error) {
		resp, err := generate(ctx, request)
		if err != nil {
			return ModelResponse{}, err
		}

		for _, content := range resp.Message.Content {
			if err := emit.Emit(NewModelContentStartedEvent(content)); err != nil {
				return resp, err
			}

			if err := emit.Emit(NewModelContentDoneEvent(content)); err != nil {
				return resp, err
			}
		}

		if err := emit.Emit(NewModelMessageEvent(resp.Message)); err != nil {
			return resp, err
		}

		return resp, nil
	})
}

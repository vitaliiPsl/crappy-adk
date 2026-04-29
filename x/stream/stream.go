package stream

import "iter"

// Stream is a lazy, single-consumption iterator that yields events of type E
// and accumulates a final result of type R.
//
// If iteration stops early, [Stream.Result] returns the partial result seen so
// far; it does not resume iteration.
type Stream[E, R any] struct {
	iter   iter.Seq[E]
	result R
	err    error
	done   bool
}

// New constructs a Stream from fn, invoked lazily on first consumption.
func New[E, R any](fn func(*Emitter[E]) (R, error)) *Stream[E, R] {
	s := &Stream[E, R]{}
	s.iter = func(yield func(E) bool) {
		s.result, s.err = fn(NewEmitter(yield))
		if IsStopped(s.err) {
			s.err = nil
		}
	}

	return s
}

// Iter returns the stream iterator. Range over it at most once.
func (s *Stream[E, R]) Iter() iter.Seq[E] {
	return func(yield func(E) bool) {
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
			_ = ""
		}
	}

	return s.result, s.err
}

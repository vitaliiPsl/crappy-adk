package kit

import "iter"

// Stream is a lazy, single-consumption iterator that yields events of type E
// and accumulates a final result of type R.
//
// If iteration stops early, [Stream.Result] returns the partial result seen so
// far; it does not resume iteration.
type Stream[E, R any] struct {
	iter   iter.Seq2[E, error]
	result R
	err    error
	done   bool
}

// NewStream constructs a Stream from fn, invoked lazily on first consumption.
func NewStream[E, R any](fn func(yield func(E, error) bool) R) *Stream[E, R] {
	s := &Stream[E, R]{}
	s.iter = func(yield func(E, error) bool) {
		s.result = fn(yield)
	}

	return s
}

// Iter returns the stream iterator. Range over it at most once.
func (s *Stream[E, R]) Iter() iter.Seq2[E, error] {
	return func(yield func(E, error) bool) {
		defer func() { s.done = true }()

		for event, err := range s.iter {
			if err != nil {
				s.err = err
			}

			if !yield(event, err) {
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

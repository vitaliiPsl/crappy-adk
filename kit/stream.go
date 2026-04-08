package kit

import "iter"

// Stream is a lazy iterator that yields incremental events of type E and
// accumulates a final result of type R. Execution begins when the caller
// first ranges over [Stream.Iter].
type Stream[E, R any] struct {
	iter   iter.Seq2[E, error]
	result R
	err    error
	done   bool
}

// NewStream constructs a Stream from fn. fn is invoked lazily on first
// iteration; it should yield events and return the accumulated result.
func NewStream[E, R any](fn func(yield func(E, error) bool) R) *Stream[E, R] {
	s := &Stream[E, R]{}
	s.iter = func(yield func(E, error) bool) {
		s.result = fn(yield)
	}

	return s
}

// Iter returns an iterator over the events produced by the stream.
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

// Result returns the accumulated result and any error that occurred during
// the run. If Iter has not been exhausted, it drains the remaining events first.
func (s *Stream[E, R]) Result() (R, error) {
	if !s.done {
		for range s.Iter() {
			_ = ""
		}
	}

	return s.result, s.err
}

package stream

import "errors"

// errStopped signals that the stream consumer stopped iterating.
var errStopped = errors.New("stream consumer stopped")

// Emitter wraps a stream yield function. Once the consumer rejects an event,
// every subsequent Emit returns a stopped error.
type Emitter[E any] struct {
	yield   func(E) bool
	stopped bool
}

func NewEmitter[E any](yield func(E) bool) *Emitter[E] {
	return &Emitter[E]{yield: yield}
}

func (e *Emitter[E]) Emit(event E) error {
	if e.stopped {
		return errStopped
	}

	if !e.yield(event) {
		e.stopped = true

		return errStopped
	}

	return nil
}

func IsStopped(err error) bool {
	return errors.Is(err, errStopped)
}

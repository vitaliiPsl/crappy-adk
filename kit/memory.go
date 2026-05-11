package kit

import "context"

// Memory provides the model-facing context for a run and records messages
// produced or accepted during the run.
type Memory interface {
	// Context returns the messages that should be sent to the model.
	Context(ctx context.Context) ([]Message, error)
	// History returns the complete messages history.
	History(ctx context.Context) ([]Message, error)
	// Record appends messages to the memory.
	Record(ctx context.Context, messages ...Message) error
}

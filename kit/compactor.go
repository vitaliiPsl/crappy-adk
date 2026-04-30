package kit

import "context"

// Compactor compresses conversation history. The agent calls Compact when it
// determines that the conversation is approaching the model's context limit.
type Compactor interface {
	// Compact compresses messages and returns the compacted messages along
	// with an optional human-readable summary of what was removed. When summary
	// is empty, the agent still uses the compacted messages but does not emit or
	// record a summary message.
	Compact(ctx context.Context, messages []Message) ([]Message, string, error)
}

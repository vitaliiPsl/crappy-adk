package kit

import "context"

// Compactor compresses conversation history. The agent calls Compact when it
// determines that the conversation is approaching the model's context limit.
type Compactor interface {
	// Compact compresses messages and returns the compacted messages along
	// with a human-readable summary of what was removed. The summary is
	// empty if no compaction occurred.
	Compact(ctx context.Context, messages []Message) ([]Message, string, error)
}

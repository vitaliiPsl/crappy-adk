package kit

import "context"

// RunContext is the per-run state that the agent threads through its loop.
// Hooks receive a pointer to it and may inspect or mutate it.
type RunContext struct {
	context.Context

	// Messages is the conversation sent to the model on the next turn.
	// Hooks may mutate it (e.g. to compact older turns).
	Messages []Message

	// Generated holds every message produced during this run, in order.
	// It is returned to the caller in [AgentResponse.Messages].
	Generated []Message

	// Turn is the 1-indexed counter of the current turn, incremented before
	// each model call.
	Turn int

	// Usage is the cumulative token usage across all completed turns.
	Usage Usage

	// LastUsage is the usage reported by the most recent model response.
	LastUsage Usage
}

// Append records a message in both the working set sent to the model and the
// append-only log returned to the caller.
func (rc *RunContext) Append(msg Message) {
	rc.Messages = append(rc.Messages, msg)
	rc.Generated = append(rc.Generated, msg)
}

// RecordUsage stores u as the most recent turn's usage and adds it to the
// cumulative total.
func (rc *RunContext) RecordUsage(u Usage) {
	rc.LastUsage = u
	rc.Usage.Add(u)
}

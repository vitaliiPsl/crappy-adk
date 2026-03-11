package kit

// Event is emitted by [Agent.Run] during streaming.
// Either Delta or Message is set, never both.
type Event struct {
	// Incremental content produced during an active model turn.
	Delta *Delta

	// A complete message, emitted once per finished turn.
	Message *Message
}

// Delta is an incremental piece of a streamed model turn.
type Delta struct {
	// Incremental response token.
	Text string
}

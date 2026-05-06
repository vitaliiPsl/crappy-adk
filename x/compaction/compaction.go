package compaction

import "github.com/vitaliiPsl/crappy-adk/kit"

// Trigger reports whether [Strategy] should run on the current turn.
type Trigger func(rc *kit.RunContext) bool

// Strategy mutates [kit.RunContext] to reduce the working conversation.
type Strategy func(rc *kit.RunContext) error

// OnTurnStart returns an [kit.OnTurnStart] hook that runs strategy whenever
// trigger returns true.
func OnTurnStart(trigger Trigger, strategy Strategy) kit.OnTurnStart {
	return func(rc *kit.RunContext) error {
		if !trigger(rc) {
			return nil
		}

		return strategy(rc)
	}
}

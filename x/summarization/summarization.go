package summarization

import (
	"github.com/vitaliiPsl/crappy-adk/agent"
	"github.com/vitaliiPsl/crappy-adk/kit"
)

// Trigger reports whether [Strategy] should run on the current turn.
type Trigger func(rc *kit.RunContext) bool

// Strategy records summary state through [kit.RunContext].
type Strategy func(rc *kit.RunContext) error

// WithSummarization configures an agent to run strategy whenever trigger fires.
func WithSummarization(trigger Trigger, strategy Strategy) agent.Option {
	return agent.WithOnTurnStart(onTurnStart(trigger, strategy))
}

func onTurnStart(trigger Trigger, strategy Strategy) kit.OnTurnStart {
	return func(rc *kit.RunContext) error {
		if !trigger(rc) {
			return nil
		}

		return strategy(rc)
	}
}

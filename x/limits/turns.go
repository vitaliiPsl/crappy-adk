package limits

import (
	"context"
	"fmt"

	"github.com/vitaliiPsl/crappy-adk/agent"
	"github.com/vitaliiPsl/crappy-adk/kit"
)

type turnGuard struct {
	max   int
	turns int
}

// WithMaxTurns limits model turns in a single run. Zero disables the guard.
func WithMaxTurns(maxTurns int) agent.Option {
	return func(a *agent.Agent) error {
		if maxTurns <= 0 {
			return nil
		}

		guard := &turnGuard{max: maxTurns}

		if err := agent.WithOnRunStart(guard.onRunStart())(a); err != nil {
			return err
		}

		return agent.WithOnTurnStart(guard.onTurnStart())(a)
	}
}

func (g *turnGuard) onRunStart() kit.OnRunStart {
	return func(ctx context.Context, messages []kit.Message) (context.Context, []kit.Message, error) {
		g.turns = 0

		return ctx, messages, nil
	}
}

func (g *turnGuard) onTurnStart() kit.OnTurnStart {
	return func(ctx context.Context, messages []kit.Message) (context.Context, []kit.Message, error) {
		if g.turns+1 > g.max {
			return ctx, nil, fmt.Errorf("%w: max turns %d exceeded", kit.ErrLimitExceeded, g.max)
		}

		g.turns++

		return ctx, messages, nil
	}
}

package limits

import (
	"context"
	"fmt"

	"github.com/vitaliiPsl/crappy-adk/agent"
	"github.com/vitaliiPsl/crappy-adk/kit"
)

type toolCallGuard struct {
	max   int
	calls int
}

// WithMaxToolCalls limits total tool calls requested in a single run. Zero
// disables the guard. The limit is checked before the next turn starts, so a
// final response is never rejected only because it crossed the limit.
func WithMaxToolCalls(maxToolCalls int) agent.Option {
	return func(a *agent.Agent) error {
		if maxToolCalls <= 0 {
			return nil
		}

		guard := &toolCallGuard{max: maxToolCalls}

		if err := agent.WithOnRunStart(guard.onRunStart())(a); err != nil {
			return err
		}

		if err := agent.WithOnModelResponse(guard.onModelResponse())(a); err != nil {
			return err
		}

		return agent.WithOnTurnStart(guard.onTurnStart())(a)
	}
}

func (g *toolCallGuard) onRunStart() kit.OnRunStart {
	return func(ctx context.Context, messages []kit.Message) (context.Context, []kit.Message, error) {
		g.calls = 0

		return ctx, messages, nil
	}
}

func (g *toolCallGuard) onModelResponse() kit.OnModelResponse {
	return func(ctx context.Context, resp kit.ModelResponse) (context.Context, kit.ModelResponse, error) {
		g.calls += len(resp.Message.ToolCalls())

		return ctx, resp, nil
	}
}

func (g *toolCallGuard) onTurnStart() kit.OnTurnStart {
	return func(ctx context.Context, messages []kit.Message) (context.Context, []kit.Message, error) {
		if g.calls > g.max {
			return ctx, nil, fmt.Errorf("%w: max tool calls %d exceeded", kit.ErrLimitExceeded, g.max)
		}

		return ctx, messages, nil
	}
}

package limits

import (
	"context"
	"fmt"

	"github.com/vitaliiPsl/crappy-adk/agent"
	"github.com/vitaliiPsl/crappy-adk/kit"
)

// UsageLimits controls accumulated token usage. Zero values disable the
// corresponding token limit.
type UsageLimits struct {
	InputTokens  int64
	OutputTokens int64
}

// WithMaxUsage limits accumulated token usage in a single run. Limits are
// checked before the next turn starts, so a final response is allowed even if
// the last model call crosses the configured limits.
func WithMaxUsage(limits UsageLimits) agent.Option {
	return func(a *agent.Agent) error {
		if limits.InputTokens <= 0 && limits.OutputTokens <= 0 {
			return nil
		}

		guard := &usageGuard{limits: limits}

		if err := agent.WithOnRunStart(guard.onRunStart())(a); err != nil {
			return err
		}

		if err := agent.WithOnModelResponse(guard.onModelResponse())(a); err != nil {
			return err
		}

		return agent.WithOnTurnStart(guard.onTurnStart())(a)
	}
}

type usageGuard struct {
	limits UsageLimits
	usage  kit.Usage
}

func (g *usageGuard) onRunStart() kit.OnRunStart {
	return func(ctx context.Context, messages []kit.Message) (context.Context, []kit.Message, error) {
		g.usage = kit.Usage{}

		return ctx, messages, nil
	}
}

func (g *usageGuard) onModelResponse() kit.OnModelResponse {
	return func(ctx context.Context, resp kit.ModelResponse) (context.Context, kit.ModelResponse, error) {
		g.usage.Add(resp.Usage)

		return ctx, resp, nil
	}
}

func (g *usageGuard) onTurnStart() kit.OnTurnStart {
	return func(ctx context.Context, messages []kit.Message) (context.Context, []kit.Message, error) {
		if g.limits.InputTokens > 0 && int64(g.usage.InputTokens) > g.limits.InputTokens {
			return ctx, nil, fmt.Errorf("%w: max input tokens %d exceeded", kit.ErrLimitExceeded, g.limits.InputTokens)
		}

		if g.limits.OutputTokens > 0 && int64(g.usage.OutputTokens) > g.limits.OutputTokens {
			return ctx, nil, fmt.Errorf("%w: max output tokens %d exceeded", kit.ErrLimitExceeded, g.limits.OutputTokens)
		}

		return ctx, messages, nil
	}
}

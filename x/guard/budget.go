package guard

import (
	"fmt"

	"github.com/vitaliiPsl/crappy-adk/agent"
	"github.com/vitaliiPsl/crappy-adk/kit"
)

// WithInputTokenBudget limits cumulative input tokens in a single run.
// A non-positive limit disables the guard.
func WithInputTokenBudget(limit int64) agent.Option {
	if limit <= 0 {
		return agent.WithExtensions()
	}

	return agent.WithOnModelResponse(func(rc *kit.RunContext, resp kit.ModelResponse) (kit.ModelResponse, error) {
		usage := rc.Usage
		usage.Add(resp.Usage)

		if usage.InputTokens > limit {
			return kit.ModelResponse{}, fmt.Errorf("%w: input tokens %d > %d", ErrBudgetExceeded, usage.InputTokens, limit)
		}

		return resp, nil
	})
}

// WithOutputTokenBudget limits cumulative output tokens in a single run.
// A non-positive limit disables the guard.
func WithOutputTokenBudget(limit int64) agent.Option {
	if limit <= 0 {
		return agent.WithExtensions()
	}

	return agent.WithOnModelResponse(func(rc *kit.RunContext, resp kit.ModelResponse) (kit.ModelResponse, error) {
		usage := rc.Usage
		usage.Add(resp.Usage)

		if usage.OutputTokens > limit {
			return kit.ModelResponse{}, fmt.Errorf("%w: output tokens %d > %d", ErrBudgetExceeded, usage.OutputTokens, limit)
		}

		return resp, nil
	})
}

// WithTotalTokenBudget limits cumulative input plus output tokens in a single run.
// A non-positive limit disables the guard.
func WithTotalTokenBudget(limit int64) agent.Option {
	if limit <= 0 {
		return agent.WithExtensions()
	}

	return agent.WithOnModelResponse(func(rc *kit.RunContext, resp kit.ModelResponse) (kit.ModelResponse, error) {
		usage := rc.Usage
		usage.Add(resp.Usage)
		total := usage.InputTokens + usage.OutputTokens

		if total > limit {
			return kit.ModelResponse{}, fmt.Errorf("%w: total tokens %d > %d", ErrBudgetExceeded, total, limit)
		}

		return resp, nil
	})
}

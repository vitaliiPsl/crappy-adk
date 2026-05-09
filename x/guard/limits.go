package guard

import (
	"fmt"

	"github.com/vitaliiPsl/crappy-adk/agent"
	"github.com/vitaliiPsl/crappy-adk/kit"
)

// WithMaxTurns limits the number of model turns in a single run.
// A non-positive limit disables the guard.
func WithMaxTurns(limit int) agent.Option {
	if limit <= 0 {
		return agent.WithExtensions()
	}

	return agent.WithOnTurnStart(func(rc *kit.RunContext) error {
		turns := 0
		for _, msg := range rc.Generated {
			if msg.Role == kit.RoleModel {
				turns++
			}
		}

		if turns >= limit {
			return fmt.Errorf("%w: limit %d", ErrMaxTurnsExceeded, limit)
		}

		return nil
	})
}

// WithMaxToolCalls limits the total number of tool calls requested in a single run.
// A non-positive limit disables the guard.
func WithMaxToolCalls(limit int) agent.Option {
	if limit <= 0 {
		return agent.WithExtensions()
	}

	return agent.WithOnModelResponse(func(rc *kit.RunContext, resp kit.ModelResponse) (kit.ModelResponse, error) {
		toolCalls := len(resp.Message.ToolCalls())
		for _, msg := range rc.Generated {
			toolCalls += len(msg.ToolCalls())
		}

		if toolCalls > limit {
			return kit.ModelResponse{}, fmt.Errorf("%w: limit %d", ErrMaxToolCallsExceeded, limit)
		}

		return resp, nil
	})
}

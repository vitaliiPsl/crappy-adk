package limits

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/vitaliiPsl/crappy-adk/agent"
	"github.com/vitaliiPsl/crappy-adk/kit"
)

type toolLoopGuard struct {
	maxRepetitions int
	windowSize     int
	window         [][]string
}

// WithToolLoopDetection limits identical tool calls within a sliding turn
// window. Zero max repetitions or window disables the guard.
func WithToolLoopDetection(maxRepetitions int, window int) agent.Option {
	return func(a *agent.Agent) error {
		if maxRepetitions <= 0 || window <= 0 {
			return nil
		}

		guard := &toolLoopGuard{
			maxRepetitions: maxRepetitions,
			windowSize:     window,
		}

		if err := agent.WithOnRunStart(guard.onRunStart())(a); err != nil {
			return err
		}

		return agent.WithOnModelResponse(guard.onModelResponse())(a)
	}
}

func (g *toolLoopGuard) onRunStart() kit.OnRunStart {
	return func(ctx context.Context, messages []kit.Message) (context.Context, []kit.Message, error) {
		g.window = nil

		return ctx, messages, nil
	}
}

func (g *toolLoopGuard) onModelResponse() kit.OnModelResponse {
	return func(ctx context.Context, resp kit.ModelResponse) (context.Context, kit.ModelResponse, error) {
		if err := g.check(resp.Message.ToolCalls()); err != nil {
			return ctx, kit.ModelResponse{}, err
		}

		return ctx, resp, nil
	}
}

func (g *toolLoopGuard) check(calls []kit.ToolCall) error {
	if len(calls) == 0 {
		return nil
	}

	batch := make([]string, len(calls))
	for i, call := range calls {
		batch[i] = loopKey(call)
	}

	g.window = append(g.window, batch)
	if len(g.window) > g.windowSize {
		g.window = g.window[len(g.window)-g.windowSize:]
	}

	counts := make(map[string]int)
	for _, turn := range g.window {
		for _, k := range turn {
			counts[k]++
		}
	}

	for _, call := range calls {
		if n := counts[loopKey(call)]; n > g.maxRepetitions {
			return fmt.Errorf("%w: %s called %d times within the last %d turns",
				kit.ErrToolLoop, call.Name, n, g.windowSize)
		}
	}

	return nil
}

func loopKey(call kit.ToolCall) string {
	b, _ := json.Marshal(call.Arguments)

	return call.Name + "-" + string(b)
}

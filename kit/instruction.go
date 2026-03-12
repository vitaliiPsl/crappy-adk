package kit

import (
	"context"
	"strings"
)

// Instruction is a function that produces a fragment of the agent system prompt.
// Instructions are evaluated fresh on each [Agent.Run] call and joined in order.
type Instruction func(ctx context.Context) (string, error)

// ComposeInstructions returns an [Instruction] that evaluates each source and
// joins the non-empty results with sep.
func ComposeInstructions(ctx context.Context, sep string, sources ...Instruction) (string, error) {
	parts := make([]string, 0, len(sources))
	for _, s := range sources {
		text, err := s(ctx)
		if err != nil {
			return "", err
		}

		if text != "" {
			parts = append(parts, text)
		}
	}

	return strings.Join(parts, sep), nil
}

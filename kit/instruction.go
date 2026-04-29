package kit

import (
	"strings"
)

// Instruction is a function that produces a fragment of an agent system prompt.
type Instruction func() (string, error)

// ComposeInstructions evaluates each source and joins the non-empty results with sep.
func ComposeInstructions(sep string, sources ...Instruction) (string, error) {
	parts := make([]string, 0, len(sources))
	for _, s := range sources {
		text, err := s()
		if err != nil {
			return "", err
		}

		if text != "" {
			parts = append(parts, text)
		}
	}

	return strings.Join(parts, sep), nil
}

package instructions

import (
	"context"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

// Static wraps a plain string as a [kit.Instruction].
func Static(text string) kit.Instruction {
	return func(_ context.Context) (string, error) {
		return text, nil
	}
}

package instructions

import "github.com/vitaliiPsl/crappy-adk/kit"

// Static wraps a plain string as a [kit.Instruction].
func Static(text string) kit.Instruction {
	return func() (string, error) {
		return text, nil
	}
}

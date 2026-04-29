package instructions

import (
	"fmt"
	"os"
	"strings"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

// File returns a [kit.Instruction] that reads the file at path.
// If the file does not exist the instruction contributes an empty string.
func File(path string) kit.Instruction {
	return func() (string, error) {
		data, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			return "", nil
		}

		if err != nil {
			return "", fmt.Errorf("instruction file %q: %w", path, err)
		}

		return strings.TrimSpace(string(data)), nil
	}
}

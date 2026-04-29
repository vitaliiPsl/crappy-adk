package instructions

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

// Env returns a [kit.Instruction] that describes the runtime environment.
func Env(workdir string) kit.Instruction {
	return func() (string, error) {
		hostname, _ := os.Hostname()

		shell := os.Getenv("SHELL")

		var b strings.Builder
		b.WriteString("# Environment\n")
		fmt.Fprintf(&b, "- Working directory: %s\n", workdir)
		fmt.Fprintf(&b, "- OS: %s\n", runtime.GOOS)
		fmt.Fprintf(&b, "- Architecture: %s\n", runtime.GOARCH)

		if shell != "" {
			fmt.Fprintf(&b, "- Shell: %s\n", shell)
		}

		if hostname != "" {
			fmt.Fprintf(&b, "- Hostname: %s\n", hostname)
		}

		return b.String(), nil
	}
}

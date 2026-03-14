package bash

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/vitaliiPsl/crappy-adk/kit"
	"github.com/vitaliiPsl/crappy-adk/kit/tool"
)

const (
	bashToolName        = "bash"
	bashToolDescription = "Execute a bash command and return its output (stdout and stderr combined)."

	defaultTimeoutSeconds = 120
)

type bashInput struct {
	Command        string `json:"command"                    jsonschema:"The bash command to execute"`
	Description    string `json:"description,omitempty"      jsonschema:"Short description of what this command does"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"  jsonschema:"Timeout in seconds (default 120, max 300)"`
}

// New creates a [kit.Tool] that runs bash commands.
func New() kit.Tool {
	return tool.MustFunction(
		bashToolName,
		bashToolDescription,
		func(ctx context.Context, input bashInput) (string, error) {
			timeout := time.Duration(input.TimeoutSeconds) * time.Second
			if timeout <= 0 {
				timeout = defaultTimeoutSeconds * time.Second
			} else if timeout > 600*time.Second {
				timeout = 600 * time.Second
			}

			ctx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			cmd := exec.CommandContext(ctx, "bash", "-c", input.Command)

			var stdout, stderr bytes.Buffer

			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			if err := cmd.Run(); err != nil {
				out := stdout.String() + stderr.String()

				return out, fmt.Errorf("command failed: %w", err)
			}

			return stdout.String(), nil
		},
	)
}

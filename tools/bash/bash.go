package bash

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/vitaliiPsl/crappy-adk/kit"
	"github.com/vitaliiPsl/crappy-adk/x/tool"
)

const (
	bashToolName        = "bash"
	bashToolDescription = `Execute a bash command and return combined stdout and stderr output.

Use this tool to run shell commands, scripts, build tools, tests, and other executables.
Avoid commands that run indefinitely (servers, watchers) — they will be killed when the context deadline is exceeded.
Prefer dedicated tools (read_file, edit_file, list_directory, etc.) over bash when available.`

	defaultTimeout = 30 * time.Second
)

type Input struct {
	Command     string `json:"command" jsonschema:"The bash command to execute"`
	Description string `json:"description" jsonschema:"A short description of what this command does e.g. 'Run tests' or 'Install dependencies'"`
	Timeout     *int   `json:"timeout,omitempty" jsonschema:"Timeout in seconds. Defaults to 30s. Use for long-running commands like builds or tests"`
}

func NewBash() kit.Tool {
	return tool.MustFunction(
		bashToolName,
		bashToolDescription,
		func(ctx context.Context, input Input) (string, error) {
			timeout := defaultTimeout
			if input.Timeout != nil {
				timeout = time.Duration(*input.Timeout) * time.Second
			}

			ctx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			return runBash(ctx, input.Command)
		},
	)
}

func runBash(ctx context.Context, command string) (string, error) {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "sh"
	}

	cmd := exec.CommandContext(ctx, shell, "-c", command)

	var buf bytes.Buffer

	cmd.Stdout = &buf
	cmd.Stderr = &buf

	err := cmd.Run()

	output := strings.TrimRight(buf.String(), "\n")

	if err != nil {
		if output != "" {
			return "", fmt.Errorf("%s\n%w", output, err)
		}

		return "", err
	}

	return output, nil
}

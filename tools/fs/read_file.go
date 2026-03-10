package filesystem

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/vitaliiPsl/crappy-adk/kit"
	"github.com/vitaliiPsl/crappy-adk/kit/tool"
)

const (
	readFileName        = "read_file"
	readFileDescription = "Read the contents of a file with line numbers (like cat -n). Use start/end parameters to read specific line ranges to avoid reading massive files."
)

type ReadFileInput struct {
	Path  string `json:"path" jsonschema:"Absolute path to the file to read e.g. /home/user/repo/main.go"`
	Start *int   `json:"start,omitempty" jsonschema:"Starting line number (0-indexed). If not provided reads from beginning"`
	End   *int   `json:"end,omitempty" jsonschema:"Ending line number (0-indexed). Use -1 to read to end of file. If not provided reads to end"`
}

func NewReadFile() kit.Tool {
	return tool.MustFunction(
		readFileName,
		readFileDescription,
		func(_ context.Context, input ReadFileInput) (string, error) {
			return readFileLines(input.Path, input.Start, input.End)
		},
	)
}

func readFileLines(path string, start, end *int) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer func() { _ = file.Close() }()

	startLine := 0
	if start != nil {
		startLine = max(*start, 0)
	}

	endLine := -1
	if end != nil {
		endLine = *end
	}

	scanner := bufio.NewScanner(file)
	var result strings.Builder

	currLine := 0
	for currLine < startLine && scanner.Scan() {
		currLine++
	}

	if currLine < startLine {
		return "", fmt.Errorf("start line %d is beyond file length", startLine)
	}

	for scanner.Scan() {
		if endLine != -1 && currLine > endLine {
			break
		}

		fmt.Fprintf(&result, "%6d %s\n", currLine, scanner.Text())
		currLine++
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("error reading file: %w", err)
	}

	return result.String(), nil
}

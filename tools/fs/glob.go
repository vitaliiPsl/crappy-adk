package filesystem

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/vitaliiPsl/crappy-adk/kit"
	"github.com/vitaliiPsl/crappy-adk/utils/glob"
	"github.com/vitaliiPsl/crappy-adk/x/tool"
)

const (
	globName        = "glob"
	globDescription = "Find files matching a glob pattern. Supports ** for recursive matching (e.g. **/*.go, src/**/*.ts). Returns paths relative to dir."

	globMaxResults = 200
)

type GlobInput struct {
	Pattern string `json:"pattern" jsonschema:"Glob pattern to match files against e.g. **/*.txt or src/**/*.md"`
	Path    string `json:"path" jsonschema:"Absolute path to the directory to search in e.g. /home/user/dir"`
}

func NewGlob() kit.Tool {
	return tool.MustFunction(
		globName,
		globDescription,
		func(_ context.Context, input GlobInput) (string, error) {
			return globFiles(input.Path, input.Pattern)
		},
	)
}

func globFiles(dir, pattern string) (string, error) {
	var matches []string

	err := walkWithGitignore(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}

		rel = filepath.ToSlash(rel)

		if glob.Match(pattern, rel) {
			matches = append(matches, rel)
		}

		return nil
	})
	if err != nil {
		return "", fmt.Errorf("failed to walk directory: %w", err)
	}

	if len(matches) == 0 {
		return "(no matches)", nil
	}

	truncated := false
	if len(matches) > globMaxResults {
		matches = matches[:globMaxResults]
		truncated = true
	}

	var result strings.Builder
	for _, m := range matches {
		result.WriteString(m)
		result.WriteString("\n")
	}

	if truncated {
		fmt.Fprintf(&result, "\n... truncated at %d results\n", globMaxResults)
	}

	return result.String(), nil
}

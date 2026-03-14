package filesystem

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/vitaliiPsl/crappy-adk/kit"
	"github.com/vitaliiPsl/crappy-adk/kit/tool"
)

const (
	listName        = "list"
	listDescription = "List the contents of a directory. Returns file and directory names with type indicators. Use this to explore the filesystem structure."

	listDefaultLimit = 100
	listMaxLimit     = 200
)

type ListDirectoryInput struct {
	Path  string `json:"path" jsonschema:"Absolute path to the directory to list e.g. /home/user/repo"`
	Limit *int   `json:"limit,omitempty" jsonschema:"Maximum number of entries to return. Defaults to 100 (capped at 200)."`
}

func NewListDirectory() kit.Tool {
	return tool.MustFunction(
		listName,
		listDescription,
		func(_ context.Context, input ListDirectoryInput) (string, error) {
			limit := listDefaultLimit
			if input.Limit != nil {
				limit = min(*input.Limit, listMaxLimit)
			}

			return listDirectory(input.Path, limit)
		},
	)
}

func listDirectory(path string, limit int) (string, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return "", fmt.Errorf("failed to read directory: %w", err)
	}

	if len(entries) == 0 {
		return "(empty directory)", nil
	}

	truncated := false
	if len(entries) > limit {
		entries = entries[:limit]
		truncated = true
	}

	var result strings.Builder
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() {
			name = name + string(filepath.Separator)
		}

		result.WriteString(name)
		result.WriteString("\n")
	}

	if truncated {
		fmt.Fprintf(&result, "\n... truncated at %d entries\n", limit)
	}

	return result.String(), nil
}

package filesystem

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/vitaliiPsl/crappy-adk/kit"
	"github.com/vitaliiPsl/crappy-adk/kit/tool"
)

const (
	writeFileName        = "write_file"
	writeFileDescription = "Create a new file or overwrite an existing file with the provided content. " +
		"Use this only for creating new files or completely replacing file contents. " +
		"Prefer edit_file for modifying existing files - it is safer and less error-prone. " +
		"If the file already exists, you MUST read it first before overwriting to avoid losing content."
)

type WriteFileInput struct {
	Path    string `json:"path" jsonschema:"Absolute path to the file e.g. /home/user/repo/main.go"`
	Content string `json:"content" jsonschema:"Complete content to write to the file"`
}

func NewWriteFile() kit.Tool {
	return tool.MustFunction(
		writeFileName,
		writeFileDescription,
		func(_ context.Context, input WriteFileInput) (string, error) {
			if err := ensureDirectoryExists(input.Path); err != nil {
				return "", err
			}

			return writeFile(input.Path, input.Content)
		},
	)
}

func ensureDirectoryExists(filePath string) error {
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	return nil
}

func writeFile(path, content string) (string, error) {
	existed := true
	if _, err := os.Stat(path); os.IsNotExist(err) {
		existed = false
	}

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	if existed {
		return fmt.Sprintf("File overwritten successfully: %s", path), nil
	}

	return fmt.Sprintf("File created successfully: %s", path), nil
}

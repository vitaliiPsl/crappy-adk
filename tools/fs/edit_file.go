package filesystem

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/vitaliiPsl/crappy-adk/kit"
	"github.com/vitaliiPsl/crappy-adk/x/tool"
)

const (
	editFileName        = "edit_file"
	editFileDescription = "Edit a file by replacing an exact string match with new content. " +
		"You MUST read the file before editing it to see the actual content. " +
		"The old_string must match exactly one location in the file (unless replace_all is true). " +
		"old_string must include enough surrounding context (whitespace, indentation, nearby lines) to be unique. " +
		"Do not guess or hallucinate file content - always base old_string on what you read from the file. " +
		"To insert text, set old_string to the text right before the insertion point and " +
		"new_string to the original text plus the new content. " +
		"To delete content, set new_string to an empty string. " +
		"Use replace_all to rename variables or replace repeated strings across the file."
)

type EditFileInput struct {
	Path       string `json:"path" jsonschema:"Absolute path to the file to edit"`
	OldString  string `json:"old_string" jsonschema:"The exact string to find in the file. Must include enough surrounding context to be unique unless replace_all is true"`
	NewString  string `json:"new_string" jsonschema:"The replacement string. Must be different from old_string. Can be empty to delete the matched text"`
	ReplaceAll bool   `json:"replace_all,omitempty" jsonschema:"Set to true to replace all occurrences of old_string. Useful for renaming variables or repeated strings. Default is false"`
}

func NewEditFile() kit.Tool {
	return tool.MustFunction(
		editFileName,
		editFileDescription,
		func(_ context.Context, input EditFileInput) (string, error) {
			if input.OldString == "" {
				return "", fmt.Errorf("old_string must not be empty")
			}

			if input.OldString == input.NewString {
				return "", fmt.Errorf("old_string and new_string are identical, no edit needed")
			}

			return editFile(input.Path, input.OldString, input.NewString, input.ReplaceAll)
		},
	)
}

func editFile(path, oldString, newString string, replaceAll bool) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	content := string(data)

	count := strings.Count(content, oldString)
	if count == 0 {
		return "", fmt.Errorf("old_string not found in file")
	}

	if !replaceAll && count > 1 {
		return "", fmt.Errorf("old_string matches %d locations in the file, it must match exactly one. Provide more surrounding context to make the match unique, or set replace_all to true", count)
	}

	var newContent string
	if replaceAll {
		newContent = strings.ReplaceAll(content, oldString, newString)
	} else {
		newContent = strings.Replace(content, oldString, newString, 1)
	}

	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("failed to stat file: %w", err)
	}

	if err := os.WriteFile(path, []byte(newContent), info.Mode()); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	if replaceAll {
		return fmt.Sprintf("Successfully replaced %d occurrences in %s", count, path), nil
	}

	return fmt.Sprintf("Successfully edited %s", path), nil
}

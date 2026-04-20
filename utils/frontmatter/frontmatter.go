package frontmatter

import (
	"bytes"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

const delim = "---"

// Unmarshal splits a markdown document into its YAML frontmatter and body.
// If no frontmatter is present the zero value of T and the full content are returned.
func Unmarshal[T any](content string) (T, string, error) {
	var zero T

	if !strings.HasPrefix(content, delim) {
		return zero, content, nil
	}

	yamlPart, body, ok := strings.Cut(content[len(delim):], delim)
	if !ok {
		return zero, content, nil
	}

	var fm T
	if err := yaml.Unmarshal([]byte(yamlPart), &fm); err != nil {
		return zero, "", fmt.Errorf("parse frontmatter: %w", err)
	}

	return fm, strings.TrimSpace(body), nil
}

// Marshal encodes fm as YAML frontmatter and appends body (if non-empty).
func Marshal[T any](fm T, body string) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString("---\n")

	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)

	if err := enc.Encode(fm); err != nil {
		return nil, fmt.Errorf("marshal frontmatter: %w", err)
	}

	buf.WriteString("---\n")

	if body != "" {
		buf.WriteString("\n")
		buf.WriteString(body)
		buf.WriteString("\n")
	}

	return buf.Bytes(), nil
}

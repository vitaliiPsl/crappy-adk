package memory

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/vitaliiPsl/crappy-adk/kit"
	"github.com/vitaliiPsl/crappy-adk/x/tool"
)

const (
	writeToolName        = "write_memory"
	writeToolDescription = "Save durable information that should be remembered across future conversations. " +
		"Use this only for stable long-term memory."

	deleteToolName        = "delete_memory"
	deleteToolDescription = "Delete a previously saved memory by ID."
)

type writeMemoryInput struct {
	Title   string `json:"title" jsonschema:"Short label for the memory"`
	Content string `json:"content" jsonschema:"The durable information to remember"`
}

type deleteMemoryInput struct {
	ID string `json:"id" jsonschema:"ID of the memory to delete"`
}

func (s *state) newWriteTool() kit.Tool {
	return tool.MustFunction(
		writeToolName,
		writeToolDescription,
		func(ctx context.Context, input writeMemoryInput) (string, error) {
			title := strings.TrimSpace(input.Title)
			content := strings.TrimSpace(input.Content)

			if title == "" {
				return "", fmt.Errorf("title is required")
			}

			if content == "" {
				return "", fmt.Errorf("content is required")
			}

			now := time.Now()

			memory := Memory{
				ID:        uuid.NewString(),
				Title:     title,
				Content:   content,
				CreatedAt: now,
				UpdatedAt: now,
			}

			if err := s.store.Put(ctx, memory); err != nil {
				return "", err
			}

			return fmt.Sprintf("saved memory %s", memory.ID), nil
		},
	)
}

func (s *state) newDeleteTool() kit.Tool {
	return tool.MustFunction(
		deleteToolName,
		deleteToolDescription,
		func(ctx context.Context, input deleteMemoryInput) (string, error) {
			id := strings.TrimSpace(input.ID)
			if id == "" {
				return "", fmt.Errorf("id is required")
			}

			if err := s.store.Delete(ctx, id); err != nil {
				return "", err
			}

			return fmt.Sprintf("deleted memory %s", id), nil
		},
	)
}

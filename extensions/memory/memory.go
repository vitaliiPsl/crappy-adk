package memory

import (
	"context"
	"time"
)

// Memory is a durable record that can be reused across future agent runs.
type Memory struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Store is the source of durable memories for the agent.
type Store interface {
	// List returns all stored memories.
	List(ctx context.Context) ([]Memory, error)
	// Put creates or replaces a memory.
	Put(ctx context.Context, memory Memory) error
	// Delete removes a memory by ID.
	Delete(ctx context.Context, id string) error
}

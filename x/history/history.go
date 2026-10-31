package history

import (
	"context"
	"slices"
	"sync"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

var _ kit.History = (*History)(nil)

// History is a [kit.History] held in memory: every message appended to it.
type History struct {
	mu       sync.Mutex
	messages []kit.Message
}

// New creates an empty History.
func New() *History {
	return &History{}
}

func (h *History) Messages(ctx context.Context) ([]kit.Message, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	return slices.Clone(h.messages), nil
}

func (h *History) Append(_ context.Context, messages ...kit.Message) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.messages = append(h.messages, messages...)

	return nil
}

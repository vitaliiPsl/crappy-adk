package memory

import (
	"context"
	"sync"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

var _ kit.Memory = (*History)(nil)

// History is a short-term memory implementation backed by an in-memory message
// history.
type History struct {
	mu       sync.Mutex
	messages []kit.Message
}

// NewHistory creates a short-term memory initialized with messages.
func NewHistory(messages ...kit.Message) *History {
	return &History{
		messages: messages,
	}
}

// Record appends messages to the history.
func (m *History) Record(_ context.Context, messages ...kit.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.messages = append(m.messages, messages...)

	return nil
}

// Context returns the messages that should be sent to the model.
func (m *History) Context(context.Context) ([]kit.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := len(m.messages) - 1; i >= 0; i-- {
		if isSummary(m.messages[i]) {
			return m.messages[i:], nil
		}
	}

	return m.messages, nil
}

// History returns the complete recorded message history.
func (m *History) History(context.Context) ([]kit.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.messages, nil
}

func isSummary(msg kit.Message) bool {
	for _, content := range msg.Content {
		if content.Type == kit.ContentTypeSummary && content.Summary != nil {
			return true
		}
	}

	return false
}

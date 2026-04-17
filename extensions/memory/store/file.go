package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/vitaliiPsl/crappy-adk/extensions/memory"
)

const storeFile = "memories.json"

// FileStore is a file-backed memory store for durable assistant memory.
type FileStore struct {
	mu       sync.RWMutex
	filePath string
	memories []memory.Memory
}

// NewFileStore creates a file-backed memory store rooted at dir.
func NewFileStore(dir string) (*FileStore, error) {
	if dir == "" {
		return nil, fmt.Errorf("memory dir is required")
	}

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create memory dir: %w", err)
	}

	st := &FileStore{filePath: filepath.Join(dir, storeFile)}
	if err := st.load(); err != nil {
		return nil, err
	}

	return st, nil
}

func (st *FileStore) List(_ context.Context) ([]memory.Memory, error) {
	st.mu.RLock()
	defer st.mu.RUnlock()

	out := make([]memory.Memory, len(st.memories))
	copy(out, st.memories)

	return out, nil
}

func (st *FileStore) Put(_ context.Context, mem memory.Memory) error {
	st.mu.Lock()
	defer st.mu.Unlock()

	replaced := false
	for i, existing := range st.memories {
		if existing.ID == mem.ID {
			st.memories[i] = mem
			replaced = true

			break
		}
	}

	if !replaced {
		st.memories = append(st.memories, mem)
	}

	return st.persistLocked()
}

func (st *FileStore) Delete(_ context.Context, id string) error {
	st.mu.Lock()
	defer st.mu.Unlock()

	for i, mem := range st.memories {
		if mem.ID == id {
			st.memories = append(st.memories[:i], st.memories[i+1:]...)

			return st.persistLocked()
		}
	}

	return fmt.Errorf("memory not found: %s", id)
}

func (st *FileStore) load() error {
	data, err := os.ReadFile(st.filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}

		return fmt.Errorf("read memory store: %w", err)
	}

	if len(data) == 0 {
		return nil
	}

	if err := json.Unmarshal(data, &st.memories); err != nil {
		return fmt.Errorf("parse memory store: %w", err)
	}

	return nil
}

func (st *FileStore) persistLocked() error {
	data, err := json.MarshalIndent(st.memories, "", "  ")
	if err != nil {
		return fmt.Errorf("encode memory store: %w", err)
	}

	tmp := st.filePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write temp memory store: %w", err)
	}

	if err := os.Rename(tmp, st.filePath); err != nil {
		_ = os.Remove(tmp)

		return fmt.Errorf("replace memory store: %w", err)
	}

	return nil
}

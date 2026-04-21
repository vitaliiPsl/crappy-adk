package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/vitaliiPsl/crappy-adk/extensions/memory"
)

func TestFileStorePutListDelete(t *testing.T) {
	st, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}

	mem := memory.Memory{
		ID:        "m1",
		Title:     "Style",
		Content:   "Prefer concise answers.",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := st.Put(context.Background(), mem); err != nil {
		t.Fatalf("Put: %v", err)
	}

	memories, err := st.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if len(memories) != 1 {
		t.Fatalf("len(memories) = %d, want 1", len(memories))
	}

	if memories[0].Title != "Style" {
		t.Fatalf("memory title = %q, want %q", memories[0].Title, "Style")
	}

	if err := st.Delete(context.Background(), "m1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	memories, err = st.List(context.Background())
	if err != nil {
		t.Fatalf("List after delete: %v", err)
	}

	if len(memories) != 0 {
		t.Fatalf("len(memories) = %d, want 0", len(memories))
	}
}

func TestFileStorePersistsAcrossReload(t *testing.T) {
	dir := t.TempDir()

	st, err := NewFileStore(dir)
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}

	if err := st.Put(context.Background(), memory.Memory{
		ID:        "m1",
		Title:     "Review",
		Content:   "Lead with bugs.",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}); err != nil {
		t.Fatalf("Put: %v", err)
	}

	st2, err := NewFileStore(dir)
	if err != nil {
		t.Fatalf("NewFileStore reload: %v", err)
	}

	memories, err := st2.List(context.Background())
	if err != nil {
		t.Fatalf("List reload: %v", err)
	}

	if len(memories) != 1 {
		t.Fatalf("len(memories) = %d, want 1", len(memories))
	}

	if memories[0].Content != "Lead with bugs." {
		t.Fatalf("memory content = %q, want %q", memories[0].Content, "Lead with bugs.")
	}
}

func TestNewFileStore_ExpandsHomePath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	st, err := NewFileStore("~/memory")
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}

	want := filepath.Join(home, "memory")
	if st.filePath != filepath.Join(want, storeFile) {
		t.Fatalf("filePath = %q, want %q", st.filePath, filepath.Join(want, storeFile))
	}

	if _, err := os.Stat(want); err != nil {
		t.Fatalf("expected memory dir to exist: %v", err)
	}
}

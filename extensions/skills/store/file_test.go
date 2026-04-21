package store

import (
	"path/filepath"
	"testing"
)

func TestNewFileStore_ExpandsHomePath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	st, err := NewFileStore("~/skills")
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}

	want := filepath.Join(home, "skills")
	if st.dir != want {
		t.Fatalf("dir = %q, want %q", st.dir, want)
	}
}

func TestNewFileStore_RequiresDir(t *testing.T) {
	if _, err := NewFileStore(""); err == nil {
		t.Fatal("expected error for empty dir")
	}
}

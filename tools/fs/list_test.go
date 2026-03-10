package filesystem

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestListDirectory_Basic(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "file.txt"), []byte(""), 0644)
	_ = os.Mkdir(filepath.Join(dir, "subdir"), 0755)

	out, err := listDirectory(dir, listDefaultLimit)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(out, "file.txt") {
		t.Errorf("expected file.txt in output, got: %s", out)
	}

	if !strings.Contains(out, "subdir/") {
		t.Errorf("expected subdir/ (with trailing slash) in output, got: %s", out)
	}
}

func TestListDirectory_Empty(t *testing.T) {
	dir := t.TempDir()

	out, err := listDirectory(dir, listDefaultLimit)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(out, "empty") {
		t.Errorf("expected empty message, got: %s", out)
	}
}

func TestListDirectory_Limit(t *testing.T) {
	dir := t.TempDir()
	for i := range 5 {
		_ = os.WriteFile(filepath.Join(dir, strings.Repeat("f", i+1)+".txt"), []byte(""), 0644)
	}

	out, err := listDirectory(dir, 3)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(out, "truncated") {
		t.Errorf("expected truncation notice, got: %s", out)
	}
}

func TestListDirectory_NotFound(t *testing.T) {
	_, err := listDirectory("/nonexistent/dir", listDefaultLimit)
	if err == nil {
		t.Fatal("expected error for missing directory")
	}
}

package filesystem

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteFile_CreateNew(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "new.txt")

	out, err := writeFile(path, "hello")
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(out, "created") {
		t.Errorf("expected 'created' in output, got: %s", out)
	}

	data, _ := os.ReadFile(path)
	if string(data) != "hello" {
		t.Errorf("unexpected file content: %s", data)
	}
}

func TestWriteFile_Overwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "existing.txt")
	_ = os.WriteFile(path, []byte("old"), 0644)

	out, err := writeFile(path, "new content")
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(out, "overwritten") {
		t.Errorf("expected 'overwritten' in output, got: %s", out)
	}

	data, _ := os.ReadFile(path)
	if string(data) != "new content" {
		t.Errorf("unexpected file content: %s", data)
	}
}

func TestWriteFile_CreatesParentDirs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a", "b", "c", "file.txt")

	if err := ensureDirectoryExists(path); err != nil {
		t.Fatal(err)
	}

	_, err := writeFile(path, "content")
	if err != nil {
		t.Fatal(err)
	}
}

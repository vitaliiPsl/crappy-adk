package filesystem

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEditFile_SingleReplace(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	_ = os.WriteFile(path, []byte("foo bar baz"), 0644)

	_, err := editFile(path, "bar", "qux", false)
	if err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(path)
	if string(data) != "foo qux baz" {
		t.Errorf("unexpected content: %s", data)
	}
}

func TestEditFile_ReplaceAll(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	_ = os.WriteFile(path, []byte("a a a"), 0644)

	out, err := editFile(path, "a", "b", true)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(out, "3") {
		t.Errorf("expected count 3 in output, got: %s", out)
	}

	data, _ := os.ReadFile(path)
	if string(data) != "b b b" {
		t.Errorf("unexpected content: %s", data)
	}
}

func TestEditFile_NotFound(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	_ = os.WriteFile(path, []byte("hello world"), 0644)

	_, err := editFile(path, "missing", "x", false)
	if err == nil {
		t.Fatal("expected error when old_string not found")
	}
}

func TestEditFile_AmbiguousMatch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	_ = os.WriteFile(path, []byte("x x x"), 0644)

	_, err := editFile(path, "x", "y", false)
	if err == nil {
		t.Fatal("expected error for ambiguous match without replace_all")
	}
}

func TestEditFile_Delete(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	_ = os.WriteFile(path, []byte("hello world"), 0644)

	_, err := editFile(path, " world", "", false)
	if err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(path)
	if string(data) != "hello" {
		t.Errorf("unexpected content: %s", data)
	}
}

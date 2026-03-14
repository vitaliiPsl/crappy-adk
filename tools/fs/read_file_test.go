package filesystem

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadFileLines_FullFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	_ = os.WriteFile(path, []byte("line0\nline1\nline2\n"), 0644)

	out, err := readFileLines(path, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{"     0 line0", "     1 line1", "     2 line2"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected output to contain %q, got:\n%s", want, out)
		}
	}
}

func TestReadFileLines_Range(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	_ = os.WriteFile(path, []byte("a\nb\nc\nd\n"), 0644)

	start, end := 1, 2

	out, err := readFileLines(path, &start, &end)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(out, "b") || !strings.Contains(out, "c") {
		t.Errorf("expected lines 1-2 in output, got: %s", out)
	}

	if strings.Contains(out, "a") || strings.Contains(out, "d") {
		t.Errorf("output should not contain lines outside range, got: %s", out)
	}
}

func TestReadFileLines_StartBeyondFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	_ = os.WriteFile(path, []byte("only one line\n"), 0644)

	start := 99

	_, err := readFileLines(path, &start, nil)
	if err == nil {
		t.Fatal("expected error for start beyond file length")
	}
}

func TestReadFileLines_NotFound(t *testing.T) {
	_, err := readFileLines("/nonexistent/path/file.txt", nil, nil)
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

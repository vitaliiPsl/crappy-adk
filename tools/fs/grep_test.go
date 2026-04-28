package filesystem

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupGrepDir(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()

	files := map[string]string{
		"main.go":               "package main\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n",
		"internal/util.go":      "package internal\n\nfunc Helper() string {\n\treturn \"helper\"\n}\n",
		"internal/util_test.go": "package internal\n\nfunc TestHelper(t *testing.T) {\n\tHelper()\n}\n",
		"README.md":             "# Project\n\nThis is a project.\n",
	}

	for name, content := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		_ = os.MkdirAll(filepath.Dir(path), 0755)
		_ = os.WriteFile(path, []byte(content), 0644)
	}

	return dir
}

func TestGrepSearch_BasicMatch(t *testing.T) {
	dir := setupGrepDir(t)

	out, err := grepSearch(GrepInput{Pattern: "func", Path: dir})
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(out, "main.go") {
		t.Errorf("expected main.go in output, got:\n%s", out)
	}

	if !strings.Contains(out, "util.go") {
		t.Errorf("expected util.go in output, got:\n%s", out)
	}
}

func TestGrepSearch_SingleFile(t *testing.T) {
	dir := setupGrepDir(t)
	path := filepath.Join(dir, "main.go")

	out, err := grepSearch(GrepInput{Pattern: "Println", Path: path})
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(out, "Println") {
		t.Errorf("expected Println in output, got:\n%s", out)
	}

	if strings.Contains(out, "util.go") {
		t.Errorf("should only search specified file, got:\n%s", out)
	}
}

func TestGrepSearch_WithIncludeFilter(t *testing.T) {
	dir := setupGrepDir(t)

	out, err := grepSearch(GrepInput{Pattern: "func", Path: dir, Include: "*.go"})
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(out, "README.md") {
		t.Errorf("README.md should be excluded by include filter, got:\n%s", out)
	}

	if !strings.Contains(out, "main.go") {
		t.Errorf("expected main.go in output, got:\n%s", out)
	}
}

func TestGrepSearch_CaseInsensitive(t *testing.T) {
	dir := setupGrepDir(t)

	out, err := grepSearch(GrepInput{Pattern: "HELPER", Path: dir, CaseInsensitive: true})
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(out, "util.go") {
		t.Errorf("expected case-insensitive match in util.go, got:\n%s", out)
	}
}

func TestGrepSearch_CaseSensitiveNoMatch(t *testing.T) {
	dir := setupGrepDir(t)

	out, err := grepSearch(GrepInput{Pattern: "HELPER", Path: dir, CaseInsensitive: false})
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(out, "no matches") {
		t.Errorf("expected no matches for case-sensitive HELPER, got:\n%s", out)
	}
}

func TestGrepSearch_NoMatches(t *testing.T) {
	dir := setupGrepDir(t)

	out, err := grepSearch(GrepInput{Pattern: "nonexistent_xyz_abc", Path: dir})
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(out, "no matches") {
		t.Errorf("expected no matches message, got: %s", out)
	}
}

func TestGrepSearch_OutputFormat(t *testing.T) {
	dir := setupGrepDir(t)
	path := filepath.Join(dir, "main.go")

	out, err := grepSearch(GrepInput{Pattern: "Println", Path: path})
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(out, ":4: ") {
		t.Errorf("expected file:line: content format, got:\n%s", out)
	}
}

func TestGrepSearch_InvalidPattern(t *testing.T) {
	dir := setupGrepDir(t)

	_, err := grepSearch(GrepInput{Pattern: "[invalid", Path: dir})
	if err == nil {
		t.Fatal("expected error for invalid regex pattern")
	}
}

func TestGrepSearch_Truncated(t *testing.T) {
	dir := t.TempDir()

	var lines []string
	for range grepMaxMatches + 10 {
		lines = append(lines, "match line here")
	}

	_ = os.WriteFile(filepath.Join(dir, "big.txt"), []byte(strings.Join(lines, "\n")), 0644)

	out, err := grepSearch(GrepInput{Pattern: "match", Path: dir})
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(out, "truncated") {
		t.Errorf("expected truncation notice, got: %s", out)
	}
}

func TestGrepSearch_NotFound(t *testing.T) {
	_, err := grepSearch(GrepInput{Pattern: "foo", Path: "/nonexistent/path"})
	if err == nil {
		t.Fatal("expected error for missing path")
	}
}

package filesystem

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vitaliiPsl/crappy-adk/utils/glob"
)

func setupGlobDir(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()

	files := []string{
		"main.go",
		"main_test.go",
		"README.md",
		"internal/server/server.go",
		"internal/server/server_test.go",
		"internal/config/config.go",
		"cmd/main.go",
	}

	for _, f := range files {
		path := filepath.Join(dir, filepath.FromSlash(f))
		_ = os.MkdirAll(filepath.Dir(path), 0755)
		_ = os.WriteFile(path, []byte(""), 0644)
	}

	return dir
}

func TestGlobFiles_AllGoFiles(t *testing.T) {
	dir := setupGlobDir(t)

	out, err := globFiles(dir, "**/*.go")
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{"main.go", "main_test.go", "internal/server/server.go", "cmd/main.go"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}

	if strings.Contains(out, "README.md") {
		t.Errorf("README.md should not match **/*.go, got:\n%s", out)
	}
}

func TestGlobFiles_SpecificDir(t *testing.T) {
	dir := setupGlobDir(t)

	out, err := globFiles(dir, "internal/**/*.go")
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{"internal/server/server.go", "internal/config/config.go"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}

	if strings.Contains(out, "main.go") || strings.Contains(out, "cmd/") {
		t.Errorf("top-level and cmd files should not match internal/**/*.go, got:\n%s", out)
	}
}

func TestGlobFiles_TestFiles(t *testing.T) {
	dir := setupGlobDir(t)

	out, err := globFiles(dir, "**/*_test.go")
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{"main_test.go", "internal/server/server_test.go"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}

	if strings.Contains(out, "server.go\"") || strings.Contains(out, "config.go") {
		t.Errorf("non-test files should not match **/*_test.go, got:\n%s", out)
	}
}

func TestGlobFiles_NoMatches(t *testing.T) {
	dir := setupGlobDir(t)

	out, err := globFiles(dir, "**/*.py")
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(out, "no matches") {
		t.Errorf("expected no matches message, got: %s", out)
	}
}

func TestGlobFiles_Truncated(t *testing.T) {
	dir := t.TempDir()
	for i := range globMaxResults + 10 {
		name := filepath.Join(dir, fmt.Sprintf("file_%d.go", i))
		_ = os.WriteFile(name, []byte(""), 0644)
	}

	out, err := globFiles(dir, "*.go")
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(out, "truncated") {
		t.Errorf("expected truncation notice, got: %s", out)
	}
}

func TestGlobFiles_NotFound(t *testing.T) {
	_, err := globFiles("/nonexistent/dir", "**/*.go")
	if err == nil {
		t.Fatal("expected error for missing directory")
	}
}

func TestMatchGlobPattern(t *testing.T) {
	cases := []struct {
		pattern string
		path    string
		want    bool
	}{
		{"**/*.go", "main.go", true},
		{"**/*.go", "internal/server/server.go", true},
		{"**/*.go", "README.md", false},
		{"internal/**/*.go", "internal/server/server.go", true},
		{"internal/**/*.go", "cmd/main.go", false},
		{"*.go", "main.go", true},
		{"*.go", "internal/main.go", false},
		{"**/*_test.go", "main_test.go", true},
		{"**/*_test.go", "internal/server/server_test.go", true},
		{"**/*_test.go", "server.go", false},
	}

	for _, tc := range cases {
		got := glob.Match(tc.pattern, tc.path)
		if got != tc.want {
			t.Errorf("glob.Match(%q, %q) = %v, want %v", tc.pattern, tc.path, got, tc.want)
		}
	}
}

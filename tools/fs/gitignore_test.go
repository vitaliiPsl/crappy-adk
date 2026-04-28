package filesystem

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseGitignoreRule_Comment(t *testing.T) {
	_, ok := parseGitignoreRule("# this is a comment")
	if ok {
		t.Error("comment line should not produce a rule")
	}
}

func TestParseGitignoreRule_Empty(t *testing.T) {
	_, ok := parseGitignoreRule("")
	if ok {
		t.Error("empty line should not produce a rule")
	}
}

func TestParseGitignoreRule_SimpleFile(t *testing.T) {
	rule, ok := parseGitignoreRule("*.go")
	if !ok {
		t.Fatal("expected rule to be parsed")
	}

	if rule.pattern != "**/*.go" {
		t.Errorf("expected pattern **/*.go, got %q", rule.pattern)
	}

	if rule.negate || rule.dirOnly {
		t.Errorf("expected plain rule, got negate=%v dirOnly=%v", rule.negate, rule.dirOnly)
	}
}

func TestParseGitignoreRule_DirOnly(t *testing.T) {
	rule, ok := parseGitignoreRule("node_modules/")
	if !ok {
		t.Fatal("expected rule to be parsed")
	}

	if !rule.dirOnly {
		t.Error("expected dirOnly=true for trailing slash pattern")
	}

	if rule.pattern != "**/node_modules" {
		t.Errorf("expected pattern **/node_modules, got %q", rule.pattern)
	}
}

func TestParseGitignoreRule_Negation(t *testing.T) {
	rule, ok := parseGitignoreRule("!important.go")
	if !ok {
		t.Fatal("expected rule to be parsed")
	}

	if !rule.negate {
		t.Error("expected negate=true for ! prefix")
	}
}

func TestParseGitignoreRule_Anchored(t *testing.T) {
	rule, ok := parseGitignoreRule("/vendor")
	if !ok {
		t.Fatal("expected rule to be parsed")
	}

	if rule.pattern != "vendor" {
		t.Errorf("expected leading / to be stripped, got %q", rule.pattern)
	}
}

func TestParseGitignoreRule_PathPattern(t *testing.T) {
	rule, ok := parseGitignoreRule("src/*.go")
	if !ok {
		t.Fatal("expected rule to be parsed")
	}

	if rule.pattern != "src/*.go" {
		t.Errorf("expected path pattern unchanged, got %q", rule.pattern)
	}
}

func TestGitignoreMatcher_Match(t *testing.T) {
	cases := []struct {
		lines []string
		path  string
		isDir bool
		want  bool
	}{
		{[]string{"*.log"}, "app.log", false, true},
		{[]string{"*.log"}, "main.go", false, false},
		{[]string{"node_modules/"}, "node_modules", true, true},
		{[]string{"node_modules/"}, "node_modules", false, false},
		{[]string{"*.go", "!main.go"}, "main.go", false, false},
		{[]string{"*.go", "!main.go"}, "util.go", false, true},
		{[]string{"dist/"}, "dist", true, true},
		{[]string{"src/*.go"}, "src/main.go", false, true},
		{[]string{"src/*.go"}, "other/main.go", false, false},
	}

	for _, tc := range cases {
		m := &gitignoreMatcher{}
		for _, line := range tc.lines {
			if rule, ok := parseGitignoreRule(line); ok {
				m.rules = append(m.rules, rule)
			}
		}

		got := m.match(tc.path, tc.isDir)
		if got != tc.want {
			t.Errorf("match(%q, isDir=%v) with rules %v = %v, want %v",
				tc.path, tc.isDir, tc.lines, got, tc.want)
		}
	}
}

func TestWalkWithGitignore_SkipsIgnored(t *testing.T) {
	dir := t.TempDir()

	_ = os.MkdirAll(filepath.Join(dir, "node_modules", "pkg"), 0755)
	_ = os.WriteFile(filepath.Join(dir, "node_modules", "pkg", "index.js"), []byte(""), 0644)
	_ = os.WriteFile(filepath.Join(dir, "main.go"), []byte(""), 0644)
	_ = os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("node_modules/\n"), 0644)

	var visited []string

	_ = walkWithGitignore(dir, func(path string, _ fs.DirEntry, _ error) error {
		rel, _ := filepath.Rel(dir, path)
		visited = append(visited, filepath.ToSlash(rel))

		return nil
	})

	for _, v := range visited {
		if v == "node_modules/pkg/index.js" || v == "node_modules" {
			t.Errorf("ignored path %q should not have been visited", v)
		}
	}

	found := false
	for _, v := range visited {
		if v == "main.go" {
			found = true
		}
	}

	if !found {
		t.Error("main.go should have been visited")
	}
}

func TestWalkWithGitignore_SkipsGitDir(t *testing.T) {
	dir := t.TempDir()

	_ = os.MkdirAll(filepath.Join(dir, ".git", "objects"), 0755)
	_ = os.WriteFile(filepath.Join(dir, ".git", "HEAD"), []byte("ref: refs/heads/main"), 0644)
	_ = os.WriteFile(filepath.Join(dir, "main.go"), []byte(""), 0644)

	var visited []string

	_ = walkWithGitignore(dir, func(path string, _ fs.DirEntry, _ error) error {
		rel, _ := filepath.Rel(dir, path)
		visited = append(visited, filepath.ToSlash(rel))

		return nil
	})

	for _, v := range visited {
		if v == ".git" || strings.HasPrefix(v, ".git/") {
			t.Errorf(".git path %q should not have been visited", v)
		}
	}
}

func TestWalkWithGitignore_AncestorGitignoreApplies(t *testing.T) {
	root := t.TempDir()

	_ = os.Mkdir(filepath.Join(root, ".git"), 0755)
	_ = os.WriteFile(filepath.Join(root, ".gitignore"), []byte("*.log\n"), 0644)

	subdir := filepath.Join(root, "src")
	_ = os.MkdirAll(subdir, 0755)
	_ = os.WriteFile(filepath.Join(subdir, "main.go"), []byte(""), 0644)
	_ = os.WriteFile(filepath.Join(subdir, "debug.log"), []byte(""), 0644)

	var visited []string

	_ = walkWithGitignore(subdir, func(path string, _ fs.DirEntry, _ error) error {
		rel, _ := filepath.Rel(root, path)
		visited = append(visited, filepath.ToSlash(rel))

		return nil
	})

	for _, v := range visited {
		if v == "src/debug.log" {
			t.Errorf("debug.log should be ignored by ancestor .gitignore, but was visited")
		}
	}

	found := false
	for _, v := range visited {
		if v == "src/main.go" {
			found = true
		}
	}

	if !found {
		t.Error("src/main.go should have been visited")
	}
}

func TestWalkWithGitignore_NegationUnignores(t *testing.T) {
	dir := t.TempDir()

	_ = os.WriteFile(filepath.Join(dir, "app.log"), []byte(""), 0644)
	_ = os.WriteFile(filepath.Join(dir, "important.log"), []byte(""), 0644)
	_ = os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("*.log\n!important.log\n"), 0644)

	var visited []string

	_ = walkWithGitignore(dir, func(path string, _ fs.DirEntry, _ error) error {
		rel, _ := filepath.Rel(dir, path)
		visited = append(visited, filepath.ToSlash(rel))

		return nil
	})

	foundImportant := false
	for _, v := range visited {
		if v == "app.log" {
			t.Errorf("app.log should be ignored, got it in visited")
		}

		if v == "important.log" {
			foundImportant = true
		}
	}

	if !foundImportant {
		t.Error("important.log should be visited due to negation rule")
	}
}

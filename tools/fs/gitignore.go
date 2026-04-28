package filesystem

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/vitaliiPsl/crappy-adk/utils/glob"
)

const gitignoreFilename = ".gitignore"

type gitignoreRule struct {
	pattern string
	negate  bool
	dirOnly bool
}

type gitignoreMatcher struct {
	dir   string
	rules []gitignoreRule
}

func (m *gitignoreMatcher) match(relPath string, isDir bool) bool {
	relPath = filepath.ToSlash(relPath)

	matched := false
	for _, rule := range m.rules {
		if rule.dirOnly && !isDir {
			continue
		}

		if glob.Match(rule.pattern, relPath) {
			matched = !rule.negate
		}
	}

	return matched
}

func loadGitignore(dir string) *gitignoreMatcher {
	data, err := os.ReadFile(filepath.Join(dir, gitignoreFilename))
	if err != nil {
		return nil
	}

	var rules []gitignoreRule
	for _, line := range strings.Split(string(data), "\n") {
		if rule, ok := parseGitignoreRule(line); ok {
			rules = append(rules, rule)
		}
	}

	if len(rules) == 0 {
		return nil
	}

	return &gitignoreMatcher{dir: dir, rules: rules}
}

func parseGitignoreRule(line string) (gitignoreRule, bool) {
	line = strings.TrimRight(line, "\r")

	if line == "" || strings.HasPrefix(line, "#") {
		return gitignoreRule{}, false
	}

	negate := strings.HasPrefix(line, "!")
	if negate {
		line = line[1:]
	}

	dirOnly := strings.HasSuffix(line, "/")
	if dirOnly {
		line = strings.TrimSuffix(line, "/")
	}

	if line == "" {
		return gitignoreRule{}, false
	}

	var pattern string
	switch {
	case !strings.Contains(line, "/"):
		pattern = "**/" + line
	case strings.HasPrefix(line, "/"):
		pattern = line[1:]
	default:
		pattern = line
	}

	return gitignoreRule{pattern: pattern, negate: negate, dirOnly: dirOnly}, true
}

func isIgnoredByAny(matchers map[string]*gitignoreMatcher, path string, isDir bool) bool {
	for dir, m := range matchers {
		if !isAncestorOrSelf(dir, path) {
			continue
		}

		rel, err := filepath.Rel(dir, path)
		if err != nil {
			continue
		}

		if m.match(rel, isDir) {
			return true
		}
	}

	return false
}

func loadAncestorGitignores(root string) map[string]*gitignoreMatcher {
	matchers := make(map[string]*gitignoreMatcher)

	dir := filepath.Dir(root)
	for {
		if m := loadGitignore(dir); m != nil {
			matchers[dir] = m
		}

		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			break
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}

		dir = parent
	}

	return matchers
}

func walkWithGitignore(root string, fn func(path string, d fs.DirEntry, err error) error) error {
	matchers := loadAncestorGitignores(root)

	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			if d.Name() == ".git" {
				return fs.SkipDir
			}

			if m := loadGitignore(path); m != nil {
				matchers[path] = m
			}
		}

		if isIgnoredByAny(matchers, path, d.IsDir()) {
			if d.IsDir() {
				return fs.SkipDir
			}

			return nil
		}

		return fn(path, d, err)
	})
}

func isAncestorOrSelf(ancestor, path string) bool {
	return path == ancestor || strings.HasPrefix(path, ancestor+string(filepath.Separator))
}

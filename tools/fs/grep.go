package filesystem

import (
	"bufio"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/vitaliiPsl/crappy-adk/kit"
	"github.com/vitaliiPsl/crappy-adk/x/tool"
)

const (
	grepName        = "grep"
	grepDescription = "Search file contents for a regex pattern. Returns matching lines in file:line: content format. Searches recursively when path is a directory."

	grepMaxMatches = 100
)

type GrepInput struct {
	Pattern         string `json:"pattern" jsonschema:"Regular expression pattern to search for"`
	Path            string `json:"path" jsonschema:"Absolute path to a file or directory to search in"`
	Include         string `json:"include,omitempty" jsonschema:"Glob pattern to filter files e.g. *.txt or *.go. Only applies when path is a directory."`
	CaseInsensitive bool   `json:"case_insensitive,omitempty" jsonschema:"Whether to perform case-insensitive matching"`
}

func NewGrep() kit.Tool {
	return tool.MustFunction(
		grepName,
		grepDescription,
		func(_ context.Context, input GrepInput) (string, error) {
			return grepSearch(input)
		},
	)
}

func grepSearch(input GrepInput) (string, error) {
	pattern := input.Pattern
	if input.CaseInsensitive {
		pattern = "(?i)" + pattern
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return "", fmt.Errorf("invalid pattern: %w", err)
	}

	info, err := os.Stat(input.Path)
	if err != nil {
		return "", fmt.Errorf("path not found: %w", err)
	}

	var results []string

	if !info.IsDir() {
		results, err = searchFile(re, input.Path)
		if err != nil {
			return "", err
		}
	} else {
		results, err = searchDir(re, input.Path, input.Include)
		if err != nil {
			return "", err
		}
	}

	if len(results) == 0 {
		return "(no matches)", nil
	}

	truncated := false
	if len(results) > grepMaxMatches {
		results = results[:grepMaxMatches]
		truncated = true
	}

	var out strings.Builder
	for _, r := range results {
		out.WriteString(r)
		out.WriteString("\n")
	}

	if truncated {
		fmt.Fprintf(&out, "\n... truncated at %d matches\n", grepMaxMatches)
	}

	return out.String(), nil
}

func searchDir(re *regexp.Regexp, dir, include string) ([]string, error) {
	files, err := collectFiles(dir, include)
	if err != nil {
		return nil, err
	}

	return searchFiles(re, files)
}

func collectFiles(dir, include string) ([]string, error) {
	var files []string

	err := walkWithGitignore(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}

		if include != "" {
			matched, matchErr := filepath.Match(include, filepath.Base(path))
			if matchErr != nil || !matched {
				return matchErr
			}
		}

		files = append(files, path)

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to walk directory: %w", err)
	}

	return files, nil
}

func searchFiles(re *regexp.Regexp, files []string) ([]string, error) {
	type fileResult struct {
		path    string
		matches []string
		err     error
	}

	jobs := make(chan string)
	resultsCh := make(chan fileResult, len(files))

	var wg sync.WaitGroup

	for range runtime.NumCPU() {
		wg.Go(func() {
			for path := range jobs {
				matches, err := searchFile(re, path)
				resultsCh <- fileResult{path: path, matches: matches, err: err}
			}
		})
	}

	for _, f := range files {
		jobs <- f
	}

	close(jobs)
	wg.Wait()
	close(resultsCh)

	var fileResults []fileResult

	for r := range resultsCh {
		if r.err != nil {
			return nil, r.err
		}

		if len(r.matches) > 0 {
			fileResults = append(fileResults, r)
		}
	}

	sort.Slice(fileResults, func(i, j int) bool {
		return fileResults[i].path < fileResults[j].path
	})

	var results []string

	for _, r := range fileResults {
		results = append(results, r.matches...)
	}

	return results, nil
}

func searchFile(re *regexp.Regexp, path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open %s: %w", path, err)
	}

	defer func() { _ = f.Close() }()

	var matches []string

	lineNum := 0

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	for scanner.Scan() {
		lineNum++

		line := scanner.Text()
		if re.MatchString(line) {
			matches = append(matches, fmt.Sprintf("%s:%d: %s", path, lineNum, line))
		}
	}

	return matches, scanner.Err()
}

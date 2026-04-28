package glob

import (
	"path/filepath"
	"strings"
)

// Match reports whether path matches pattern.
// The pattern syntax follows filepath.Match for each path segment and supports
// ** as a recursive wildcard across path segments.
func Match(pattern, path string) bool {
	return matchSegs(
		strings.Split(filepath.ToSlash(pattern), "/"),
		strings.Split(filepath.ToSlash(path), "/"),
	)
}

func matchSegs(patSegs, pathSegs []string) bool {
	pi, si := 0, 0
	for pi < len(patSegs) {
		if patSegs[pi] == "**" {
			pi++
			if pi == len(patSegs) {
				return true
			}

			for si <= len(pathSegs) {
				if matchSegs(patSegs[pi:], pathSegs[si:]) {
					return true
				}

				si++
			}

			return false
		}

		if si >= len(pathSegs) {
			return false
		}

		matched, err := filepath.Match(patSegs[pi], pathSegs[si])
		if err != nil || !matched {
			return false
		}

		pi++
		si++
	}

	return si == len(pathSegs)
}

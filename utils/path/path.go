package path

import (
	"os"
	"path/filepath"
	"strings"
)

const homeDirPrefix = "~/"

func ExpandHome(path string) string {
	if strings.HasPrefix(path, homeDirPrefix) {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, path[len(homeDirPrefix):])
		}
	}

	return path
}

package optimizer

import (
	"path/filepath"
	"strings"
)

// buildExcludeMatchers and isHardExcluded are in optimizer.go

// isWithinRoot and isDangerousRoot are in optimizer.go

func isPathSeparator(r rune) bool {
	return r == '/' || r == '\\'
}

func normalizeAbs(path string) string {
	p := filepath.Clean(path)
	p = filepath.ToSlash(p)
	return strings.ToLower(p)
}

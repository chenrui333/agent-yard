package cli

import "path/filepath"

func sameFilesystemPath(left, right string) bool {
	return canonicalPath(left) == canonicalPath(right)
}

func canonicalPath(path string) string {
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	if evaluated, err := filepath.EvalSymlinks(path); err == nil {
		path = evaluated
	}
	return filepath.Clean(path)
}

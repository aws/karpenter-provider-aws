package project

import (
	"path/filepath"
	"runtime"
)

func RelativeToRoot(path string) string {
	_, file, _, _ := runtime.Caller(0)
	manifestsRoot := filepath.Join(filepath.Dir(file), "..", "..", "..")
	return filepath.Join(manifestsRoot, path)
}

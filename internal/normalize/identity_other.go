//go:build !linux && !darwin && !windows

package normalize

import (
	"os"
	"path/filepath"
)

func directoryKey(path string, _ os.FileInfo) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	canonical, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", err
	}
	return canonical, nil
}

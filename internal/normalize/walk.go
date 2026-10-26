package normalize

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

func Walk(roots []string, config Config) ([]string, error) {
	var paths []string
	var errs []error
	for _, root := range roots {
		p, err := walkRoot(root, config)
		paths = append(paths, p...)
		if err != nil {
			errs = append(errs, err)
		}
	}
	return dedupe(paths), errors.Join(errs...)
}

// dedupe removes exact-duplicate paths, keeping the first occurrence and
// preserving order.
func dedupe(paths []string) []string {
	seen := make(map[string]struct{}, len(paths))
	out := paths[:0]
	for _, p := range paths {
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	return out
}

func walkRoot(root string, config Config) ([]string, error) {
	root = filepath.Clean(root)
	if shouldSkip(root) {
		fmt.Fprintf(config.errWriter(), "fpn: %s: skipped\n", root)
		return nil, nil
	}

	info, err := os.Lstat(root)
	if err != nil {
		return nil, fmt.Errorf("fpn: %s: %w", root, err)
	}

	if !info.IsDir() || !config.Recursive {
		return []string{root}, nil
	}

	var paths []string
	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if shouldSkip(p) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		paths = append(paths, p)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("fpn: %s: %w", root, err)
	}
	return paths, nil
}

func shouldSkip(path string) bool {
	base := filepath.Base(path)
	return base == ".git" || base == ".DS_Store"
}

package normalize

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Walk returns physical absolute paths for one input. Recursive traversal emits
// files and symlinks only; directories, including the root, are never renamed.
func Walk(root string, config Config) ([]string, error) {
	if hasSkippedComponent(root) {
		_, err := fmt.Fprintf(config.errWriter(), "fpn: %s: skipped\n", displayPath(root))
		return nil, err
	}
	physical, special, err := resolveWalkRoot(root)
	if err != nil {
		return nil, fmt.Errorf("fpn: %s: %w", displayPath(root), err)
	}
	if hasSkippedComponent(physical) {
		_, err := fmt.Fprintf(config.errWriter(), "fpn: %s: skipped\n", displayPath(physical))
		return nil, err
	}
	info, err := os.Lstat(physical)
	if err != nil {
		return nil, fmt.Errorf("fpn: %s: %w", displayPath(physical), err)
	}
	if !info.IsDir() || !config.Recursive {
		if special {
			return nil, nil
		}
		return []string{physical}, nil
	}
	// Recursive roots are not passed to the rename planner, but their supplied
	// basename must still match the stored entry exactly.
	if !special {
		var cache directoryCache
		d, err := cache.get(filepath.Dir(physical))
		if err != nil {
			return nil, fmt.Errorf("fpn: resolve parent of %s: %w", displayPath(physical), err)
		}
		if err := d.validateSource(filepath.Base(physical)); err != nil {
			return nil, err
		}
	}
	var paths []string
	err = filepath.WalkDir(physical, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if shouldSkip(p) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if !d.IsDir() {
			paths = append(paths, p)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("fpn: %s: %w", displayPath(physical), err)
	}
	return paths, nil
}

// resolveWalkRoot resolves parents before lexical cleaning can discard .. or
// a nonexistent component. The final entry is deliberately not resolved, even
// when the argument has a trailing slash: final symlinks are entries, not trees.
func resolveWalkRoot(root string) (physical string, special bool, err error) {
	if root == "" {
		return "", false, fmt.Errorf("fpn: path argument must not be empty")
	}
	root = filepath.FromSlash(root)
	raw := strings.TrimRight(root, string(filepath.Separator))
	if volume := filepath.VolumeName(root); volume != "" && raw == volume && len(root) > len(volume) {
		raw = volume + string(filepath.Separator)
	}
	if raw == "" {
		raw = string(filepath.Separator)
	}
	parent, base := filepath.Split(raw) // Unlike Dir, Split does not clean.
	special = base == "" || base == "." || base == ".."
	if special {
		parent = raw
	} else if parent == "" {
		parent = "."
	}
	if !filepath.IsAbs(parent) && filepath.VolumeName(parent) == "" {
		var cwd string
		cwd, err = os.Getwd()
		if err != nil {
			return
		}
		parent = cwd + string(filepath.Separator) + parent
	}
	physical, err = filepath.EvalSymlinks(parent)
	if err != nil {
		return
	}
	if !special {
		physical = filepath.Join(physical, base)
	}
	return
}

func hasSkippedComponent(path string) bool {
	for _, part := range strings.Split(filepath.FromSlash(path), string(filepath.Separator)) {
		if shouldSkip(part) {
			return true
		}
	}
	return false
}

func shouldSkip(path string) bool {
	base := filepath.Base(path)
	return strings.EqualFold(base, ".git") || strings.EqualFold(base, ".DS_Store")
}

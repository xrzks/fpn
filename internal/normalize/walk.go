package normalize

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func Walk(roots []string, config Config) ([]string, error) {
	var paths []string
	var errs []error
	seen := make(map[string]bool)
	state := newWalkState()
	for _, root := range roots {
		// Deduplicate before resolution or directory reads, but do not clean:
		// link/../file and file need not name the same entry.
		if config.Recursive && seen[root] {
			continue
		}
		seen[root] = true
		p, err := walkRootWithCompleted(root, config, state)
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

// Traversing a directory's contents and emitting its renameable entry are
// separate operations: a dot root does only the former. Keys are directory
// identities so alternate path spellings share both kinds of state.
type walkState struct {
	completed map[string]bool
	emitted   map[string]bool
}

func newWalkState() *walkState {
	return &walkState{completed: make(map[string]bool), emitted: make(map[string]bool)}
}

func walkRoot(root string, config Config) ([]string, error) {
	return walkRootWithCompleted(root, config, newWalkState())
}

// resolveWalkRoot resolves parents before lexical cleaning can discard .. or
// a nonexistent component. The final entry is deliberately not resolved, even
// when the argument has a trailing slash: final symlinks are entries, not trees.
func resolveWalkRoot(root string) (display, physical string, special bool, err error) {
	root = filepath.FromSlash(root)
	raw := strings.TrimRight(root, string(filepath.Separator))
	if volume := filepath.VolumeName(root); volume != "" && raw == volume && len(root) > len(volume) {
		raw = volume + string(filepath.Separator)
	}
	if raw == "" {
		if root == "" {
			raw = "."
		} else {
			raw = string(filepath.Separator)
		}
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
	display = filepath.Clean(raw)
	// Ordinary argument spelling is useful in reports. With .., however,
	// cleaning is not safe; use the already resolved physical spelling.
	for _, part := range strings.Split(raw, string(filepath.Separator)) {
		if part == ".." {
			display = physical
			break
		}
	}
	return
}

func walkRootWithCompleted(root string, config Config, state *walkState) ([]string, error) {
	if hasSkippedComponent(root) {
		_, err := fmt.Fprintf(config.errWriter(), "fpn: %s: skipped\n", displayPath(root))
		return nil, err
	}
	display, physical, special, err := resolveWalkRoot(root)
	if err != nil {
		return nil, fmt.Errorf("fpn: %s: %w", displayPath(root), err)
	}
	if hasSkippedComponent(physical) {
		_, err := fmt.Fprintf(config.errWriter(), "fpn: %s: skipped\n", displayPath(root))
		return nil, err
	}
	info, err := os.Lstat(physical)
	if err != nil {
		return nil, fmt.Errorf("fpn: %s: %w", displayPath(root), err)
	}
	if !info.IsDir() || !config.Recursive {
		if special {
			return nil, nil
		}
		return []string{display}, nil
	}
	var paths []string
	pending := newWalkState()
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
		entry := display
		if p != physical {
			rel, err := filepath.Rel(physical, p)
			if err != nil {
				return err
			}
			entry = filepath.Join(display, rel)
		}
		if !d.IsDir() {
			paths = append(paths, entry)
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		key, err := directoryKey(p, info)
		if err != nil {
			return err
		}
		// . and .. select contents, never the physical directory's name.
		if !(p == physical && special) && !state.emitted[key] && !pending.emitted[key] {
			paths = append(paths, entry)
			pending.emitted[key] = true
		}
		if state.completed[key] || pending.completed[key] {
			return fs.SkipDir
		}
		pending.completed[key] = true
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("fpn: %s: %w", displayPath(root), err)
	}
	// Failed walks must not suppress a later, independently accessible root.
	for key := range pending.completed {
		state.completed[key] = true
	}
	for key := range pending.emitted {
		state.emitted[key] = true
	}
	return paths, nil
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

package normalize

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// NormalizePaths preflights the entire batch before reporting or executing any
// renames. Recursive callers supply only files; a nonrecursive caller may supply
// an explicit directory.
func NormalizePaths(paths []string, config Config) error {
	type entryID struct {
		parent *directory
		name   string
	}
	var cache directoryCache
	var plan []renameOp
	byTarget := make(map[entryID][]int)
	for _, path := range paths {
		if path == "" {
			return fmt.Errorf("fpn: invalid normalized basename for empty path")
		}
		physical, special, err := resolveWalkRoot(path)
		if err != nil {
			return fmt.Errorf("fpn: resolve %s: %w", displayPath(path), err)
		}
		if special {
			continue
		}
		d, err := cache.get(filepath.Dir(physical))
		if err != nil {
			return fmt.Errorf("fpn: resolve parent of %s: %w", displayPath(path), err)
		}
		if _, err := os.Lstat(physical); err != nil {
			return fmt.Errorf("fpn: %s: %w", displayPath(physical), err)
		}
		base := filepath.Base(physical)
		if err := d.validateSource(base); err != nil {
			return err
		}
		if shouldSkip(base) || hasSkippedComponent(d.path) {
			continue
		}
		normalized := normalizeBaseName(base)
		if normalized == base {
			continue
		}
		if normalized == "" || normalized == "." || normalized == ".." {
			return fmt.Errorf("fpn: invalid normalized basename %q for %q: must not be empty, . or ..", normalized, physical)
		}
		target := entryID{d, d.nameKey(normalized)}
		byTarget[target] = append(byTarget[target], len(plan))
		plan = append(plan, renameOp{from: filepath.Join(d.path, base), to: filepath.Join(d.path, normalized)})
	}
	var conflicts []string
	for _, items := range byTarget {
		if len(items) > 1 {
			var sources []string
			for _, index := range items {
				sources = append(sources, displayPath(plan[index].from))
			}
			sort.Strings(sources)
			conflicts = append(conflicts, fmt.Sprintf("collision: %s would be produced by multiple sources: %s",
				displayPath(plan[items[0]].to), strings.Join(sources, ", ")))
			continue
		}
		op := plan[items[0]]
		_, err := os.Lstat(op.to)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("fpn: inspect destination %s: %w", displayPath(op.to), err)
		}
		// Any existing destination is unsupported, even a lookup alias of
		// the source. Never use inode equality to excuse hardlinks or symlinks.
		conflicts = append(conflicts, fmt.Sprintf("collision: %s would overwrite existing file or alias (from %s)",
			displayPath(op.to), displayPath(op.from)))
	}
	if len(conflicts) > 0 {
		sort.Strings(conflicts)
		collisionErr := fmt.Errorf("%d collision(s); no renames performed", len(conflicts))
		for _, conflict := range conflicts {
			if _, err := fmt.Fprintln(config.errWriter(), conflict); err != nil {
				return errors.Join(collisionErr, fmt.Errorf("fpn: write diagnostics: %w", err))
			}
		}
		return collisionErr
	}

	sort.Slice(plan, func(i, j int) bool { return plan[i].from < plan[j].from })
	if config.DryRun {
		return logRenames(plan, config.writer())
	}
	for i, op := range plan {
		if err := renameNoReplace(op.from, op.to); err != nil {
			return errors.Join(err, logRenames(plan[:i], config.writer()))
		}
	}
	return logRenames(plan, config.writer())
}

func normalizeBaseName(name string) string {
	name = strings.Join(strings.Fields(name), " ")
	replacer := strings.NewReplacer(" ", "-", "_", "-")
	result := strings.ToLower(replacer.Replace(name))
	for strings.Contains(result, "--") {
		result = strings.ReplaceAll(result, "--", "-")
	}
	return result
}

package normalize

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func NormalizePaths(paths []string, config Config) error {
	if len(paths) == 0 {
		return nil
	}

	// Identify entries by their parent directory and basename, not path
	// spelling. Stat follows parent symlinks and SameFile also recognizes
	// case-insensitive parent aliases. Never resolve the final component:
	// a symlink there is itself the entry we intend to rename.
	type entryID struct {
		parent int
		name   string
	}
	var parents []os.FileInfo
	parentIDs := make(map[string]int)
	parentID := func(path string) (int, error) {
		dir := filepath.Dir(path)
		if id, ok := parentIDs[dir]; ok {
			return id, nil
		}
		info, err := os.Stat(dir)
		if err != nil {
			return 0, fmt.Errorf("fpn: resolve parent of %s: %w", path, err)
		}
		if !info.IsDir() {
			return 0, fmt.Errorf("fpn: parent of %s is not a directory", path)
		}
		for id, parent := range parents {
			if os.SameFile(info, parent) {
				parentIDs[dir] = id
				return id, nil
			}
		}
		id := len(parents)
		parents = append(parents, info)
		parentIDs[dir] = id
		return id, nil
	}

	type planItem struct {
		original, normalized string
		target               entryID
	}
	var plan []planItem
	seenSources := make(map[entryID]bool)
	for _, p := range paths {
		dir, base := filepath.Split(p)
		base = normalizeBaseName(base)
		if base == "" || base == "." || base == ".." {
			return fmt.Errorf("fpn: invalid normalized basename %q for %q: must not be empty, . or ..", base, p)
		}
		n := dir + base
		if p == n {
			continue
		}
		id, err := parentID(p)
		if err != nil {
			return err
		}
		source := entryID{parent: id, name: filepath.Base(p)}
		if seenSources[source] {
			continue
		}
		seenSources[source] = true
		plan = append(plan, planItem{
			original: p, normalized: n,
			target: entryID{parent: id, name: filepath.Base(n)},
		})
	}

	var pending []renameOp
	var conflicts []string
	skipped := 0

	// Group by normalized target: if several sources map to the same
	// target, report one collision for the group and rename none of them.
	byTarget := make(map[entryID][]planItem)
	for _, item := range plan {
		byTarget[item.target] = append(byTarget[item.target], item)
	}
	var filtered []planItem
	for _, item := range plan {
		if len(byTarget[item.target]) > 1 {
			continue
		}
		filtered = append(filtered, item)
	}
	for _, items := range byTarget {
		if len(items) < 2 {
			continue
		}
		target := items[0].normalized
		var sources []string
		for _, item := range items {
			sources = append(sources, item.original)
		}
		sort.Strings(sources)
		conflicts = append(conflicts, fmt.Sprintf(
			"collision: %s would be produced by multiple sources: %s",
			target, strings.Join(sources, ", "),
		))
		skipped += len(sources)
	}
	plan = filtered

	for _, item := range plan {
		original, normalized := item.original, item.normalized
		srcInfo, srcErr := os.Lstat(original)
		if srcErr != nil {
			return fmt.Errorf("fpn: %s: %w", original, srcErr)
		}
		dstInfo, dstErr := os.Lstat(normalized)
		switch {
		case os.IsNotExist(dstErr):
			pending = append(pending, renameOp{from: original, to: normalized})
		case dstErr != nil:
			conflicts = append(conflicts, fmt.Sprintf(
				"collision: %s would overwrite existing file (from %s)",
				normalized, original,
			))
			skipped++
		default:
			if os.SameFile(srcInfo, dstInfo) && isCaseAlias(normalized) {
				pending = append(pending, renameOp{from: original, to: normalized, caseOnly: true})
				continue
			}
			conflicts = append(conflicts, fmt.Sprintf(
				"collision: %s would overwrite existing file (from %s)",
				normalized, original,
			))
			skipped++
		}
	}

	sort.Strings(conflicts)
	for _, c := range conflicts {
		fmt.Fprintln(config.errWriter(), c)
	}

	sort.Slice(pending, func(i, j int) bool {
		return pending[i].from < pending[j].from
	})

	// Resolve only parent directories, before any mutation. Execution paths
	// must share an absolute, physical namespace: argument spelling (including
	// symlink aliases) says nothing about ancestry. Keep pending unchanged for
	// reporting, and never follow a final-component symlink being renamed.
	execOrder := append([]renameOp(nil), pending...)
	for i := range execOrder {
		r := &execOrder[i]
		dir, err := filepath.EvalSymlinks(filepath.Dir(r.from))
		if err != nil {
			return fmt.Errorf("fpn: resolve parent of %s: %w", r.from, err)
		}
		dir, err = filepath.Abs(dir)
		if err != nil {
			return fmt.Errorf("fpn: resolve parent of %s: %w", r.from, err)
		}
		r.from = filepath.Join(dir, filepath.Base(r.from))
		r.to = filepath.Join(dir, filepath.Base(r.to))
	}

	if config.DryRun {
		logRenames(pending, config.writer())
		if len(conflicts) > 0 {
			return fmt.Errorf("%d collision(s); would rename %d, skipped %d",
				len(conflicts), len(pending), skipped)
		}
		return nil
	}

	// Execute deepest physical paths first: renaming a directory invalidates
	// the still-original paths of its descendants. Resolved execution paths
	// also remain usable if a symlink alias itself is renamed first.
	sort.Slice(execOrder, func(i, j int) bool {
		di := strings.Count(execOrder[i].from, string(filepath.Separator))
		dj := strings.Count(execOrder[j].from, string(filepath.Separator))
		if di != dj {
			return di > dj
		}
		return execOrder[i].from < execOrder[j].from
	})

	for _, r := range execOrder {
		if err := renameOne(r); err != nil {
			return err
		}
	}

	logRenames(pending, config.writer())

	if len(conflicts) > 0 {
		return fmt.Errorf("%d collision(s); renamed %d, skipped %d",
			len(conflicts), len(pending), skipped)
	}
	return nil
}

// isCaseAlias distinguishes a case-insensitive lookup of the source from a
// separate directory entry (such as a hard link) at the destination. On an
// unreadable directory, fail closed and treat the destination as a collision.
func isCaseAlias(destination string) bool {
	entries, err := os.ReadDir(filepath.Dir(destination))
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if entry.Name() == filepath.Base(destination) {
			return false
		}
	}
	return true
}

func normalizePathName(path string) string {
	dir, base := filepath.Split(path)
	return dir + normalizeBaseName(base)
}

func normalizeBaseName(name string) string {
	name = strings.Join(strings.Fields(name), " ")
	replacer := strings.NewReplacer(" ", "-", "_", "-")
	result := strings.ToLower(replacer.Replace(name))
	// Collapse runs of hyphens (e.g. "- -" -> "-" and "a  -  b" -> "a-b").
	for strings.Contains(result, "--") {
		result = strings.ReplaceAll(result, "--", "-")
	}
	return result
}

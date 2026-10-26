package normalize

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func NormalizePaths(paths []string, config Config) error {
	type entryID struct {
		parent *directory
		name   string
	}
	type planItem struct {
		report, exec renameOp
		parent       *directory
		source       string
		target       entryID
	}
	var cache directoryCache
	var plan []planItem
	seen := make(map[entryID]bool)
	for _, path := range paths {
		if path == "" {
			return fmt.Errorf("fpn: invalid normalized basename for empty path")
		}
		display, physical, special, err := resolveWalkRoot(path)
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
		info, err := os.Lstat(physical)
		if err != nil {
			return fmt.Errorf("fpn: %s: %w", displayPath(path), err)
		}
		base, err := d.actualName(filepath.Base(physical), info)
		if err != nil {
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
			return fmt.Errorf("fpn: invalid normalized basename %q for %q: must not be empty, . or ..", normalized, path)
		}
		source := entryID{d, base}
		if seen[source] {
			continue
		}
		seen[source] = true
		dir, _ := filepath.Split(display)
		plan = append(plan, planItem{
			report: renameOp{from: dir + base, to: dir + normalized},
			exec:   renameOp{from: filepath.Join(d.path, base), to: filepath.Join(d.path, normalized)},
			parent: d, source: base, target: entryID{d, d.nameKey(normalized)},
		})
	}

	byTarget := make(map[entryID][]planItem)
	for _, item := range plan {
		byTarget[item.target] = append(byTarget[item.target], item)
	}
	var pending []planItem
	var conflicts []string
	skipped := 0
	for _, items := range byTarget {
		if len(items) > 1 {
			var sources []string
			for _, item := range items {
				sources = append(sources, displayPath(item.report.from))
			}
			sort.Strings(sources)
			conflicts = append(conflicts, fmt.Sprintf("collision: %s would be produced by multiple sources: %s",
				displayPath(items[0].report.to), strings.Join(sources, ", ")))
			skipped += len(items)
			continue
		}
		item := items[0]
		info, err := os.Lstat(item.exec.to)
		if os.IsNotExist(err) {
			pending = append(pending, item)
			continue
		}
		if err != nil {
			return fmt.Errorf("fpn: inspect destination %s: %w", displayPath(item.report.to), err)
		}
		name, err := item.parent.actualName(filepath.Base(item.exec.to), info)
		// An existing destination whose entry cannot be identified safely is
		// a collision, not grounds to abandon unrelated safe operations.
		if err == nil && name == item.source {
			item.exec.caseOnly = true
			pending = append(pending, item)
			continue
		}
		conflicts = append(conflicts, fmt.Sprintf("collision: %s would overwrite existing file (from %s)",
			displayPath(item.report.to), displayPath(item.report.from)))
		skipped++
	}

	finish := func(completed []renameOp, operationErr error) error {
		sort.Slice(completed, func(i, j int) bool { return completed[i].from < completed[j].from })
		var collisionErr error
		if len(conflicts) > 0 {
			verb := "renamed"
			if config.DryRun {
				verb = "would rename"
			}
			collisionErr = fmt.Errorf("%d collision(s); %s %d, skipped %d", len(conflicts), verb, len(completed), skipped)
		}
		return errors.Join(operationErr, collisionErr, logRenames(completed, config.writer()))
	}
	sort.Strings(conflicts)
	for _, conflict := range conflicts {
		if _, err := fmt.Fprintln(config.errWriter(), conflict); err != nil {
			return finish(nil, fmt.Errorf("fpn: write diagnostics: %w", err))
		}
	}
	if config.DryRun {
		var reports []renameOp
		for _, item := range pending {
			reports = append(reports, item.report)
		}
		return finish(reports, nil)
	}

	// Execute deepest physical paths first, independent of argument spelling.
	sort.Slice(pending, func(i, j int) bool {
		a, b := pending[i].exec.from, pending[j].exec.from
		da, db := strings.Count(a, string(filepath.Separator)), strings.Count(b, string(filepath.Separator))
		if da != db {
			return da > db
		}
		return a < b
	})
	var completed []renameOp
	for _, item := range pending {
		if err := renameOne(item.exec); err != nil {
			return finish(completed, err)
		}
		completed = append(completed, item.report)
	}
	return finish(completed, nil)
}

func normalizePathName(path string) string {
	dir, base := filepath.Split(path)
	return dir + normalizeBaseName(base)
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

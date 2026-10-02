package main

import (
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"
)

type Config struct {
	DryRun    bool
	Recursive bool
}

func main() {
	dryRun := flag.Bool("dry", false, "show what would happen without making changes")
	recursive := flag.Bool("recursive", false, "also rename everything nested inside directory arguments")
	flag.Parse()
	config := Config{
		DryRun:    *dryRun,
		Recursive: *recursive,
	}

	if err := run(flag.Args(), config); err != nil {
		os.Exit(1)
	}
}

func run(args []string, config Config) error {
	var argsErr error
	if len(args) < 1 {
		argsErr = fmt.Errorf("please specify one or more directories or files")
	}
	paths, walkErr := Walk(args, config)
	normErr := NormalizePaths(paths, config)

	err := errors.Join(argsErr, walkErr, normErr)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	return err
}

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
		fmt.Fprintf(os.Stderr, "fpn: %s: skipped\n", root)
		return nil, nil
	}

	info, err := os.Stat(root)
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

func NormalizePaths(paths []string, config Config) error {
	if len(paths) == 0 {
		return nil
	}

	type planItem struct{ original, normalized string }
	var plan []planItem
	for _, p := range paths {
		n := normalizePathName(p)
		if p != n {
			plan = append(plan, planItem{original: p, normalized: n})
		}
	}

	var pending []renameOp
	var conflicts []string
	skipped := 0

	// Group by normalized target: if several sources map to the same
	// target, report one collision for the group and rename none of them.
	byTarget := make(map[string][]string)
	for _, item := range plan {
		byTarget[item.normalized] = append(byTarget[item.normalized], item.original)
	}
	var filtered []planItem
	for _, item := range plan {
		if len(byTarget[item.normalized]) > 1 {
			continue
		}
		filtered = append(filtered, item)
	}
	for target, sources := range byTarget {
		if len(sources) < 2 {
			continue
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
		srcInfo, srcErr := os.Stat(original)
		if srcErr != nil {
			continue
		}
		dstInfo, dstErr := os.Stat(normalized)
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
			if os.SameFile(srcInfo, dstInfo) {
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
		fmt.Fprintln(os.Stderr, c)
	}

	sort.Slice(pending, func(i, j int) bool {
		return pending[i].from < pending[j].from
	})

	if config.DryRun {
		logRenames(pending)
		return nil
	}

	// Execute deepest paths first: renaming a directory invalidates the
	// still-original paths of everything nested inside it, so children
	// must be renamed before their ancestors.
	execOrder := append([]renameOp(nil), pending...)
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

	logRenames(pending)

	if len(conflicts) > 0 {
		return fmt.Errorf("%d collision(s); renamed %d, skipped %d",
			len(conflicts), len(pending), skipped)
	}
	return nil
}

type renameOp struct {
	from, to string
	caseOnly bool
}

func renameOne(r renameOp) error {
	if !r.caseOnly {
		return os.Rename(r.from, r.to)
	}
	dir := filepath.Dir(r.from)
	// The placeholder must match the source type: a directory cannot be
	// renamed onto a file (and vice versa).
	info, err := os.Lstat(r.from)
	if err != nil {
		return err
	}
	var tmpPath string
	if info.IsDir() {
		tmpPath, err = os.MkdirTemp(dir, ".fpn-*")
		if err != nil {
			return err
		}
		// Reserve a unique name; renaming onto an existing dir is not
		// portable.
		if err := os.Remove(tmpPath); err != nil {
			return err
		}
	} else {
		f, err := os.CreateTemp(dir, ".fpn-*")
		if err != nil {
			return err
		}
		tmpPath = f.Name()
		f.Close()
	}
	if err := os.Rename(r.from, tmpPath); err != nil {
		os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, r.to); err != nil {
		os.Rename(tmpPath, r.from)
		return err
	}
	return nil
}

func normalizePathName(path string) string {
	dir, base := filepath.Split(path)
	return dir + normalizeBaseName(base)
}

func normalizeBaseName(name string) string {
	name = strings.Join(strings.Fields(name), " ")
	replacer := strings.NewReplacer(" ", "-", "_", "-")
	return strings.ToLower(replacer.Replace(name))
}

func logRenames(renames []renameOp) {
	if len(renames) == 0 {
		return
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ORIGINAL\tNORMALIZED")
	for _, r := range renames {
		fmt.Fprintf(w, "%s\t%s\n", r.from, r.to)
	}
	w.Flush()
}

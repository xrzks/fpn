package normalize

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
)

// directory caches the actual entry names and lookup rules of one physical
// parent. Hard links remain distinct entries even when SameFile returns true.
type directory struct {
	path                       string
	entries                    map[string]fs.DirEntry
	ignoreCase, ignoreEncoding bool
	fullCaseFold               bool
}

type directoryCache struct {
	paths map[string]*directory
	ids   map[string]*directory
}

func (c *directoryCache) get(path string) (*directory, error) {
	if d := c.paths[path]; d != nil {
		return d, nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%q is not a directory", path)
	}
	id, err := directoryKey(path, info)
	if err != nil {
		return nil, err
	}
	if c.paths == nil {
		c.paths = make(map[string]*directory)
		c.ids = make(map[string]*directory)
	}
	if d := c.ids[id]; d != nil {
		c.paths[path] = d
		return d, nil
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	d := &directory{path: path, entries: make(map[string]fs.DirEntry)}
	for _, entry := range entries {
		name := entry.Name()
		d.entries[name] = entry
	}
	// Probe only existing entries: dry runs never create filesystem entries.
	// A negative Unicode probe does not establish a directory-wide rule (for
	// example, dotless i need not alias I on a case-insensitive filesystem).
	for _, entry := range entries {
		name := entry.Name()
		if !d.ignoreCase {
			alternate := strings.ToUpper(name)
			if alternate == name {
				alternate = strings.ToLower(name)
			}
			if alternate != name {
				d.ignoreCase, _, err = d.isAlias(name, alternate)
			}
		}
		// Some filesystems also expand characters while folding (ß -> ss).
		// Probe this separately so simple-fold filesystems keep those distinct.
		if err == nil && !d.fullCaseFold {
			alternate := cases.Fold().String(name)
			if utf8.RuneCountInString(alternate) > utf8.RuneCountInString(name) {
				d.fullCaseFold, _, err = d.isAlias(name, alternate)
				if d.fullCaseFold {
					d.ignoreCase = true
				}
			}
		}
		if err == nil && !d.ignoreEncoding {
			alternate := norm.NFD.String(name)
			if alternate == name {
				alternate = norm.NFC.String(name)
			}
			if alternate != name {
				d.ignoreEncoding, _, err = d.isAlias(name, alternate)
			}
		}
		if err != nil {
			return nil, err
		}
		if d.ignoreCase && d.fullCaseFold && d.ignoreEncoding {
			break
		}
	}
	c.paths[path], c.ids[id] = d, d
	return d, nil
}

func (d *directory) isAlias(name, alternate string) (alias, checked bool, err error) {
	if d.entries[alternate] != nil {
		return false, true, nil // A separately listed entry is never an alias.
	}
	other, err := os.Lstat(filepath.Join(d.path, alternate))
	if os.IsNotExist(err) {
		return false, true, nil
	}
	if errors.Is(err, syscall.ENAMETOOLONG) {
		return false, false, nil // An optional probe cannot invalidate another entry.
	}
	if err != nil {
		return false, false, err
	}
	info, err := d.entries[name].Info()
	return err == nil && os.SameFile(info, other), err == nil, err
}

func (d *directory) nameKey(name string) string {
	if d.ignoreEncoding {
		name = norm.NFD.String(name)
	}
	if d.fullCaseFold {
		name = cases.Fold().String(name)
	} else if d.ignoreCase {
		name = foldedName(name)
	}
	return name
}

// validateSource requires the exact stored basename, not a case, Unicode,
// short-name, or inode alias. Lookup rules are used only for target collisions.
func (d *directory) validateSource(name string) error {
	if d.entries[name] == nil {
		return fmt.Errorf("fpn: source basename %q does not exactly match a stored directory entry in %q", name, d.path)
	}
	return nil
}

func foldedName(name string) string {
	return strings.Map(func(r rune) rune {
		minimum := r
		for next := unicode.SimpleFold(r); next != r; next = unicode.SimpleFold(next) {
			if next < minimum {
				minimum = next
			}
		}
		return minimum
	}, name)
}

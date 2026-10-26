package normalize

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type renameOp struct {
	from, to string
	caseOnly bool
}

func renameOne(r renameOp) error {
	if !r.caseOnly {
		return renameNoReplace(r.from, r.to)
	}
	dir := filepath.Dir(r.from)
	// Keep the temporary entry inside a private directory. No placeholder
	// needs to be removed or overwritten, for either files or directories.
	tmpDir, err := os.MkdirTemp(dir, ".fpn-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmpDir) // Never remove a source left here after failed rollback.
	tmpPath := filepath.Join(tmpDir, "entry")
	if err := renameNoReplace(r.from, tmpPath); err != nil {
		return err
	}
	if err := renameNoReplace(tmpPath, r.to); err != nil {
		if rollbackErr := renameNoReplace(tmpPath, r.from); rollbackErr != nil {
			return errors.Join(err, fmt.Errorf("restore %q failed; source preserved at %q: %w", r.from, tmpPath, rollbackErr))
		}
		return err
	}
	return nil
}

// renameNoReplace atomically moves an entry only if its destination is absent.
// Unsupported kernels and filesystems must fail rather than fall back to Rename.
func renameNoReplace(from, to string) error {
	if err := renameNoReplaceOS(from, to); err != nil {
		return &os.LinkError{Op: "rename", Old: from, New: to, Err: err}
	}
	return nil
}

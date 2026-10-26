package normalize

import "os"

type renameOp struct {
	from, to string
}

func renameOne(r renameOp) error {
	return renameNoReplace(r.from, r.to)
}

// renameNoReplace atomically moves an entry only if its destination is absent.
// Unsupported kernels and filesystems must fail rather than fall back to Rename.
func renameNoReplace(from, to string) error {
	if err := renameNoReplaceOS(from, to); err != nil {
		return &os.LinkError{Op: "rename", Old: from, New: to, Err: err}
	}
	return nil
}

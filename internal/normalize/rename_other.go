//go:build !linux && !darwin && !windows

package normalize

import "errors"

func renameNoReplaceOS(from, to string) error {
	return errors.ErrUnsupported
}

//go:build !linux && !darwin && !windows

package main

import "errors"

func renameNoReplaceOS(from, to string) error {
	return errors.ErrUnsupported
}

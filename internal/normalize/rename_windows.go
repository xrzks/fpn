package normalize

import (
	"strings"

	"golang.org/x/sys/windows"
)

func renameNoReplaceOS(from, to string) error {
	from, err := longRenamePath(from)
	if err != nil {
		return err
	}
	to, err = longRenamePath(to)
	if err != nil {
		return err
	}
	source, err := windows.UTF16PtrFromString(from)
	if err != nil {
		return err
	}
	destination, err := windows.UTF16PtrFromString(to)
	if err != nil {
		return err
	}
	// Omitting MOVEFILE_REPLACE_EXISTING makes an existing destination an error.
	return windows.MoveFileEx(source, destination, 0)
}

// Match os file operations' support for paths beyond the legacy Windows limit,
// including relative and UNC paths, without requiring a system-wide opt-in.
func longRenamePath(path string) (string, error) {
	path = strings.ReplaceAll(path, "/", `\`)
	if strings.HasPrefix(path, `\\?\`) || strings.HasPrefix(path, `\??\`) || strings.HasPrefix(path, `\\.\`) {
		return path, nil
	}
	absolute, err := windows.FullPath(path)
	if err != nil {
		return "", err
	}
	if len(absolute) < 248 {
		return path, nil
	}
	if strings.HasPrefix(absolute, `\\`) {
		return `\\?\UNC\` + absolute[2:], nil
	}
	return `\\?\` + absolute, nil
}

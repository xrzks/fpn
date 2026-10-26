package normalize

import (
	"fmt"
	"os"
	"syscall"
)

func directoryKey(path string, info os.FileInfo) (string, error) {
	if info != nil {
		if stat, ok := info.Sys().(*syscall.Stat_t); ok && stat != nil {
			return fmt.Sprintf("%d:%d", stat.Dev, stat.Ino), nil
		}
	}
	return "", fmt.Errorf("fpn: directory identity for %q: unavailable stat information", path)
}

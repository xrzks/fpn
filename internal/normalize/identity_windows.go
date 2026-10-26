package normalize

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

func directoryKey(path string, _ os.FileInfo) (string, error) {
	name, err := longRenamePath(path)
	if err != nil {
		return "", err
	}
	p, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return "", err
	}
	handle, err := windows.CreateFile(p, windows.FILE_READ_ATTRIBUTES,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return "", err
	}
	defer windows.CloseHandle(handle)
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &info); err != nil {
		return "", err
	}
	index := uint64(info.FileIndexHigh)<<32 | uint64(info.FileIndexLow)
	return fmt.Sprintf("%d:%d", info.VolumeSerialNumber, index), nil
}

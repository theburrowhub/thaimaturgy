//go:build windows

package storage

import (
	"errors"
	"os"
	"time"

	"golang.org/x/sys/windows"
)

func lockSessionFile(file *os.File) (func() error, error) {
	handle := windows.Handle(file.Fd())
	var overlapped windows.Overlapped
	flags := uint32(windows.LOCKFILE_EXCLUSIVE_LOCK | windows.LOCKFILE_FAIL_IMMEDIATELY)
	for {
		err := windows.LockFileEx(handle, flags, 0, 1, 0, &overlapped)
		if err == nil {
			return func() error { return windows.UnlockFileEx(handle, 0, 1, 0, &overlapped) }, nil
		}
		if !errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
			return nil, err
		}
		time.Sleep(10 * time.Millisecond)
	}
}

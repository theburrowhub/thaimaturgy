//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package storage

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

func lockSessionFile(file *os.File) (func() error, error) {
	for {
		err := unix.Flock(int(file.Fd()), unix.LOCK_EX)
		if err == nil {
			return func() error { return unix.Flock(int(file.Fd()), unix.LOCK_UN) }, nil
		}
		if !errors.Is(err, unix.EINTR) {
			return nil, err
		}
	}
}

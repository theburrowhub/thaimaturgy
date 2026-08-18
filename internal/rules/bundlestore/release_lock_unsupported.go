//go:build !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris && !windows

package bundlestore

import (
	"context"
	"errors"
	"os"
)

func lockReleaseFile(context.Context, *os.File) (func() error, error) {
	return nil, errors.New("interprocess release locking is unsupported on this platform")
}

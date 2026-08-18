//go:build !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris && !windows

package bundlepack

import (
	"context"
	"errors"
	"os"
)

func lockOutputFile(context.Context, *os.File) (func() error, error) {
	return nil, errors.New("interprocess output locking is unsupported on this platform")
}

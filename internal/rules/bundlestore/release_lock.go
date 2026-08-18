package bundlestore

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const releaseLockFileName = ".install.lock"

type releaseLock struct {
	file   *os.File
	unlock func() error
}

func acquireReleaseLock(ctx context.Context, directory string) (*releaseLock, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(filepath.Join(directory, releaseLockFileName), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("rules bundle store: open release lock: %w", err)
	}
	unlock, err := lockReleaseFile(ctx, file)
	if err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("rules bundle store: acquire release lock: %w", err)
	}
	return &releaseLock{file: file, unlock: unlock}, nil
}

func (l *releaseLock) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	unlockErr := l.unlock()
	closeErr := l.file.Close()
	l.file = nil
	return errors.Join(unlockErr, closeErr)
}

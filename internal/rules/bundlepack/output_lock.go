package bundlepack

import (
	"context"
	"errors"
	"fmt"
	"os"
)

// outputLock uses a stable sidecar inode. Locking the output itself would be
// ineffective because publishing replaces that inode atomically.
type outputLock struct {
	file   *os.File
	unlock func() error
}

func acquireOutputLock(ctx context.Context, outputPath string) (*outputLock, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(outputPath+".lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("rules bundle pack: open output lock: %w", err)
	}
	unlock, err := lockOutputFile(ctx, file)
	if err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("rules bundle pack: acquire output lock: %w", err)
	}
	return &outputLock{file: file, unlock: unlock}, nil
}

func (l *outputLock) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	unlockErr := l.unlock()
	closeErr := l.file.Close()
	l.file = nil
	return errors.Join(unlockErr, closeErr)
}

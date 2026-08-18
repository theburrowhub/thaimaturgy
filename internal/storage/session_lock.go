package storage

import (
	"errors"
	"fmt"
	"os"
)

// sessionWriteLock uses a sidecar whose inode is never replaced. Locking the
// session JSON itself would be incorrect because atomicWriteFile publishes a new
// inode and a second process could then lock that while the first still holds
// the old one.
type sessionWriteLock struct {
	file   *os.File
	unlock func() error
}

func acquireSessionWriteLock(sessionPath string) (*sessionWriteLock, error) {
	file, err := os.OpenFile(sessionPath+".lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open sidecar: %w", err)
	}
	unlock, err := lockSessionFile(file)
	if err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("acquire sidecar: %w", err)
	}
	return &sessionWriteLock{file: file, unlock: unlock}, nil
}

func (l *sessionWriteLock) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	unlockErr := l.unlock()
	closeErr := l.file.Close()
	l.file = nil
	return errors.Join(unlockErr, closeErr)
}

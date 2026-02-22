package lock

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

var ErrLocked = errors.New("lock already held")

type FileLock struct {
	file *os.File
}

func New(path string) (*FileLock, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create lock dir: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open lock file: %w", err)
	}
	return &FileLock{file: f}, nil
}

func (l *FileLock) TryLock() error {
	if err := syscall.Flock(int(l.file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return ErrLocked
		}
		return fmt.Errorf("acquire lock: %w", err)
	}
	return nil
}

func (l *FileLock) Unlock() error {
	if l == nil || l.file == nil {
		return nil
	}
	if err := syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN); err != nil {
		return fmt.Errorf("release lock: %w", err)
	}
	return nil
}

func (l *FileLock) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	err := l.file.Close()
	l.file = nil
	if err != nil {
		return fmt.Errorf("close lock file: %w", err)
	}
	return nil
}

package lock

import (
	"errors"
	"fmt"
	"os"
	"strconv"
)

var ErrLocked = errors.New("lock already held")

type FileLock struct {
	path string
	file *os.File
}

func Acquire(path string) (*FileLock, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return nil, fmt.Errorf("%w: %s", ErrLocked, path)
		}
		return nil, fmt.Errorf("acquire lock %s: %w", path, err)
	}
	if _, err := file.WriteString(strconv.Itoa(os.Getpid()) + "\n"); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return nil, fmt.Errorf("write lock %s: %w", path, err)
	}
	return &FileLock{path: path, file: file}, nil
}

func (l *FileLock) Release() error {
	if l == nil {
		return nil
	}
	var closeErr error
	if l.file != nil {
		closeErr = l.file.Close()
	}
	removeErr := os.Remove(l.path)
	if closeErr != nil {
		return fmt.Errorf("close lock %s: %w", l.path, closeErr)
	}
	if removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
		return fmt.Errorf("remove lock %s: %w", l.path, removeErr)
	}
	return nil
}

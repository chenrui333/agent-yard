package lock

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

var ErrLocked = errors.New("lock already held")

type FileLock struct {
	path string
	file *os.File
}

func Acquire(path string) (*FileLock, error) {
	file, err := createLockFile(path)
	if err != nil {
		if !errors.Is(err, os.ErrExist) {
			return nil, fmt.Errorf("acquire lock %s: %w", path, err)
		}
		stale, detail, staleErr := staleLock(path)
		if staleErr != nil {
			return nil, fmt.Errorf("%w: %s (%v)", ErrLocked, path, staleErr)
		}
		if !stale {
			return nil, fmt.Errorf("%w: %s (%s)", ErrLocked, path, detail)
		}
		if removeErr := os.Remove(path); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			return nil, fmt.Errorf("remove stale lock %s: %w", path, removeErr)
		}
		file, err = createLockFile(path)
		if err != nil {
			if errors.Is(err, os.ErrExist) {
				return nil, fmt.Errorf("%w: %s", ErrLocked, path)
			}
			return nil, fmt.Errorf("acquire lock %s: %w", path, err)
		}
	}
	if _, err := file.WriteString(lockMetadata()); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return nil, fmt.Errorf("write lock %s: %w", path, err)
	}
	return &FileLock{path: path, file: file}, nil
}

func createLockFile(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
}

func lockMetadata() string {
	return fmt.Sprintf("pid=%d\ncreated_at=%s\n", os.Getpid(), time.Now().UTC().Format(time.RFC3339Nano))
}

func staleLock(path string) (bool, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, "", fmt.Errorf("read lock: %w", err)
	}
	pid, err := parseLockPID(string(data))
	if err != nil {
		return false, "", err
	}
	if processAlive(pid) {
		return false, fmt.Sprintf("pid %d is still running", pid), nil
	}
	return true, fmt.Sprintf("pid %d is stale", pid), nil
}

func parseLockPID(data string) (int, error) {
	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "pid=") {
			value := strings.TrimSpace(strings.TrimPrefix(line, "pid="))
			pid, err := strconv.Atoi(value)
			if err != nil || pid <= 0 {
				return 0, fmt.Errorf("malformed lock pid %q", value)
			}
			return pid, nil
		}
		pid, err := strconv.Atoi(line)
		if err != nil || pid <= 0 {
			return 0, fmt.Errorf("malformed lock pid %q", line)
		}
		return pid, nil
	}
	return 0, fmt.Errorf("malformed empty lock file")
}

func processAlive(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = process.Signal(syscall.Signal(0))
	if err == nil {
		return true
	}
	var errno syscall.Errno
	if errors.As(err, &errno) {
		if errno == syscall.ESRCH {
			return false
		}
		return errno == syscall.EPERM
	}
	return !errors.Is(err, os.ErrProcessDone)
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

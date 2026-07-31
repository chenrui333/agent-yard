package lock

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestAcquireRejectsActiveLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tasks.yaml.lock")
	if err := os.WriteFile(path, []byte("pid="+strconv.Itoa(os.Getpid())+"\n"), 0o644); err != nil {
		t.Fatalf("write lock: %v", err)
	}
	lock, err := Acquire(path)
	if err == nil {
		_ = lock.Release()
		t.Fatal("Acquire returned nil for active lock")
	}
	if !errors.Is(err, ErrLocked) {
		t.Fatalf("Acquire error = %v; want ErrLocked", err)
	}
	if !strings.Contains(err.Error(), "still running") {
		t.Fatalf("Acquire error should explain active PID: %v", err)
	}
}

func TestAcquireRecoversStaleLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tasks.yaml.lock")
	if err := os.WriteFile(path, []byte("pid=999999999\ncreated_at=2026-01-01T00:00:00Z\n"), 0o644); err != nil {
		t.Fatalf("write stale lock: %v", err)
	}
	lock, err := Acquire(path)
	if err != nil {
		t.Fatalf("Acquire returned error for stale lock: %v", err)
	}
	defer lock.Release()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read recovered lock: %v", err)
	}
	if !strings.Contains(string(data), "pid="+strconv.Itoa(os.Getpid())) {
		t.Fatalf("recovered lock metadata = %q; want current pid", data)
	}
}

func TestAcquireKeepsMalformedLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tasks.yaml.lock")
	if err := os.WriteFile(path, []byte("not a pid\n"), 0o644); err != nil {
		t.Fatalf("write malformed lock: %v", err)
	}
	lock, err := Acquire(path)
	if err == nil {
		_ = lock.Release()
		t.Fatal("Acquire returned nil for malformed lock")
	}
	if !errors.Is(err, ErrLocked) {
		t.Fatalf("Acquire error = %v; want ErrLocked", err)
	}
	data, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("read malformed lock: %v", readErr)
	}
	if string(data) != "not a pid\n" {
		t.Fatalf("malformed lock was changed: %q", data)
	}
}

func TestAcquireReleaseRemovesLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tasks.yaml.lock")
	lock, err := Acquire(path)
	if err != nil {
		t.Fatalf("Acquire returned error: %v", err)
	}
	if err := lock.Release(); err != nil {
		t.Fatalf("Release returned error: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("lock file should be removed, stat err = %v", err)
	}
}

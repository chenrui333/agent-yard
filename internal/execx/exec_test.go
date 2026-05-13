package execx

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

func TestRunnerAppliesDefaultTimeout(t *testing.T) {
	oldTimeout := DefaultTimeout
	DefaultTimeout = 20 * time.Millisecond
	t.Cleanup(func() { DefaultTimeout = oldTimeout })

	_, err := (Runner{}).Run(context.Background(), helperCommand(t, "1s"))
	if err == nil {
		t.Fatal("Run returned nil for timed-out helper")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Run error = %v; want context deadline exceeded", err)
	}
	if !strings.Contains(err.Error(), "run ") || !strings.Contains(err.Error(), "deadline") {
		t.Fatalf("timeout error lacks command/deadline context: %v", err)
	}
}

func TestRunnerRespectsCallerDeadline(t *testing.T) {
	oldTimeout := DefaultTimeout
	DefaultTimeout = time.Millisecond
	t.Cleanup(func() { DefaultTimeout = oldTimeout })

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := (Runner{}).Run(ctx, helperCommand(t, "30ms")); err != nil {
		t.Fatalf("Run returned error with caller deadline: %v", err)
	}
}

func TestRunnerCommandTimeoutOverridesDefault(t *testing.T) {
	oldTimeout := DefaultTimeout
	DefaultTimeout = time.Second
	t.Cleanup(func() { DefaultTimeout = oldTimeout })

	command := helperCommand(t, "1s")
	command.Timeout = 20 * time.Millisecond
	_, err := (Runner{}).Run(context.Background(), command)
	if err == nil {
		t.Fatal("Run returned nil for command timeout")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Run error = %v; want context deadline exceeded", err)
	}
}

func helperCommand(t *testing.T, sleep string) Command {
	t.Helper()
	return Command{
		Name: os.Args[0],
		Args: []string{"-test.run=TestExecxHelperProcess", "--"},
		Dir:  t.TempDir(),
		Env:  []string{"EXECX_HELPER_SLEEP=" + sleep},
	}
}

func TestExecxHelperProcess(t *testing.T) {
	sleep := os.Getenv("EXECX_HELPER_SLEEP")
	if sleep == "" {
		return
	}
	duration, err := time.ParseDuration(sleep)
	if err != nil {
		os.Exit(2)
	}
	time.Sleep(duration)
	os.Exit(0)
}

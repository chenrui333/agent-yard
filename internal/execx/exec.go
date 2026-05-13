package execx

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

type Command struct {
	Name    string
	Args    []string
	Dir     string
	Env     []string
	Timeout time.Duration
}

type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

type CommandError struct {
	Command Command
	Result  Result
	Err     error
}

func (e *CommandError) Error() string {
	parts := []string{fmt.Sprintf("run %s", e.Command.String())}
	if e.Command.Dir != "" {
		parts = append(parts, "in "+e.Command.Dir)
	}
	if e.Err != nil {
		parts = append(parts, e.Err.Error())
	}
	if strings.TrimSpace(e.Result.Stderr) != "" {
		parts = append(parts, "stderr: "+strings.TrimSpace(e.Result.Stderr))
	}
	if strings.TrimSpace(e.Result.Stdout) != "" {
		parts = append(parts, "stdout: "+strings.TrimSpace(e.Result.Stdout))
	}
	return strings.Join(parts, ": ")
}

func (e *CommandError) Unwrap() error {
	return e.Err
}

func (c Command) String() string {
	if len(c.Args) == 0 {
		return c.Name
	}
	return c.Name + " " + strings.Join(c.Args, " ")
}

type Runner struct{}

var DefaultTimeout = 2 * time.Minute

func LookPath(name string) (string, error) {
	path, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("find %s: %w", name, err)
	}
	return path, nil
}

func Exists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func (Runner) Run(ctx context.Context, command Command) (Result, error) {
	runCtx := ctx
	var cancel context.CancelFunc
	if _, ok := ctx.Deadline(); !ok {
		timeout := command.Timeout
		if timeout == 0 {
			timeout = DefaultTimeout
		}
		if timeout > 0 {
			runCtx, cancel = context.WithTimeout(ctx, timeout)
		}
	}
	if cancel != nil {
		defer cancel()
	}
	cmd := exec.CommandContext(runCtx, command.Name, command.Args...)
	cmd.Dir = command.Dir
	if len(command.Env) > 0 {
		cmd.Env = append(os.Environ(), command.Env...)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	result := Result{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: 0,
	}
	if err != nil {
		result.ExitCode = -1
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			result.ExitCode = exitErr.ExitCode()
		}
		if runCtx.Err() != nil {
			err = fmt.Errorf("%w (%s)", runCtx.Err(), deadlineDescription(runCtx))
		}
		return result, &CommandError{Command: command, Result: result, Err: err}
	}
	return result, nil
}

func deadlineDescription(ctx context.Context) string {
	deadline, ok := ctx.Deadline()
	if !ok {
		return "deadline reached"
	}
	return "deadline " + deadline.Format(time.RFC3339Nano)
}

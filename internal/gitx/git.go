package gitx

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/chenrui333/agent-yard/internal/execx"
)

type Runner interface {
	Run(context.Context, execx.Command) (execx.Result, error)
}

type Client struct {
	Runner Runner
}

type Worktree struct {
	Path     string
	HEAD     string
	Branch   string
	Detached bool
	Bare     bool
}

type AheadBehind struct {
	Behind int
	Ahead  int
}

func New() Client {
	return Client{Runner: execx.Runner{}}
}

func EnsureExists() error {
	_, err := execx.LookPath("git")
	return err
}

func (c Client) run(ctx context.Context, dir string, args ...string) (execx.Result, error) {
	return c.Runner.Run(ctx, execx.Command{Name: "git", Args: args, Dir: dir})
}

func (c Client) Fetch(ctx context.Context, repo, remote string) error {
	_, err := c.run(ctx, repo, "fetch", remote)
	return err
}

func (c Client) Push(ctx context.Context, dir, remote, branch string) error {
	if remote == "" {
		return fmt.Errorf("remote is required")
	}
	if branch == "" {
		return fmt.Errorf("branch is required")
	}
	_, err := c.run(ctx, dir, "push", "-u", remote, branch)
	return err
}

func (c Client) TopLevel(ctx context.Context, dir string) (string, error) {
	result, err := c.run(ctx, dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(result.Stdout), nil
}

func (c Client) CurrentBranch(ctx context.Context, dir string) (string, error) {
	result, err := c.run(ctx, dir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(result.Stdout), nil
}

func (c Client) BranchShowCurrent(ctx context.Context, dir string) (string, error) {
	result, err := c.run(ctx, dir, "branch", "--show-current")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(result.Stdout), nil
}

func (c Client) VerifyRef(ctx context.Context, dir, ref string) error {
	_, err := c.run(ctx, dir, "rev-parse", "--verify", ref)
	return err
}

func (c Client) IsAncestor(ctx context.Context, dir, ancestor, descendant string) (bool, error) {
	_, err := c.run(ctx, dir, "merge-base", "--is-ancestor", ancestor, descendant)
	if err == nil {
		return true, nil
	}
	var cmdErr *execx.CommandError
	if errors.As(err, &cmdErr) && cmdErr.Result.ExitCode == 1 {
		return false, nil
	}
	return false, err
}

func (c Client) StatusPorcelain(ctx context.Context, dir string) (string, error) {
	result, err := c.run(ctx, dir, "status", "--porcelain")
	if err != nil {
		return "", err
	}
	return result.Stdout, nil
}

func (c Client) IsDirty(ctx context.Context, dir string) (bool, error) {
	status, err := c.StatusPorcelain(ctx, dir)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(status) != "", nil
}

func (c Client) WorktreeList(ctx context.Context, repo string) ([]Worktree, error) {
	result, err := c.run(ctx, repo, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	return ParseWorktreeList(result.Stdout), nil
}

func (c Client) RemoveWorktree(ctx context.Context, repo, path string, force bool) error {
	if path == "" {
		return fmt.Errorf("worktree path is required")
	}
	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, path)
	_, err := c.run(ctx, repo, args...)
	return err
}

func (c Client) AddWorktree(ctx context.Context, repo, branch, path, remote, baseBranch string) error {
	if branch == "" {
		return fmt.Errorf("branch is required")
	}
	if path == "" {
		return fmt.Errorf("worktree path is required")
	}
	base := remote + "/" + baseBranch
	_, err := c.run(ctx, repo, "worktree", "add", "-b", branch, path, base)
	return err
}

func (c Client) AddDetachedWorktree(ctx context.Context, repo, path, remote, baseBranch string) error {
	if path == "" {
		return fmt.Errorf("worktree path is required")
	}
	base := remote + "/" + baseBranch
	_, err := c.run(ctx, repo, "worktree", "add", "--detach", path, base)
	return err
}

func (c Client) MergeBase(ctx context.Context, dir, ref string) (string, error) {
	result, err := c.run(ctx, dir, "merge-base", "HEAD", ref)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(result.Stdout), nil
}

func (c Client) DiffCheck(ctx context.Context, dir string) error {
	_, err := c.run(ctx, dir, "diff", "--check")
	return err
}

func (c Client) DiffCheckSince(ctx context.Context, dir, baseRef string) error {
	base, err := c.MergeBase(ctx, dir, baseRef)
	if err != nil {
		return err
	}
	_, err = c.run(ctx, dir, "diff", "--check", base+"..HEAD")
	return err
}

func (c Client) ResetHard(ctx context.Context, dir string) error {
	_, err := c.run(ctx, dir, "reset", "--hard")
	return err
}

func (c Client) Clean(ctx context.Context, dir string) error {
	_, err := c.run(ctx, dir, "clean", "-fd")
	return err
}

func (c Client) AheadBehind(ctx context.Context, dir, baseRef string) (AheadBehind, error) {
	result, err := c.run(ctx, dir, "rev-list", "--left-right", "--count", baseRef+"...HEAD")
	if err != nil {
		return AheadBehind{}, err
	}
	return ParseAheadBehind(result.Stdout)
}

func (c Client) ChangedFilesSince(ctx context.Context, dir, baseRef string) ([]string, error) {
	base, err := c.MergeBase(ctx, dir, baseRef)
	if err != nil {
		return nil, err
	}
	result, err := c.run(ctx, dir, "diff", "--name-only", base+"..HEAD")
	if err != nil {
		return nil, err
	}
	return splitNonEmptyLines(result.Stdout), nil
}

func (c Client) RemoteBranchExists(ctx context.Context, dir, remote, branch string) (bool, error) {
	if branch == "" {
		return false, nil
	}
	result, err := c.run(ctx, dir, "ls-remote", remote, "refs/heads/"+branch)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(result.Stdout) != "", nil
}

func (c Client) RemoteTrackingBranchExists(ctx context.Context, dir, remote, branch string) (bool, error) {
	if branch == "" {
		return false, nil
	}
	ref := "refs/remotes/" + remote + "/" + branch
	_, err := c.run(ctx, dir, "show-ref", "--verify", "--quiet", ref)
	if err == nil {
		return true, nil
	}
	var cmdErr *execx.CommandError
	if errors.As(err, &cmdErr) && cmdErr.Result.ExitCode == 1 {
		return false, nil
	}
	return false, err
}

func ParseWorktreeList(output string) []Worktree {
	var worktrees []Worktree
	var current *Worktree
	flush := func() {
		if current != nil && current.Path != "" {
			worktrees = append(worktrees, *current)
		}
		current = nil
	}
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			flush()
			continue
		}
		key, value, _ := strings.Cut(line, " ")
		switch key {
		case "worktree":
			flush()
			current = &Worktree{Path: value}
		case "HEAD":
			if current != nil {
				current.HEAD = value
			}
		case "branch":
			if current != nil {
				current.Branch = strings.TrimPrefix(value, "refs/heads/")
			}
		case "detached":
			if current != nil {
				current.Detached = true
			}
		case "bare":
			if current != nil {
				current.Bare = true
			}
		}
	}
	flush()
	return worktrees
}

func ParseAheadBehind(output string) (AheadBehind, error) {
	fields := strings.Fields(output)
	if len(fields) != 2 {
		return AheadBehind{}, fmt.Errorf("parse ahead/behind %q: expected two fields", strings.TrimSpace(output))
	}
	behind, err := strconv.Atoi(fields[0])
	if err != nil {
		return AheadBehind{}, fmt.Errorf("parse behind count %q: %w", fields[0], err)
	}
	ahead, err := strconv.Atoi(fields[1])
	if err != nil {
		return AheadBehind{}, fmt.Errorf("parse ahead count %q: %w", fields[1], err)
	}
	return AheadBehind{Behind: behind, Ahead: ahead}, nil
}

func splitNonEmptyLines(output string) []string {
	var lines []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

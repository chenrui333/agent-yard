package gitx

import (
	"context"
	"fmt"
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

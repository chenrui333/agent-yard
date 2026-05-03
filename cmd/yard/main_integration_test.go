package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

const (
	buildTimeout   = 2 * time.Minute
	commandTimeout = 30 * time.Second
)

func TestYardInitAndDryRunWorkflow(t *testing.T) {
	bin := buildYard(t)
	dir := t.TempDir()
	configPath := filepath.Join(dir, "yard.yaml")

	runYard(t, bin, dir, "--config", configPath, "init")
	for _, path := range []string{
		configPath,
		filepath.Join(dir, "tasks.yaml"),
		filepath.Join(dir, "prompts", "implement.md"),
		filepath.Join(dir, "prompts", "local-review.md"),
		filepath.Join(dir, "prompts", "pr-review.md"),
		filepath.Join(dir, ".yard", "runs"),
		filepath.Join(dir, ".yard", "reviews"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected %s to exist: %v", path, err)
		}
	}

	worktree := filepath.Join(dir, "worktree")
	if err := os.MkdirAll(worktree, 0o755); err != nil {
		t.Fatalf("create worktree dir: %v", err)
	}
	writeFile(t, filepath.Join(dir, "tasks.yaml"), fmt.Sprintf(`tasks:
  - id: aws-route53
    issue: 338
    checkbox: Route53 resources
    service_family: route53
    branch: aws-route53-resources
    worktree: %q
    status: ready
    assigned_agent: impl-01
    pr_url: ""
    pr_number: 0
`, worktree))

	statusOut := runYard(t, bin, dir, "--config", configPath, "status")
	assertContains(t, statusOut, "aws-route53")
	assertContains(t, statusOut, "ready")

	boardOut := runYard(t, bin, dir, "--config", configPath, "board")
	assertContains(t, boardOut, "aws-route53")
	assertContains(t, boardOut, "ready")

	launchOut := runYard(t, bin, dir, "--config", configPath, "launch", "aws-route53", "--dry-run", "--force")
	assertContains(t, launchOut, "window: impl-01")
	assertContains(t, launchOut, "codex")
	assertContains(t, launchOut, "implement.md")

	waveOut := runYard(t, bin, dir, "--config", configPath, "launch-wave", "--limit", "2", "--dry-run", "--force")
	assertContains(t, waveOut, "selected 1 task(s)")

	wavePlanOut := runYard(t, bin, dir, "--config", configPath, "wave", "plan", "--limit", "2")
	assertContains(t, wavePlanOut, "aws-route53")
	assertContains(t, wavePlanOut, "distinct service_family")

	wavePrepareOut := runYard(t, bin, dir, "--config", configPath, "wave", "prepare", "--limit", "2", "--dry-run")
	assertContains(t, wavePrepareOut, "aws-route53")

	prOut := runYard(t, bin, dir, "--config", configPath, "pr", "aws-route53", "--dry-run")
	assertContains(t, prOut, "Refs #338")
	assertContains(t, prOut, "head: aws-route53-resources")

	localReviewOut := runYard(t, bin, dir, "--config", configPath, "review-local", "aws-route53", "--dry-run", "--force")
	assertContains(t, localReviewOut, "window: local-review-aws-route53")
	assertContains(t, localReviewOut, "local-review.md")

	prReviewOut := runYard(t, bin, dir, "--config", configPath, "review-pr", "123", "--dry-run", "--force")
	assertContains(t, prReviewOut, "worktree:")
	assertContains(t, prReviewOut, "window: pr-review-123-pr-review-a")
	assertContains(t, prReviewOut, "pr-review.md")

	claimOut := runYard(t, bin, dir, "--config", configPath, "claim", "aws-route53", "--agent", "impl-02")
	assertContains(t, claimOut, "not posting GitHub comment without --comment")
	assertContains(t, claimOut, "Claiming task aws-route53 for impl-02.")
	tasksData := readFile(t, filepath.Join(dir, "tasks.yaml"))
	assertContains(t, tasksData, "status: claimed")
	assertContains(t, tasksData, "assigned_agent: impl-02")

	setStatusOut := runYard(t, bin, dir, "--config", configPath, "set-status", "aws-route53", "blocked", "--note", "needs manual split")
	assertContains(t, setStatusOut, "aws-route53 -> blocked")
	tasksData = readFile(t, filepath.Join(dir, "tasks.yaml"))
	assertContains(t, tasksData, "status: blocked")
	assertContains(t, tasksData, "note: needs manual split")

	clearNoteOut := runYard(t, bin, dir, "--config", configPath, "set-status", "aws-route53", "merge_ready", "--note", "")
	assertContains(t, clearNoteOut, "aws-route53 -> merge_ready")
	tasksData = readFile(t, filepath.Join(dir, "tasks.yaml"))
	assertContains(t, tasksData, "status: merge_ready")
	assertNotContains(t, tasksData, "note:")
}

func TestWavePrepareRevertsClaimOnFailure(t *testing.T) {
	bin := buildYard(t)
	dir := t.TempDir()
	configPath := filepath.Join(dir, "yard.yaml")

	runYard(t, bin, dir, "--config", configPath, "init")
	writeFile(t, filepath.Join(dir, "tasks.yaml"), `tasks:
  - id: broken
    issue: 338
    checkbox: Missing branch
    service_family: broken
    branch: ""
    worktree: ""
    status: ready
    pr_url: ""
    pr_number: 0
`)

	out, err := runYardErr(bin, dir, "--config", configPath, "wave", "prepare", "--limit", "1")
	if err == nil {
		t.Fatalf("expected wave prepare to fail\noutput:\n%s", out)
	}
	assertContains(t, out, "skip broken")
	assertContains(t, out, "prepared 0 task(s)")

	tasksData := readFile(t, filepath.Join(dir, "tasks.yaml"))
	assertContains(t, tasksData, "status: ready")
	assertNotContains(t, tasksData, "assigned_agent:")
}

func TestWaveLaunchSkipsUnlaunchableTasksBeforeLimit(t *testing.T) {
	bin := buildYard(t)
	dir := t.TempDir()
	configPath := filepath.Join(dir, "yard.yaml")
	launchableWorktree := filepath.Join(dir, "launchable-worktree")

	runYard(t, bin, dir, "--config", configPath, "init")
	if err := os.MkdirAll(launchableWorktree, 0o755); err != nil {
		t.Fatalf("create launchable worktree: %v", err)
	}
	writeFile(t, filepath.Join(dir, "tasks.yaml"), fmt.Sprintf(`tasks:
  - id: running
    issue: 338
    checkbox: Running task
    service_family: route53
    branch: running
    worktree: ""
    status: running
    assigned_agent: impl-01
    pr_url: ""
    pr_number: 0
  - id: claimed-missing
    issue: 338
    checkbox: Missing worktree
    service_family: ec2
    branch: missing-worktree
    worktree: %q
    status: claimed
    pr_url: ""
    pr_number: 0
  - id: ready-launch
    issue: 338
    checkbox: Ready launch
    service_family: s3
    branch: ready-launch
    worktree: %q
    status: worktree_created
    pr_url: ""
    pr_number: 0
`, filepath.Join(dir, "missing-worktree"), launchableWorktree))

	out := runYard(t, bin, dir, "--config", configPath, "wave", "launch", "--limit", "1", "--dry-run", "--force")
	assertContains(t, out, "window: impl-02")
	assertContains(t, out, "selected 1 task(s)")
	assertNotContains(t, out, "claimed-missing")
}

func TestWavePrepareFetchesBeforeLaterNewWorktree(t *testing.T) {
	bin := buildYard(t)
	dir := t.TempDir()
	srcRoot := filepath.Join(dir, "src")
	origin := filepath.Join(dir, "origin.git")
	repo := filepath.Join(srcRoot, "repo")

	runGit(t, dir, "init", "--bare", origin)
	runGit(t, dir, "init", repo)
	runGit(t, repo, "config", "user.name", "Yard Test")
	runGit(t, repo, "config", "user.email", "yard@example.com")
	writeFile(t, filepath.Join(repo, "README.md"), "hello\n")
	runGit(t, repo, "add", "README.md")
	runGit(t, repo, "commit", "-m", "initial")
	runGit(t, repo, "branch", "-M", "main")
	runGit(t, repo, "remote", "add", "origin", origin)
	runGit(t, repo, "push", "-u", "origin", "main")

	configPath := filepath.Join(dir, "yard.yaml")
	writeFile(t, configPath, fmt.Sprintf(`repo: %q
base_branch: main
default_remote: origin
session: yard-test
worktrees:
  root: %q
  prefix: repo.
agents:
  implementation:
    command: codex
    args:
      - exec
  local_review:
    command: codex
    args:
      - review
  pr_review:
    command: codex
    args:
      - review
`, repo, srcRoot))
	writeFile(t, filepath.Join(dir, "tasks.yaml"), `tasks:
  - id: existing
    issue: 338
    checkbox: Existing worktree
    service_family: route53
    branch: existing-worktree
    worktree: ""
    status: ready
    pr_url: ""
    pr_number: 0
  - id: later
    issue: 338
    checkbox: Later worktree
    service_family: s3
    branch: later-worktree
    worktree: ""
    status: ready
    pr_url: ""
    pr_number: 0
`)

	runYard(t, bin, dir, "--config", configPath, "worktree", "existing")
	tasksData := readFile(t, filepath.Join(dir, "tasks.yaml"))
	tasksData = strings.Replace(tasksData, "status: worktree_created", "status: ready", 1)
	writeFile(t, filepath.Join(dir, "tasks.yaml"), tasksData)

	runGit(t, repo, "remote", "set-url", "origin", filepath.Join(dir, "missing-origin.git"))
	out, err := runYardErr(bin, dir, "--config", configPath, "wave", "prepare", "--limit", "2")
	if err == nil {
		t.Fatalf("expected wave prepare to fail when later worktree fetch fails: %s", out)
	}
	assertContains(t, out, "worktree already exists:")
	assertContains(t, out, "skip later")
	assertContains(t, out, "prepared 1 task(s)")
	if _, err := os.Stat(filepath.Join(srcRoot, "repo.later-worktree")); !os.IsNotExist(err) {
		t.Fatalf("later worktree should not have been created, stat error: %v", err)
	}
}

func TestReviewPRDryRunWithRelativeConfigUsesAbsolutePaths(t *testing.T) {
	bin := buildYard(t)
	dir := t.TempDir()

	runYard(t, bin, dir, "init")
	out := runYard(t, bin, dir, "review-pr", "123", "--dry-run", "--force")

	reviewWorktree := filepath.Join(dir, ".yard", "reviews", "pr-123-pr-review-a")
	promptPath := filepath.Join(dir, ".yard", "runs", "pr-123-pr-review-a", "pr-review.md")
	assertContains(t, out, "worktree: "+reviewWorktree)
	assertContains(t, out, "cd '"+reviewWorktree+"'")
	assertContains(t, out, "< '"+promptPath+"'")
	assertNotContains(t, out, "< '.yard/")
}

func TestYardWorktreeCreatesGitWorktree(t *testing.T) {
	bin := buildYard(t)
	dir := t.TempDir()
	srcRoot := filepath.Join(dir, "src")
	origin := filepath.Join(dir, "origin.git")
	repo := filepath.Join(srcRoot, "repo")

	runGit(t, dir, "init", "--bare", origin)
	runGit(t, dir, "init", repo)
	runGit(t, repo, "config", "user.name", "Yard Test")
	runGit(t, repo, "config", "user.email", "yard@example.com")
	writeFile(t, filepath.Join(repo, "README.md"), "hello\n")
	runGit(t, repo, "add", "README.md")
	runGit(t, repo, "commit", "-m", "initial")
	runGit(t, repo, "branch", "-M", "main")
	runGit(t, repo, "remote", "add", "origin", origin)
	runGit(t, repo, "push", "-u", "origin", "main")

	configPath := filepath.Join(dir, "yard.yaml")
	writeFile(t, configPath, fmt.Sprintf(`repo: %q
base_branch: main
default_remote: origin
session: yard-test
worktrees:
  root: %q
  prefix: repo.
agents:
  implementation:
    command: codex
    args:
      - exec
      - --sandbox
      - danger-full-access
  local_review:
    command: codex
    args:
      - review
  pr_review:
    command: codex
    args:
      - review
`, repo, srcRoot))
	writeFile(t, filepath.Join(dir, "tasks.yaml"), `tasks:
  - id: route53
    issue: 338
    checkbox: Route53 resources
    service_family: route53
    branch: route53-resources
    worktree: ""
    status: ready
    pr_url: ""
    pr_number: 0
  - id: s3
    issue: 338
    checkbox: S3 resources
    service_family: s3
    branch: s3-resources
    worktree: ""
    status: ready
    pr_url: ""
    pr_number: 0
`)

	out := runYard(t, bin, dir, "--config", configPath, "worktree", "route53")
	worktree := filepath.Join(srcRoot, "repo.route53-resources")
	assertContains(t, out, "created worktree: "+worktree)
	if _, err := os.Stat(filepath.Join(worktree, ".git")); err != nil {
		t.Fatalf("expected git worktree at %s: %v", worktree, err)
	}

	tasksData := readFile(t, filepath.Join(dir, "tasks.yaml"))
	assertContains(t, tasksData, "status: worktree_created")
	assertContains(t, tasksData, worktree)

	runGit(t, repo, "remote", "set-url", "origin", filepath.Join(dir, "missing-origin.git"))
	rerunOut := runYard(t, bin, dir, "--config", configPath, "worktree", "route53")
	assertContains(t, rerunOut, "worktree already exists: "+worktree)
	runGit(t, repo, "remote", "set-url", "origin", origin)

	statusOut := runYard(t, bin, dir, "--config", configPath, "status")
	assertContains(t, statusOut, "worktree_created")
	assertContains(t, statusOut, "clean")

	wavePlanOut := runYard(t, bin, dir, "--config", configPath, "wave", "plan", "--limit", "1")
	assertContains(t, wavePlanOut, "s3")
	assertContains(t, wavePlanOut, "distinct service_family")

	wavePrepareOut := runYard(t, bin, dir, "--config", configPath, "wave", "prepare", "--limit", "1")
	s3Worktree := filepath.Join(srcRoot, "repo.s3-resources")
	assertContains(t, wavePrepareOut, "prepared 1 task(s)")
	if _, err := os.Stat(filepath.Join(s3Worktree, ".git")); err != nil {
		t.Fatalf("expected git worktree at %s: %v", s3Worktree, err)
	}
	tasksData = readFile(t, filepath.Join(dir, "tasks.yaml"))
	assertContains(t, tasksData, "assigned_agent: impl-01")
	assertContains(t, tasksData, s3Worktree)

	waveLaunchOut := runYard(t, bin, dir, "--config", configPath, "wave", "launch", "--limit", "1", "--dry-run", "--force")
	assertContains(t, waveLaunchOut, "selected 1 task(s)")
	assertContains(t, waveLaunchOut, "implement.md")
}

func buildYard(t *testing.T) string {
	t.Helper()
	root := repoRoot()
	bin := filepath.Join(t.TempDir(), "yard")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	ctx, cancel := context.WithTimeout(context.Background(), buildTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "build", "-o", bin, "./cmd/yard")
	cmd.Dir = root
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build yard: %v\n%s", err, output)
	}
	return bin
}

func repoRoot() string {
	return filepath.Clean(filepath.Join("..", ".."))
}

func runYard(t *testing.T, bin, dir string, args ...string) string {
	t.Helper()
	output, err := runYardErr(bin, dir, args...)
	if err != nil {
		t.Fatalf("yard %s: %v\n%s", strings.Join(args, " "), err, output)
	}
	return output
}

func runYardErr(bin, dir string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return string(output), fmt.Errorf("yard %s: %w", strings.Join(args, " "), ctx.Err())
	}
	return string(output), err
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), ctx.Err(), output)
	}
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
	return string(output)
}

func writeFile(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create dir for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func assertContains(t *testing.T, got, want string) {
	t.Helper()
	if !strings.Contains(got, want) {
		t.Fatalf("expected output to contain %q\noutput:\n%s", want, got)
	}
}

func assertNotContains(t *testing.T, got, want string) {
	t.Helper()
	if strings.Contains(got, want) {
		t.Fatalf("expected output not to contain %q\noutput:\n%s", want, got)
	}
}

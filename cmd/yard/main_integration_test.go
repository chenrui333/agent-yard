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
		filepath.Join(dir, "prompts", "commander.md"),
		filepath.Join(dir, "prompts", "implement.md"),
		filepath.Join(dir, "prompts", "local-review.md"),
		filepath.Join(dir, "prompts", "pr-review.md"),
		filepath.Join(dir, ".yard", "runs"),
		filepath.Join(dir, ".yard", "reviews"),
		filepath.Join(dir, ".yard", "review-results"),
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

	commanderOut := runYard(t, bin, dir, "--config", configPath, "commander", "--goal", "finish test wave", "--dry-run", "--force")
	assertContains(t, commanderOut, "window: commander")
	assertContains(t, commanderOut, "commander.md")

	launchOut := runYard(t, bin, dir, "--config", configPath, "launch", "aws-route53", "--dry-run", "--force")
	assertContains(t, launchOut, "window: impl-01")
	assertContains(t, launchOut, "codex")
	assertContains(t, launchOut, "implement.md")

	waveOut := runYard(t, bin, dir, "--config", configPath, "launch-wave", "--limit", "2", "--dry-run", "--force")
	assertContains(t, waveOut, "launch-wave is an alias for wave launch")
	assertContains(t, waveOut, "selected 0 task(s)")

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

func TestDoctorWarnsWhenGitHubCLIAbsentWithoutGitHubConfig(t *testing.T) {
	bin := buildYard(t)
	dir := t.TempDir()
	binDir := filepath.Join(dir, "bin")
	configPath := filepath.Join(dir, "yard.yaml")
	missingRoot := filepath.Join(dir, "missing", "worktrees")

	writeExecutable(t, filepath.Join(binDir, "git"), "#!/bin/sh\nexit 0\n")
	writeExecutable(t, filepath.Join(binDir, "tmux"), "#!/bin/sh\nif [ \"$1\" = \"has-session\" ]; then exit 1; fi\nexit 0\n")
	writeExecutable(t, filepath.Join(binDir, "codex"), "#!/bin/sh\nexit 0\n")
	writeFile(t, configPath, `repo: "."
base_branch: main
default_remote: origin
session: yard-test
worktrees:
  root: missing/worktrees
  prefix: yard.
agents:
  implementation:
    command: codex
  local_review:
    command: codex
  pr_review:
    command: codex
`)

	out, err := runYardErrEnv(bin, dir, []string{"PATH=" + binDir}, "--config", configPath, "doctor")
	if err != nil {
		t.Fatalf("doctor should warn but not fail without GitHub config: %v; output: %s", err, out)
	}
	assertContains(t, out, "gh")
	assertContains(t, out, "warn")
	assertContains(t, out, "GitHub CLI missing; required for GitHub commands")
	if _, err := os.Stat(missingRoot); !os.IsNotExist(err) {
		t.Fatalf("doctor should not create missing worktree root, stat error: %v", err)
	}
}

func TestDoctorScopesGitHubAuthToConfiguredHost(t *testing.T) {
	bin := buildYard(t)
	dir := t.TempDir()
	binDir := filepath.Join(dir, "bin")
	configPath := filepath.Join(dir, "yard.yaml")

	writeExecutable(t, filepath.Join(binDir, "git"), "#!/bin/sh\nexit 0\n")
	writeExecutable(t, filepath.Join(binDir, "tmux"), "#!/bin/sh\nif [ \"$1\" = \"has-session\" ]; then exit 1; fi\nexit 0\n")
	writeExecutable(t, filepath.Join(binDir, "codex"), "#!/bin/sh\nexit 0\n")
	writeExecutable(t, filepath.Join(binDir, "gh"), `#!/bin/sh
if [ "$1" = "auth" ] && [ "$2" = "status" ] && [ "$3" = "--hostname" ] && [ "$4" = "ghe.example.com" ]; then
  exit 0
fi
echo "unexpected gh args: $*" >&2
exit 1
`)
	writeFile(t, configPath, `repo: "."
base_branch: main
default_remote: origin
session: yard-test
github:
  host: https://ghe.example.com/
  owner: chenrui333
  repo: agent-yard
worktrees:
  root: .
  prefix: yard.
agents:
  implementation:
    command: codex
  local_review:
    command: codex
  pr_review:
    command: codex
`)

	out, err := runYardErrEnv(bin, dir, []string{"PATH=" + binDir}, "--config", configPath, "doctor")
	if err != nil {
		t.Fatalf("doctor should check configured GitHub host: %v; output: %s", err, out)
	}
	assertContains(t, out, "gh auth")
	assertContains(t, out, "authenticated GitHub CLI for ghe.example.com")
}

func TestBoardSkipsRemoteBranchProbe(t *testing.T) {
	bin := buildYard(t)
	dir := t.TempDir()
	binDir := filepath.Join(dir, "bin")
	configPath := filepath.Join(dir, "yard.yaml")
	worktree := filepath.Join(dir, "worktree")
	gitMarker := filepath.Join(dir, "git-called")
	ghMarker := filepath.Join(dir, "gh-called")

	if err := os.MkdirAll(worktree, 0o755); err != nil {
		t.Fatalf("create worktree: %v", err)
	}
	gitScript := fmt.Sprintf("#!/bin/sh\necho called > %q\nif [ \"$1\" = \"show-ref\" ]; then\n  exit 0\nfi\nif [ \"$1\" = \"rev-list\" ]; then\n  echo '0 0'\n  exit 0\nfi\nif [ \"$1\" = \"merge-base\" ]; then\n  echo abc\n  exit 0\nfi\nexit 0\n", gitMarker)
	writeExecutable(t, filepath.Join(binDir, "git"), gitScript)
	writeExecutable(t, filepath.Join(binDir, "gh"), fmt.Sprintf("#!/bin/sh\necho called > %q\necho '{\"number\":123,\"url\":\"https://github.com/o/r/pull/123\",\"state\":\"OPEN\",\"mergeStateStatus\":\"CLEAN\",\"reviewDecision\":\"APPROVED\"}'\n", ghMarker))
	writeExecutable(t, filepath.Join(binDir, "tmux"), "#!/bin/sh\nexit 0\n")
	writeFile(t, configPath, "repo: \".\"\nbase_branch: main\ndefault_remote: origin\nsession: yard-test\nagents:\n  implementation:\n    command: codex\n  local_review:\n    command: codex\n  pr_review:\n    command: codex\n")
	writeFile(t, filepath.Join(dir, "tasks.yaml"), fmt.Sprintf("tasks:\n  - id: remote-check\n    issue: 338\n    checkbox: Remote check\n    service_family: s3\n    branch: remote-check\n    worktree: %q\n    status: worktree_created\n    pr_url: \"https://github.com/o/r/pull/123\"\n    pr_number: 123\n", worktree))

	boardOut, err := runYardErrEnv(bin, dir, []string{"PATH=" + binDir}, "--config", configPath, "board")
	if err != nil {
		t.Fatalf("board should not require remote branch probe: %v\noutput:\n%s", err, boardOut)
	}
	assertContains(t, boardOut, "remote-check")
	if _, err := os.Stat(gitMarker); !os.IsNotExist(err) {
		t.Fatalf("board should not run git probes, stat error: %v", err)
	}
	if _, err := os.Stat(ghMarker); !os.IsNotExist(err) {
		t.Fatalf("board should not run GitHub PR probes, stat error: %v", err)
	}

	statusOut, err := runYardErrEnv(bin, dir, []string{"PATH=" + binDir}, "--config", configPath, "status")
	if err != nil {
		t.Fatalf("status should use local remote-tracking ref: %v\noutput:\n%s", err, statusOut)
	}
	assertContains(t, statusOut, "pushed")
	if _, err := os.Stat(gitMarker); err != nil {
		t.Fatalf("status should probe local remote-tracking ref, stat error: %v", err)
	}
	if _, err := os.Stat(ghMarker); err != nil {
		t.Fatalf("status should probe PR details, stat error: %v", err)
	}
}

func TestCaptureTailAndLanes(t *testing.T) {
	bin := buildYard(t)
	dir := t.TempDir()
	binDir := filepath.Join(dir, "bin")
	configPath := filepath.Join(dir, "yard.yaml")
	tmuxLog := filepath.Join(dir, "tmux.log")

	writeExecutable(t, filepath.Join(binDir, "tmux"), fmt.Sprintf(`#!/bin/sh
echo "$@" >> %q
if [ "$1" = "has-session" ]; then
  exit 0
fi
if [ "$1" = "list-windows" ]; then
  echo impl-01
  echo manual
  exit 0
fi
if [ "$1" = "list-panes" ]; then
  printf '%%%%1\tcodex\t0\t\n'
  exit 0
fi
if [ "$1" = "capture-pane" ]; then
  echo tail-output
  exit 0
fi
exit 0
`, tmuxLog))
	writeFile(t, configPath, `repo: "."
session: yard-test
agents:
  implementation:
    command: codex
  local_review:
    command: codex
  pr_review:
    command: codex
`)
	writeFile(t, filepath.Join(dir, "tasks.yaml"), `tasks:
  - id: feature
    issue: 338
    checkbox: Feature task
    service_family: feature
    branch: feature
    worktree: ""
    status: running
    assigned_agent: impl-01
    pr_url: ""
    pr_number: 0
`)

	captureOut, err := runYardErrEnv(bin, dir, []string{"PATH=" + binDir}, "--config", configPath, "capture", "feature", "--tail", "80")
	if err != nil {
		t.Fatalf("capture tail: %v\n%s", err, captureOut)
	}
	assertContains(t, captureOut, "tail-output")
	tmuxData := readFile(t, tmuxLog)
	assertContains(t, tmuxData, "capture-pane -p -S -80 -t yard-test:impl-01")

	lanesOut, err := runYardErrEnv(bin, dir, []string{"PATH=" + binDir}, "--config", configPath, "lanes")
	if err != nil {
		t.Fatalf("lanes: %v\n%s", err, lanesOut)
	}
	assertContains(t, lanesOut, "impl-01")
	assertContains(t, lanesOut, "feature")
	assertContains(t, lanesOut, "running codex")
	assertContains(t, lanesOut, "manual")
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

func TestWavePrepareDryRunCommentSkipsGitHubPreflight(t *testing.T) {
	bin := buildYard(t)
	dir := t.TempDir()
	binDir := filepath.Join(dir, "bin")
	configPath := filepath.Join(dir, "yard.yaml")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("create bin dir: %v", err)
	}

	writeFile(t, configPath, `repo: "."
base_branch: main
default_remote: origin
session: yard-test
agents:
  implementation:
    command: codex
  local_review:
    command: codex
  pr_review:
    command: codex
`)
	writeFile(t, filepath.Join(dir, "tasks.yaml"), `tasks:
  - id: ready
    issue: 338
    checkbox: Ready task
    service_family: s3
    branch: ready-task
    worktree: ""
    status: ready
    pr_url: ""
    pr_number: 0
`)

	out, err := runYardErrEnv(bin, dir, []string{"PATH=" + binDir}, "--config", configPath, "wave", "prepare", "--dry-run", "--comment", "--limit", "1")
	if err != nil {
		t.Fatalf("wave prepare dry-run should not require gh: %v; output: %s", err, out)
	}
	assertContains(t, out, "ready")
	assertContains(t, out, "distinct service_family")
}

func TestWavePrepareKeepsPreparingWhenCommentsFail(t *testing.T) {
	bin := buildYard(t)
	dir := t.TempDir()
	binDir := filepath.Join(dir, "bin")
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

	writeExecutable(t, filepath.Join(binDir, "gh"), `#!/bin/sh
echo comment failed >&2
exit 1
`)
	writeExecutable(t, filepath.Join(binDir, "tmux"), "#!/bin/sh\nexit 1\n")
	configPath := filepath.Join(dir, "yard.yaml")
	writeFile(t, configPath, fmt.Sprintf(`repo: %q
base_branch: main
default_remote: origin
session: yard-test
github:
  owner: chenrui333
  repo: agent-yard
  issue: 338
worktrees:
  root: %q
  prefix: repo.
agents:
  implementation:
    command: codex
  local_review:
    command: codex
  pr_review:
    command: codex
`, repo, srcRoot))
	writeFile(t, filepath.Join(dir, "tasks.yaml"), `tasks:
  - id: s3
    issue: 338
    checkbox: S3 resources
    service_family: s3
    branch: s3-resources
    worktree: ""
    status: ready
    pr_url: ""
    pr_number: 0
  - id: route53
    issue: 338
    checkbox: Route53 resources
    service_family: route53
    branch: route53-resources
    worktree: ""
    status: ready
    pr_url: ""
    pr_number: 0
`)

	out, err := runYardErrEnv(bin, dir, []string{"PATH=" + binDir + string(os.PathListSeparator) + os.Getenv("PATH")}, "--config", configPath, "wave", "prepare", "--limit", "2", "--comment")
	if err != nil {
		t.Fatalf("wave prepare comment failures should be non-fatal: %v\noutput:\n%s", err, out)
	}
	assertContains(t, out, "comment failed s3")
	assertContains(t, out, "comment failed route53")
	assertContains(t, out, "prepared 2 task(s)")
	assertContains(t, out, "comment failed for 2 prepared task(s): s3, route53")

	worktree := filepath.Join(srcRoot, "repo.s3-resources")
	if _, err := os.Stat(filepath.Join(worktree, ".git")); err != nil {
		t.Fatalf("expected git worktree at %s: %v", worktree, err)
	}
	secondWorktree := filepath.Join(srcRoot, "repo.route53-resources")
	if _, err := os.Stat(filepath.Join(secondWorktree, ".git")); err != nil {
		t.Fatalf("expected git worktree at %s: %v", secondWorktree, err)
	}
	tasksData := readFile(t, filepath.Join(dir, "tasks.yaml"))
	assertContains(t, tasksData, "status: worktree_created")
	assertContains(t, tasksData, "assigned_agent: impl-01")
	assertContains(t, tasksData, "assigned_agent: impl-02")
	assertContains(t, tasksData, worktree)
	assertContains(t, tasksData, secondWorktree)
}

func TestReviewPRChecksWindowBeforeWorktreeMutation(t *testing.T) {
	bin := buildYard(t)
	dir := t.TempDir()
	binDir := filepath.Join(dir, "bin")
	configPath := filepath.Join(dir, "yard.yaml")
	marker := filepath.Join(dir, "gh-called")

	writeExecutable(t, filepath.Join(binDir, "tmux"), `#!/bin/sh
if [ "$1" = "has-session" ]; then
  exit 0
fi
if [ "$1" = "list-windows" ]; then
  echo pr-review-123-pr-review-a
  exit 0
fi
exit 0
`)
	writeExecutable(t, filepath.Join(binDir, "git"), "#!/bin/sh\nexit 0\n")
	writeExecutable(t, filepath.Join(binDir, "codex"), "#!/bin/sh\nexit 0\n")
	writeExecutable(t, filepath.Join(binDir, "gh"), fmt.Sprintf("#!/bin/sh\necho called >> %q\nexit 1\n", marker))
	writeFile(t, configPath, `repo: "."
base_branch: main
default_remote: origin
session: yard-test
agents:
  implementation:
    command: codex
  local_review:
    command: codex
  pr_review:
    command: codex
`)

	out, err := runYardErrEnv(bin, dir, []string{"PATH=" + binDir}, "--config", configPath, "review-pr", "123", "--reset-worktree")
	if err == nil {
		t.Fatalf("expected existing review window error; output: %s", out)
	}
	assertContains(t, out, "tmux window pr-review-123-pr-review-a already exists")
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("gh should not be called before existing window rejection, stat error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".yard", "reviews")); !os.IsNotExist(err) {
		t.Fatalf("review worktree should not be created before existing window rejection, stat error: %v", err)
	}
}

func TestReviewPRRejectsPlainDirectoryInsideParentRepo(t *testing.T) {
	bin := buildYard(t)
	dir := t.TempDir()
	binDir := filepath.Join(dir, "bin")
	configPath := filepath.Join(dir, "yard.yaml")
	marker := filepath.Join(dir, "gh-called")
	reviewDir := filepath.Join(dir, ".yard", "reviews", "pr-123-pr-review-a")

	runGit(t, dir, "init", dir)
	if err := os.MkdirAll(reviewDir, 0o755); err != nil {
		t.Fatalf("create plain review dir: %v", err)
	}
	writeExecutable(t, filepath.Join(binDir, "tmux"), "#!/bin/sh\nif [ \"$1\" = \"has-session\" ]; then\n  exit 0\nfi\nif [ \"$1\" = \"list-windows\" ]; then\n  exit 0\nfi\nexit 0\n")
	writeExecutable(t, filepath.Join(binDir, "codex"), "#!/bin/sh\nexit 0\n")
	writeExecutable(t, filepath.Join(binDir, "gh"), fmt.Sprintf("#!/bin/sh\necho called >> %q\nexit 0\n", marker))
	writeFile(t, configPath, "repo: \".\"\nbase_branch: main\ndefault_remote: origin\nsession: yard-test\nagents:\n  implementation:\n    command: codex\n  local_review:\n    command: codex\n  pr_review:\n    command: codex\n")

	out, err := runYardErrEnv(bin, dir, []string{"PATH=" + binDir + string(os.PathListSeparator) + os.Getenv("PATH")}, "--config", configPath, "review-pr", "123", "--reset-worktree")
	if err == nil {
		t.Fatalf("expected plain review directory rejection; output: %s", out)
	}
	assertContains(t, out, "is not an isolated git worktree root")
	assertContains(t, out, reviewDir)
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("gh should not be called for non-worktree review dir, stat error: %v", err)
	}
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

func TestWaveLaunchChecksTmuxWindowAfterLaneReassignment(t *testing.T) {
	bin := buildYard(t)
	dir := t.TempDir()
	binDir := filepath.Join(dir, "bin")
	configPath := filepath.Join(dir, "yard.yaml")
	worktree := filepath.Join(dir, "worktree")
	tmuxLog := filepath.Join(dir, "tmux.log")

	if err := os.MkdirAll(worktree, 0o755); err != nil {
		t.Fatalf("create worktree: %v", err)
	}
	writeExecutable(t, filepath.Join(binDir, "git"), "#!/bin/sh\nexit 0\n")
	writeExecutable(t, filepath.Join(binDir, "codex"), "#!/bin/sh\nexit 0\n")
	writeExecutable(t, filepath.Join(binDir, "tmux"), fmt.Sprintf(`#!/bin/sh
echo "$@" >> %q
if [ "$1" = "has-session" ]; then
  exit 0
fi
if [ "$1" = "list-windows" ]; then
  echo impl-01
  exit 0
fi
exit 0
`, tmuxLog))
	writeFile(t, configPath, `repo: "."
base_branch: main
default_remote: origin
session: yard-test
agents:
  implementation:
    command: codex
  local_review:
    command: codex
  pr_review:
    command: codex
`)
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
  - id: launchable
    issue: 338
    checkbox: Launchable task
    service_family: s3
    branch: launchable
    worktree: %q
    status: worktree_created
    assigned_agent: impl-01
    pr_url: ""
    pr_number: 0
`, worktree))

	out, err := runYardErrEnv(bin, dir, []string{"PATH=" + binDir}, "--config", configPath, "wave", "launch", "--limit", "1")
	if err != nil {
		t.Fatalf("wave launch should reassign away from occupied stale lane: %v\noutput:\n%s", err, out)
	}
	assertContains(t, out, "selected 1 task(s)")

	tmuxData := readFile(t, tmuxLog)
	assertContains(t, tmuxData, "new-window -t yard-test -n impl-02")
	assertContains(t, tmuxData, "send-keys -t yard-test:impl-02")
	tasksData := readFile(t, filepath.Join(dir, "tasks.yaml"))
	assertContains(t, tasksData, "id: launchable")
	assertContains(t, tasksData, "assigned_agent: impl-02")
	assertContains(t, tasksData, "status: running")
}

func TestWaveLaunchReassignsAwayFromOccupiedManualLane(t *testing.T) {
	bin := buildYard(t)
	dir := t.TempDir()
	binDir := filepath.Join(dir, "bin")
	configPath := filepath.Join(dir, "yard.yaml")
	firstWorktree := filepath.Join(dir, "first-worktree")
	secondWorktree := filepath.Join(dir, "second-worktree")
	tmuxLog := filepath.Join(dir, "tmux.log")

	if err := os.MkdirAll(firstWorktree, 0o755); err != nil {
		t.Fatalf("create first worktree: %v", err)
	}
	if err := os.MkdirAll(secondWorktree, 0o755); err != nil {
		t.Fatalf("create second worktree: %v", err)
	}
	writeExecutable(t, filepath.Join(binDir, "git"), "#!/bin/sh\nexit 0\n")
	writeExecutable(t, filepath.Join(binDir, "codex"), "#!/bin/sh\nexit 0\n")
	writeExecutable(t, filepath.Join(binDir, "tmux"), fmt.Sprintf(`#!/bin/sh
echo "$@" >> %q
if [ "$1" = "has-session" ]; then
  exit 0
fi
if [ "$1" = "list-windows" ]; then
  echo impl-01
  exit 0
fi
exit 0
`, tmuxLog))
	writeFile(t, configPath, `repo: "."
base_branch: main
default_remote: origin
session: yard-test
agents:
  implementation:
    command: codex
  local_review:
    command: codex
  pr_review:
    command: codex
`)
	writeFile(t, filepath.Join(dir, "tasks.yaml"), fmt.Sprintf(`tasks:
  - id: first
    issue: 338
    checkbox: First task
    service_family: route53
    branch: first
    worktree: %q
    status: worktree_created
    pr_url: ""
    pr_number: 0
  - id: second
    issue: 338
    checkbox: Second task
    service_family: s3
    branch: second
    worktree: %q
    status: worktree_created
    pr_url: ""
    pr_number: 0
`, firstWorktree, secondWorktree))

	out, err := runYardErrEnv(bin, dir, []string{"PATH=" + binDir}, "--config", configPath, "wave", "launch", "--limit", "1")
	if err != nil {
		t.Fatalf("wave launch should continue after occupied manual lane: %v\noutput:\n%s", err, out)
	}
	assertContains(t, out, "selected 1 task(s)")

	tmuxData := readFile(t, tmuxLog)
	assertContains(t, tmuxData, "new-window -t yard-test -n impl-02")
	assertContains(t, tmuxData, "send-keys -t yard-test:impl-02")
	tasksData := readFile(t, filepath.Join(dir, "tasks.yaml"))
	assertContains(t, tasksData, "id: first")
	assertContains(t, tasksData, "assigned_agent: impl-02")
	assertContains(t, tasksData, "status: running")
	assertContains(t, tasksData, "id: second")
}

func TestWaveLaunchForceReusesOccupiedAssignedLane(t *testing.T) {
	bin := buildYard(t)
	dir := t.TempDir()
	binDir := filepath.Join(dir, "bin")
	configPath := filepath.Join(dir, "yard.yaml")
	worktree := filepath.Join(dir, "worktree")
	tmuxLog := filepath.Join(dir, "tmux.log")

	if err := os.MkdirAll(worktree, 0o755); err != nil {
		t.Fatalf("create worktree: %v", err)
	}
	writeExecutable(t, filepath.Join(binDir, "git"), "#!/bin/sh\nexit 0\n")
	writeExecutable(t, filepath.Join(binDir, "codex"), "#!/bin/sh\nexit 0\n")
	writeExecutable(t, filepath.Join(binDir, "tmux"), fmt.Sprintf(`#!/bin/sh
echo "$@" >> %q
if [ "$1" = "has-session" ]; then
  exit 0
fi
if [ "$1" = "list-windows" ]; then
  echo impl-01
  exit 0
fi
exit 0
`, tmuxLog))
	writeFile(t, configPath, `repo: "."
base_branch: main
default_remote: origin
session: yard-test
agents:
  implementation:
    command: codex
  local_review:
    command: codex
  pr_review:
    command: codex
`)
	writeFile(t, filepath.Join(dir, "tasks.yaml"), fmt.Sprintf(`tasks:
  - id: launchable
    issue: 338
    checkbox: Launchable task
    service_family: s3
    branch: launchable
    worktree: %q
    status: worktree_created
    assigned_agent: impl-01
    pr_url: ""
    pr_number: 0
`, worktree))

	out, err := runYardErrEnv(bin, dir, []string{"PATH=" + binDir + string(os.PathListSeparator) + os.Getenv("PATH")}, "--config", configPath, "wave", "launch", "--limit", "1", "--force")
	if err != nil {
		t.Fatalf("wave launch force: %v\n%s", err, out)
	}
	assertContains(t, out, "selected 1 task(s)")
	tmuxData := readFile(t, tmuxLog)
	assertContains(t, tmuxData, "send-keys -t yard-test:impl-01")
	assertNotContains(t, tmuxData, "new-window -t yard-test -n impl-02")
}

func TestWaveLaunchFailsWhenNoSelectedTasksStart(t *testing.T) {
	bin := buildYard(t)
	dir := t.TempDir()
	binDir := filepath.Join(dir, "bin")
	configPath := filepath.Join(dir, "yard.yaml")
	worktree := filepath.Join(dir, "worktree")

	if err := os.MkdirAll(worktree, 0o755); err != nil {
		t.Fatalf("create worktree: %v", err)
	}
	writeExecutable(t, filepath.Join(binDir, "git"), "#!/bin/sh\nexit 0\n")
	writeExecutable(t, filepath.Join(binDir, "tmux"), "#!/bin/sh\nif [ \"$1\" = \"has-session\" ]; then\n  exit 0\nfi\nif [ \"$1\" = \"list-windows\" ]; then\n  echo impl-01\n  exit 0\nfi\nexit 0\n")
	writeFile(t, configPath, "repo: \".\"\nbase_branch: main\ndefault_remote: origin\nsession: yard-test\nagents:\n  implementation:\n    command: missing-agent\n  local_review:\n    command: missing-agent\n  pr_review:\n    command: missing-agent\n")
	writeFile(t, filepath.Join(dir, "tasks.yaml"), fmt.Sprintf("tasks:\n  - id: one\n    issue: 338\n    checkbox: One task\n    service_family: s3\n    branch: one\n    worktree: %q\n    status: worktree_created\n    pr_url: \"\"\n    pr_number: 0\n", worktree))

	out, err := runYardErrEnv(bin, dir, []string{"PATH=" + binDir}, "--config", configPath, "wave", "launch", "--limit", "1")
	if err == nil {
		t.Fatalf("expected wave launch failure when no selected tasks start\noutput:\n%s", out)
	}
	assertContains(t, out, "skip one:")
	assertContains(t, out, "selected 0 task(s)")
	assertContains(t, out, "failed to launch 1 selected task(s): one")
}

func TestWaveLaunchSurfacesWorktreeDirtyProbeFailure(t *testing.T) {
	bin := buildYard(t)
	dir := t.TempDir()
	binDir := filepath.Join(dir, "bin")
	configPath := filepath.Join(dir, "yard.yaml")
	worktree := filepath.Join(dir, "worktree")

	if err := os.MkdirAll(worktree, 0o755); err != nil {
		t.Fatalf("create worktree: %v", err)
	}
	writeExecutable(t, filepath.Join(binDir, "git"), "#!/bin/sh\necho git failed >&2\nexit 2\n")
	writeFile(t, configPath, `repo: "."
base_branch: main
default_remote: origin
session: yard-test
agents:
  implementation:
    command: codex
  local_review:
    command: codex
  pr_review:
    command: codex
`)
	writeFile(t, filepath.Join(dir, "tasks.yaml"), fmt.Sprintf(`tasks:
  - id: broken
    issue: 338
    checkbox: Broken worktree
    service_family: s3
    branch: broken
    worktree: %q
    status: worktree_created
    pr_url: ""
    pr_number: 0
`, worktree))

	out, err := runYardErrEnv(bin, dir, []string{"PATH=" + binDir}, "--config", configPath, "wave", "launch", "--limit", "1")
	if err == nil {
		t.Fatalf("expected dirty probe failure\noutput:\n%s", out)
	}
	assertContains(t, out, "check worktree dirty state")
	assertContains(t, out, worktree)
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
	assertContains(t, tasksData, "assigned_agent: impl-")
	assertContains(t, tasksData, s3Worktree)

	waveLaunchOut := runYard(t, bin, dir, "--config", configPath, "wave", "launch", "--limit", "1", "--dry-run", "--force")
	assertContains(t, waveLaunchOut, "selected 1 task(s)")
	assertContains(t, waveLaunchOut, "implement.md")
}

func TestSyncIssueWriteImportsCheckboxes(t *testing.T) {
	bin := buildYard(t)
	dir := t.TempDir()
	binDir := filepath.Join(dir, "bin")
	configPath := filepath.Join(dir, "yard.yaml")

	writeExecutable(t, filepath.Join(binDir, "gh"), `#!/bin/sh
cat <<\EOF
{"title":"AWS coverage","url":"https://github.com/o/r/issues/338","body":"## Route 53\n- [ ] Hosted zone support\n- [x] Done route item\n## S3\n- [ ] Bucket support\n"}
EOF
`)
	writeFile(t, configPath, `repo: "."
github:
  owner: o
  repo: r
agents:
  implementation:
    command: codex
  local_review:
    command: codex
  pr_review:
    command: codex
`)
	writeFile(t, filepath.Join(dir, "tasks.yaml"), `tasks:
  - id: existing
    issue: 338
    checkbox: Hosted zone support
    service_family: route-53
    branch: existing
    worktree: ""
    status: running
    pr_url: "https://github.com/o/r/pull/10"
    pr_number: 10
`)

	out, err := runYardErrEnv(bin, dir, []string{"PATH=" + binDir + string(os.PathListSeparator) + os.Getenv("PATH")}, "--config", configPath, "sync", "issue", "338", "--write", "--section", "s3", "--id-prefix", "aws-", "--branch-prefix", "tf-")
	if err != nil {
		t.Fatalf("sync issue: %v\n%s", err, out)
	}
	assertContains(t, out, "added 1 task(s)")
	assertContains(t, out, "aws-bucket-support")
	tasksData := readFile(t, filepath.Join(dir, "tasks.yaml"))
	assertContains(t, tasksData, "id: existing")
	assertContains(t, tasksData, "status: running")
	assertContains(t, tasksData, "pr_number: 10")
	assertContains(t, tasksData, "id: aws-bucket-support")
	assertContains(t, tasksData, "branch: tf-bucket-support")
	assertContains(t, tasksData, "service_family: s3")
	assertNotContains(t, tasksData, "Done route item")
}

func TestPRPushesBranchAndRecordsExistingPR(t *testing.T) {
	bin := buildYard(t)
	dir := t.TempDir()
	binDir := filepath.Join(dir, "bin")
	origin := filepath.Join(dir, "origin.git")
	repo := filepath.Join(dir, "repo")
	configPath := filepath.Join(dir, "yard.yaml")

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
	runGit(t, repo, "checkout", "-b", "feature-task")
	writeFile(t, filepath.Join(repo, "feature.txt"), "feature\n")
	runGit(t, repo, "add", "feature.txt")
	runGit(t, repo, "commit", "-m", "feature")

	writeExecutable(t, filepath.Join(binDir, "gh"), `#!/bin/sh
if [ "$1" = "pr" ] && [ "$2" = "list" ]; then
  echo "[{\"number\":123,\"url\":\"https://github.com/o/r/pull/123\",\"state\":\"OPEN\",\"headRefName\":\"feature-task\",\"baseRefName\":\"main\",\"headRepositoryOwner\":{\"login\":\"o\"},\"headRepository\":{\"name\":\"r\"},\"isCrossRepository\":false}]"
  exit 0
fi
echo "unexpected gh args: $*" >&2
exit 1
`)
	writeFile(t, configPath, fmt.Sprintf(`repo: %q
base_branch: main
default_remote: origin
github:
  owner: o
  repo: r
agents:
  implementation:
    command: codex
  local_review:
    command: codex
  pr_review:
    command: codex
`, repo))
	writeFile(t, filepath.Join(dir, "tasks.yaml"), fmt.Sprintf(`tasks:
  - id: feature
    issue: 338
    checkbox: Feature task
    service_family: feature
    branch: feature-task
    worktree: %q
    status: needs_review
    pr_url: ""
    pr_number: 0
`, repo))

	out, err := runYardErrEnv(bin, dir, []string{"PATH=" + binDir + string(os.PathListSeparator) + os.Getenv("PATH")}, "--config", configPath, "pr", "feature")
	if err != nil {
		t.Fatalf("pr feature: %v\n%s", err, out)
	}
	assertContains(t, out, "existing PR: https://github.com/o/r/pull/123")
	runGit(t, repo, "ls-remote", "--exit-code", "origin", "refs/heads/feature-task")
	tasksData := readFile(t, filepath.Join(dir, "tasks.yaml"))
	assertContains(t, tasksData, "status: pr_opened")
	assertContains(t, tasksData, "pr_number: 123")
}

func TestPRRejectsExistingPRWithWrongBase(t *testing.T) {
	bin := buildYard(t)
	dir := t.TempDir()
	binDir := filepath.Join(dir, "bin")
	origin := filepath.Join(dir, "origin.git")
	repo := filepath.Join(dir, "repo")
	configPath := filepath.Join(dir, "yard.yaml")

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
	runGit(t, repo, "checkout", "-b", "feature-task")
	writeFile(t, filepath.Join(repo, "feature.txt"), "feature\n")
	runGit(t, repo, "add", "feature.txt")
	runGit(t, repo, "commit", "-m", "feature")

	writeExecutable(t, filepath.Join(binDir, "gh"), `#!/bin/sh
if [ "$1" = "pr" ] && [ "$2" = "list" ]; then
  echo "[{\"number\":123,\"url\":\"https://github.com/o/r/pull/123\",\"state\":\"OPEN\",\"headRefName\":\"feature-task\",\"baseRefName\":\"release\",\"headRepositoryOwner\":{\"login\":\"o\"},\"headRepository\":{\"name\":\"r\"},\"isCrossRepository\":false}]"
  exit 0
fi
echo "unexpected gh args: $*" >&2
exit 1
`)
	writeFile(t, configPath, fmt.Sprintf(`repo: %q
base_branch: main
default_remote: origin
github:
  owner: o
  repo: r
agents:
  implementation:
    command: codex
  local_review:
    command: codex
  pr_review:
    command: codex
`, repo))
	writeFile(t, filepath.Join(dir, "tasks.yaml"), fmt.Sprintf(`tasks:
  - id: feature
    issue: 338
    checkbox: Feature task
    service_family: feature
    branch: feature-task
    worktree: %q
    status: needs_review
    pr_url: ""
    pr_number: 0
`, repo))

	out, err := runYardErrEnv(bin, dir, []string{"PATH=" + binDir + string(os.PathListSeparator) + os.Getenv("PATH")}, "--config", configPath, "pr", "feature", "--no-push")
	if err == nil {
		t.Fatalf("expected pr feature to reject wrong-base PR\noutput:\n%s", out)
	}
	assertContains(t, out, "did not match base \"main\" and repository \"o/r\"")
	tasksData := readFile(t, filepath.Join(dir, "tasks.yaml"))
	assertContains(t, tasksData, "status: needs_review")
	assertContains(t, tasksData, "pr_number: 0")
}

func TestReadyWritesMergeReadyWhenChecksPass(t *testing.T) {
	bin := buildYard(t)
	dir := t.TempDir()
	binDir := filepath.Join(dir, "bin")
	origin := filepath.Join(dir, "origin.git")
	repo := filepath.Join(dir, "repo")
	configPath := filepath.Join(dir, "yard.yaml")

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
	runGit(t, repo, "checkout", "-b", "feature-task")
	writeFile(t, filepath.Join(repo, "feature.txt"), "feature\n")
	runGit(t, repo, "add", "feature.txt")
	runGit(t, repo, "commit", "-m", "feature")
	runGit(t, repo, "push", "-u", "origin", "feature-task")

	writeExecutable(t, filepath.Join(binDir, "gh"), `#!/bin/sh
if [ "$1" = "pr" ] && [ "$2" = "view" ]; then
  echo "{\"number\":123,\"url\":\"https://github.com/o/r/pull/123\",\"state\":\"OPEN\",\"headRefName\":\"feature-task\",\"baseRefName\":\"main\",\"mergeStateStatus\":\"CLEAN\",\"reviewDecision\":\"APPROVED\",\"statusCheckRollup\":[{\"name\":\"test\",\"status\":\"COMPLETED\",\"conclusion\":\"SUCCESS\"}]}"
  exit 0
fi
echo "unexpected gh args: $*" >&2
exit 1
`)
	writeExecutable(t, filepath.Join(binDir, "tmux"), `#!/bin/sh
if [ "$1" = "capture-pane" ]; then
  echo "There are no P1/P2/P3 TODO comments."
  exit 0
fi
exit 0
`)
	writeFile(t, configPath, fmt.Sprintf(`repo: %q
base_branch: main
default_remote: origin
session: yard-test
github:
  owner: o
  repo: r
agents:
  implementation:
    command: codex
  local_review:
    command: codex
  pr_review:
    command: codex
`, repo))
	writeFile(t, filepath.Join(dir, "tasks.yaml"), fmt.Sprintf(`tasks:
  - id: feature
    issue: 338
    checkbox: Feature task
    service_family: feature
    branch: feature-task
    worktree: %q
    status: pr_opened
    pr_url: "https://github.com/o/r/pull/123"
    pr_number: 123
`, repo))

	out, err := runYardErrEnv(bin, dir, []string{"PATH=" + binDir + string(os.PathListSeparator) + os.Getenv("PATH")}, "--config", configPath, "ready", "feature", "--review-lane", "pr-review-a", "--write")
	if err != nil {
		t.Fatalf("ready feature: %v\n%s", err, out)
	}
	assertContains(t, out, "merge state")
	assertContains(t, out, "checks")
	assertContains(t, out, "review lane")
	tasksData := readFile(t, filepath.Join(dir, "tasks.yaml"))
	assertContains(t, tasksData, "status: merge_ready")
}

func TestReadyUsesStructuredReviewResultBeforeTmuxCapture(t *testing.T) {
	bin := buildYard(t)
	dir := t.TempDir()
	binDir := filepath.Join(dir, "bin")
	origin := filepath.Join(dir, "origin.git")
	repo := filepath.Join(dir, "repo")
	configPath := filepath.Join(dir, "yard.yaml")
	tmuxMarker := filepath.Join(dir, "tmux-called")

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
	runGit(t, repo, "checkout", "-b", "feature-task")
	writeFile(t, filepath.Join(repo, "feature.txt"), "feature\n")
	runGit(t, repo, "add", "feature.txt")
	runGit(t, repo, "commit", "-m", "feature")
	runGit(t, repo, "push", "-u", "origin", "feature-task")

	writeExecutable(t, filepath.Join(binDir, "gh"), `#!/bin/sh
if [ "$1" = "pr" ] && [ "$2" = "view" ]; then
  echo "{\"number\":123,\"url\":\"https://github.com/o/r/pull/123\",\"state\":\"OPEN\",\"headRefName\":\"feature-task\",\"baseRefName\":\"main\",\"mergeStateStatus\":\"CLEAN\",\"reviewDecision\":\"APPROVED\",\"statusCheckRollup\":[{\"name\":\"test\",\"status\":\"COMPLETED\",\"conclusion\":\"SUCCESS\"}]}"
  exit 0
fi
echo "unexpected gh args: $*" >&2
exit 1
`)
	writeExecutable(t, filepath.Join(binDir, "tmux"), fmt.Sprintf("#!/bin/sh\necho called > %q\nexit 1\n", tmuxMarker))
	writeFile(t, configPath, fmt.Sprintf(`repo: %q
base_branch: main
default_remote: origin
session: yard-test
github:
  owner: o
  repo: r
agents:
  implementation:
    command: codex
  local_review:
    command: codex
  pr_review:
    command: codex
`, repo))
	writeFile(t, filepath.Join(dir, "tasks.yaml"), fmt.Sprintf(`tasks:
  - id: feature
    issue: 338
    checkbox: Feature task
    service_family: feature
    branch: feature-task
    worktree: %q
    status: pr_opened
    pr_url: "https://github.com/o/r/pull/123"
    pr_number: 123
`, repo))

	rejectOut, err := runYardErrEnv(bin, dir, []string{"PATH=" + binDir + string(os.PathListSeparator) + os.Getenv("PATH")}, "--config", configPath, "review-result", "feature", "--lane", "pr-review-a", "--priority", "P2")
	if err == nil {
		t.Fatalf("review-result with clear status and priority should fail\n%s", rejectOut)
	}
	assertContains(t, rejectOut, "cannot include blocking priorities P2")

	recordOut, err := runYardErrEnv(bin, dir, []string{"PATH=" + binDir + string(os.PathListSeparator) + os.Getenv("PATH")}, "--config", configPath, "review-result", "feature", "--lane", "pr-review-a", "--summary", "clear")
	if err != nil {
		t.Fatalf("review-result feature: %v\n%s", err, recordOut)
	}
	assertContains(t, recordOut, "recorded review result:")
	resultFile := filepath.Join(dir, ".yard", "review-results", "pr-review-123-pr-review-a.yaml")
	resultData := readFile(t, resultFile)
	assertContains(t, resultData, "status: clear")

	out, err := runYardErrEnv(bin, dir, []string{"PATH=" + binDir + string(os.PathListSeparator) + os.Getenv("PATH")}, "--config", configPath, "ready", "feature", "--review-lane", "pr-review-a", "--write")
	if err != nil {
		t.Fatalf("ready feature: %v\n%s", err, out)
	}
	assertContains(t, out, "review lane")
	assertContains(t, out, "clear")
	if _, err := os.Stat(tmuxMarker); !os.IsNotExist(err) {
		t.Fatalf("ready should not capture tmux when structured review result exists, stat error: %v", err)
	}

	head := strings.TrimSpace(runGit(t, repo, "rev-parse", "HEAD"))
	writeFile(t, resultFile, fmt.Sprintf(`pr_number: 123
task_id: feature
lane: pr-review-123-pr-review-a
head: %s
status: clear
priorities:
  - P2
recorded_at: "2026-01-01T00:00:00Z"
`, head))
	out, err = runYardErrEnv(bin, dir, []string{"PATH=" + binDir + string(os.PathListSeparator) + os.Getenv("PATH")}, "--config", configPath, "ready", "feature", "--review-lane", "pr-review-a")
	if err == nil {
		t.Fatalf("ready should fail when structured review result records blocking priorities\n%s", out)
	}
	assertContains(t, out, "structured review result has blocking priorities P2")

	writeFile(t, resultFile, fmt.Sprintf(`pr_number: 123
task_id: feature
lane: pr-review-123-pr-review-a
head: %s
status: clear
recorded_at: "2026-01-01T00:00:00Z"
`, head))
	pushRepo := filepath.Join(dir, "push-repo")
	runGit(t, dir, "clone", "--branch", "feature-task", origin, pushRepo)
	runGit(t, pushRepo, "config", "user.name", "Yard Test")
	runGit(t, pushRepo, "config", "user.email", "yard@example.com")
	writeFile(t, filepath.Join(pushRepo, "later.txt"), "later\n")
	runGit(t, pushRepo, "add", "later.txt")
	runGit(t, pushRepo, "commit", "-m", "later")
	runGit(t, pushRepo, "push", "origin", "feature-task")

	out, err = runYardErrEnv(bin, dir, []string{"PATH=" + binDir + string(os.PathListSeparator) + os.Getenv("PATH")}, "--config", configPath, "ready", "feature", "--review-lane", "pr-review-a")
	if err == nil {
		t.Fatalf("ready should fail when structured review result is stale for remote PR head\n%s", out)
	}
	assertContains(t, out, "review result is stale for current PR HEAD")
}

func TestReadyFailsWhenLocalHeadIsUnpushed(t *testing.T) {
	bin := buildYard(t)
	dir := t.TempDir()
	binDir := filepath.Join(dir, "bin")
	origin := filepath.Join(dir, "origin.git")
	repo := filepath.Join(dir, "repo")
	configPath := filepath.Join(dir, "yard.yaml")

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
	runGit(t, repo, "checkout", "-b", "feature-task")
	writeFile(t, filepath.Join(repo, "feature.txt"), "feature\n")
	runGit(t, repo, "add", "feature.txt")
	runGit(t, repo, "commit", "-m", "feature")
	runGit(t, repo, "push", "-u", "origin", "feature-task")
	writeFile(t, filepath.Join(repo, "unpushed.txt"), "unpushed\n")
	runGit(t, repo, "add", "unpushed.txt")
	runGit(t, repo, "commit", "-m", "unpushed")

	writeExecutable(t, filepath.Join(binDir, "gh"), `#!/bin/sh
if [ "$1" = "pr" ] && [ "$2" = "view" ]; then
  echo "{\"number\":123,\"url\":\"https://github.com/o/r/pull/123\",\"state\":\"OPEN\",\"headRefName\":\"feature-task\",\"baseRefName\":\"main\",\"mergeStateStatus\":\"CLEAN\",\"reviewDecision\":\"APPROVED\",\"statusCheckRollup\":[{\"name\":\"test\",\"status\":\"COMPLETED\",\"conclusion\":\"SUCCESS\"}]}"
  exit 0
fi
echo "unexpected gh args: $*" >&2
exit 1
`)
	writeFile(t, configPath, fmt.Sprintf(`repo: %q
base_branch: main
default_remote: origin
github:
  owner: o
  repo: r
agents:
  implementation:
    command: codex
  local_review:
    command: codex
  pr_review:
    command: codex
`, repo))
	writeFile(t, filepath.Join(dir, "tasks.yaml"), fmt.Sprintf(`tasks:
  - id: feature
    issue: 338
    checkbox: Feature task
    service_family: feature
    branch: feature-task
    worktree: %q
    status: pr_opened
    pr_url: "https://github.com/o/r/pull/123"
    pr_number: 123
`, repo))

	out, err := runYardErrEnv(bin, dir, []string{"PATH=" + binDir + string(os.PathListSeparator) + os.Getenv("PATH")}, "--config", configPath, "ready", "feature")
	if err == nil {
		t.Fatalf("ready should fail for unpushed local HEAD\noutput:\n%s", out)
	}
	assertContains(t, out, "local HEAD is not contained in origin/feature-task")
	tasksData := readFile(t, filepath.Join(dir, "tasks.yaml"))
	assertContains(t, tasksData, "status: pr_opened")
}

func TestReadyFailsCommittedWhitespaceDiff(t *testing.T) {
	bin := buildYard(t)
	dir := t.TempDir()
	binDir := filepath.Join(dir, "bin")
	origin := filepath.Join(dir, "origin.git")
	repo := filepath.Join(dir, "repo")
	configPath := filepath.Join(dir, "yard.yaml")

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
	runGit(t, repo, "checkout", "-b", "feature-task")
	writeFile(t, filepath.Join(repo, "bad.txt"), "bad trailing \n")
	runGit(t, repo, "add", "bad.txt")
	runGit(t, repo, "commit", "-m", "feature")
	runGit(t, repo, "push", "-u", "origin", "feature-task")

	writeExecutable(t, filepath.Join(binDir, "gh"), `#!/bin/sh
if [ "$1" = "pr" ] && [ "$2" = "view" ]; then
  echo "{\"number\":123,\"url\":\"https://github.com/o/r/pull/123\",\"state\":\"OPEN\",\"headRefName\":\"feature-task\",\"baseRefName\":\"main\",\"mergeStateStatus\":\"CLEAN\",\"reviewDecision\":\"APPROVED\",\"statusCheckRollup\":[{\"name\":\"test\",\"status\":\"COMPLETED\",\"conclusion\":\"SUCCESS\"}]}"
  exit 0
fi
echo "unexpected gh args: $*" >&2
exit 1
`)
	writeFile(t, configPath, fmt.Sprintf(`repo: %q
base_branch: main
default_remote: origin
github:
  owner: o
  repo: r
agents:
  implementation:
    command: codex
  local_review:
    command: codex
  pr_review:
    command: codex
`, repo))
	writeFile(t, filepath.Join(dir, "tasks.yaml"), fmt.Sprintf(`tasks:
  - id: feature
    issue: 338
    checkbox: Feature task
    service_family: feature
    branch: feature-task
    worktree: %q
    status: pr_opened
    pr_url: "https://github.com/o/r/pull/123"
    pr_number: 123
`, repo))

	out, err := runYardErrEnv(bin, dir, []string{"PATH=" + binDir + string(os.PathListSeparator) + os.Getenv("PATH")}, "--config", configPath, "ready", "feature")
	if err == nil {
		t.Fatalf("ready should fail for committed whitespace errors\noutput:\n%s", out)
	}
	assertContains(t, out, "diff check")
	assertContains(t, out, "trailing whitespace")
}

func TestGCPruneMergedRemovesRunStateAndReviewWorktree(t *testing.T) {
	bin := buildYard(t)
	dir := t.TempDir()
	binDir := filepath.Join(dir, "bin")
	configPath := filepath.Join(dir, "yard.yaml")
	reviewDir := filepath.Join(dir, ".yard", "reviews", "pr-123-pr-review-a")
	runDir := filepath.Join(dir, ".yard", "runs", "feature")
	resultFile := filepath.Join(dir, ".yard", "review-results", "pr-review-123-pr-review-a.yaml")

	writeExecutable(t, filepath.Join(binDir, "git"), `#!/bin/sh
if [ "$1" = "status" ]; then
  exit 0
fi
if [ "$1" = "worktree" ] && [ "$2" = "remove" ]; then
  rm -rf "$3"
  exit 0
fi
echo "unexpected git args: $*" >&2
exit 1
`)
	writeFile(t, filepath.Join(runDir, "implement.md"), "prompt\n")
	writeFile(t, filepath.Join(reviewDir, ".git"), "gitdir: fake\n")
	writeFile(t, resultFile, "status: clear\n")
	writeFile(t, configPath, `repo: "."
agents:
  implementation:
    command: codex
  local_review:
    command: codex
  pr_review:
    command: codex
`)
	writeFile(t, filepath.Join(dir, "tasks.yaml"), `tasks:
  - id: feature
    issue: 338
    checkbox: Feature task
    service_family: feature
    branch: feature-task
    worktree: ""
    status: merged
    pr_url: "https://github.com/o/r/pull/123"
    pr_number: 123
`)

	out, err := runYardErrEnv(bin, dir, []string{"PATH=" + binDir + string(os.PathListSeparator) + os.Getenv("PATH")}, "--config", configPath, "gc", "--prune", "--merged")
	if err != nil {
		t.Fatalf("gc prune: %v\n%s", err, out)
	}
	assertContains(t, out, "removed 3 candidate(s)")
	if _, err := os.Stat(runDir); !os.IsNotExist(err) {
		t.Fatalf("run dir should be removed, stat err: %v", err)
	}
	if _, err := os.Stat(reviewDir); !os.IsNotExist(err) {
		t.Fatalf("review dir should be removed, stat err: %v", err)
	}
	if _, err := os.Stat(resultFile); !os.IsNotExist(err) {
		t.Fatalf("review result should be removed, stat err: %v", err)
	}
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
	return runYardErrEnv(bin, dir, nil, args...)
}

func runYardErrEnv(bin, dir string, env []string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = dir
	if env != nil {
		cmd.Env = append(os.Environ(), env...)
	}
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

func writeExecutable(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create dir for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(data), 0o755); err != nil {
		t.Fatalf("write executable %s: %v", path, err)
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

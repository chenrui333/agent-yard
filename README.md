# agent-yard

agent-yard is a thin local orchestration CLI for running multiple coding and review agents across git worktrees. The binary is named yard.

It is intentionally generic: tmux is the durable execution backend for agent lanes, git worktrees are the implementation isolation boundary, and GitHub issues and pull requests are an optional collaboration boundary.

## Non-goals

- No web dashboard.
- No daemon or autonomous supervisor.
- No custom terminal multiplexer.
- No replacement for tmux, git, gh, Codex, or Claude.
- No SQLite, MCP, Ghostty, or iTerm automation in the MVP.

## Install From Source

    go install github.com/chenrui333/agent-yard/cmd/yard@latest

For local development:

    go test ./...
    go run ./cmd/yard --help

## Releases

Releases are built by GoReleaser from immutable version tags:

    git tag v0.1.0
    git push origin v0.1.0

The release workflow publishes tarballs for:

- macOS x86_64
- macOS arm64
- Linux x86_64
- Linux arm64

GoReleaser is configured to keep existing release notes and artifacts for an already-published tag instead of replacing them. Create a new tag for a corrected release.

Renovate is configured for dependency PRs with semantic commit titles, a two-day minimum release age, strict internal checks, and PR automerge for non-major updates.

## Required Tools

- git
- gh
- tmux
- codex
- claude is optional for future review lanes

## Quickstart

Initialize local state in the orchestration repository:

    yard init
    yard doctor

Edit yard.yaml and tasks.yaml. Then inspect the board:

    yard status
    yard board

Create a task worktree:

    yard worktree aws-route53

Launch one task or a small wave:

    yard launch aws-route53 --dry-run
    yard launch-wave --limit 2 --dry-run
    yard wave plan --limit 3
    yard wave prepare --limit 3 --dry-run
    yard wave launch --limit 3 --dry-run

Open a pull request after the task branch is ready:

    yard pr aws-route53 --dry-run

Inspect or attach to tmux-backed lanes:

    yard attach
    yard attach aws-route53
    yard capture aws-route53

## Sample yard.yaml

    repo: $HOME/src/terraformer
    base_branch: main
    default_remote: origin
    session: tf-aws

    github:
      owner: chenrui333
      repo: terraformer
      issue: 338

    worktrees:
      root: $HOME/src
      prefix: terraformer.

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

## Sample tasks.yaml

    tasks:
      - id: aws-route53
        issue: 338
        checkbox: Route53 resources
        service_family: route53
        branch: aws-route53-resources
        worktree: $HOME/src/terraformer.aws-route53-resources
        status: ready
        assigned_agent: impl-01
        pr_url: ""
        pr_number: 0

## Generic Multi-Agent Workflow

1. Add one tasks.yaml entry per issue checkbox.
2. Run yard worktree TASK_ID to create a branch-specific git worktree from origin/main.
3. Run yard launch TASK_ID to start the implementation lane in tmux.
4. Run yard review-local TASK_ID before opening a PR.
5. Run yard pr TASK_ID when the branch is ready.
6. Run yard review-pr PR_NUMBER --lane pr-review-a to launch an isolated no-push PR review lane.
7. Use yard status and yard board as the coordinator view.

For larger waves, use yard wave plan to select distinct service families when possible, yard wave prepare to claim lanes and create worktrees, and yard wave launch to start the tmux sessions.

Terraformer AWS coverage is a good example campaign for this model, but project-specific implementation rules belong in local prompt templates rather than the built-in defaults.

## Safety Model

- tmux owns long-running interactive sessions.
- git worktrees isolate implementation tasks.
- tasks.yaml is locked during writes and replaced atomically.
- yard status derives worktree, dirty, tmux, and PR hints from reality when available.
- GitHub mutations require explicit commands; claim comments require --comment, and PR creation supports --dry-run.
- Review prompts instruct agents not to push code.

## Roadmap

- list and show commands.
- Shell completions from Cobra.
- Better GitHub issue checkbox reconciliation.
- Optional Homebrew formula notes once the CLI shape stabilizes.

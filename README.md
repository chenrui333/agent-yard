# agent-yard

agent-yard is a thin local orchestration CLI for running coding and review agents across git worktrees. The binary is named yard.

It uses tmux as the durable execution backend, git worktrees as the implementation isolation boundary, and GitHub issues and pull requests as the collaboration boundary.

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

## Required Tools

- git
- gh
- tmux
- codex
- claude is optional for future review lanes

## Quickstart

Initialize local state in the orchestration repository:

    yard init

Edit yard.yaml and tasks.yaml. Then inspect the board:

    yard status
    yard board

Create a task worktree:

    yard worktree aws-route53

Launch one task or a small wave:

    yard launch aws-route53 --dry-run
    yard launch-wave --limit 2 --dry-run

Open a pull request after the task branch is ready:

    yard pr aws-route53 --dry-run

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

## Terraformer AWS Workflow

1. Add one tasks.yaml entry per issue checkbox.
2. Run yard worktree TASK_ID to create a branch-specific git worktree from origin/main.
3. Run yard launch TASK_ID to start the implementation lane in tmux.
4. Run yard review-local TASK_ID before opening a PR.
5. Run yard pr TASK_ID when the branch is ready.
6. Run yard review-pr PR_NUMBER to launch a no-push PR review lane.
7. Use yard status and yard board as the coordinator view.

## Safety Model

- tmux owns long-running interactive sessions.
- git worktrees isolate implementation tasks.
- tasks.yaml is locked during writes and replaced atomically.
- yard status derives worktree, dirty, tmux, and PR hints from reality when available.
- GitHub mutations require explicit commands; claim comments require --comment, and PR creation supports --dry-run.
- Review prompts instruct agents not to push code.

## Roadmap

- doctor command for dependency checks.
- attach, list, show, and set-status commands.
- Shell completions from Cobra.
- Better GitHub issue checkbox reconciliation.
- Optional Homebrew formula notes once the CLI shape stabilizes.

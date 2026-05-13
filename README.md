# agent-yard

agent-yard is a thin local orchestration CLI for running multiple coding and review agents across git worktrees. The binary is named yard.

It is intentionally generic: tmux is the durable execution backend for agent lanes, git worktrees are the implementation isolation boundary, and GitHub issues and pull requests are an optional collaboration boundary.

See [docs/architecture.md](docs/architecture.md) for the commander/worker/reviewer model and state boundaries.

## Non-goals

- No web dashboard.
- No daemon or autonomous supervisor.
- No custom terminal multiplexer.
- No replacement for tmux, git, gh, Codex, or Claude.
- No SQLite, MCP, Ghostty, or iTerm automation.

## Install From Source

    go install github.com/chenrui333/agent-yard/cmd/yard@latest

For local development:

    go test ./...
    go run ./cmd/yard --help

## Releases

Releases are built by GoReleaser from immutable version tags:

    git tag -a v0.0.3 -m "v0.0.3"
    git push origin v0.0.3

The release workflow publishes tarballs for:

- macOS x86_64
- macOS arm64
- Linux x86_64
- Linux arm64

GoReleaser is configured to keep existing release notes and artifacts for an already-published tag instead of replacing them. Create a new tag for a corrected release.

Renovate is configured for dependency PRs with semantic commit titles, a two-day minimum release age, strict internal checks, and PR automerge for non-major updates.

## Required Tools

- git
- gh for GitHub issue and pull request commands
- tmux
- codex
- claude is optional when configured as an agent command

## Quickstart

Initialize local state in the orchestration repository:

    yard init
    yard doctor

Edit yard.yaml, then import issue checkboxes or edit tasks.yaml directly. Inspect the board:

    yard sync issue 338 --write --id-prefix aws- --branch-prefix aws-

    yard status
    yard board
    yard show aws-route53

Launch the commander terminal for a larger workset campaign:

    yard commander --goal "finish the current maintenance wave" --dry-run
    yard commander --goal "finish the current maintenance wave"

Create a task worktree:

    yard worktree aws-route53

Launch one task or a small wave:

    yard launch aws-route53 --dry-run
    yard wave plan --limit 3
    yard wave prepare --limit 3 --dry-run
    yard wave launch --limit 3 --dry-run

Use `--reuse-idle` to reuse an existing idle shell tmux window, or `--replace-window` to explicitly kill and recreate it. `--force` only bypasses dirty-worktree checks.

`yard launch-wave` remains available as a compatibility alias for `yard wave launch`.

Open a pull request after the task branch is ready:

    yard pr aws-route53 --dry-run
    yard pr aws-route53
    yard ready aws-route53 --review-lane pr-review-a --write

Inspect or attach to tmux-backed lanes:

    yard attach
    yard attach aws-route53
    yard lanes
    yard capture aws-route53 --tail 80
    yard gc
    yard gc --prune --merged

## Sample yard.yaml

    repo: $HOME/src/terraformer
    base_branch: main
    default_remote: origin
    session: tf-aws

    github:
      host: github.com
      owner: chenrui333
      repo: terraformer
      issue: 338

    worktrees:
      root: $HOME/src
      prefix: terraformer.

    agents:
      commander:
        command: codex
        args:
          - exec
          - --sandbox
          - danger-full-access
      implementation:
        command: codex
        args:
          - exec
          - --sandbox
          - danger-full-access
      local_review:
        command: codex
        args:
          - exec
          - --sandbox
          - danger-full-access
      pr_review:
        command: codex
        args:
          - exec
          - --sandbox
          - danger-full-access

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

1. Import issue checkboxes with yard sync issue ISSUE --write, or add one tasks.yaml entry per task.
2. Run yard commander for the mother/orchestrator terminal when coordinating a larger campaign.
3. Run yard wave plan, yard wave prepare, and yard wave launch to allocate lanes and start worker terminals.
4. Run yard review-local TASK_ID before opening a PR.
5. Run yard pr TASK_ID when the branch is ready; it validates the worktree, pushes the branch by default, and records an existing PR when one already exists.
6. Run yard review-pr PR_NUMBER --lane pr-review-a to launch an isolated no-push PR review lane.
7. Run yard review-result TASK_ID --lane pr-review-a once the reviewer reports no P1/P2/P3 TODO findings.
8. Run yard ready TASK_ID --review-lane pr-review-a --write once CI is green and the review result is clear.
9. Use yard status, yard board, yard show, yard lanes, attach, capture, and gc as the coordinator view and cleanup loop.

For larger waves, yard wave commands select distinct service families when possible and reserve lanes already occupied by live impl-* tmux windows.

Terraformer AWS coverage is a good example campaign for this model, but project-specific implementation rules belong in local prompt templates rather than the built-in defaults.

## Paired Workset Loop

agent-yard is designed around independent paired worksets:

    workset-1
      terminal-1: implementation agent
      terminal-2: review agent
      boundary: one git worktree, one branch, one pull request

    workset-2
      terminal-3: implementation agent
      terminal-4: review agent
      boundary: another git worktree, branch, and pull request

Each workset can bounce between implementation and review without blocking the others. The commander terminal coordinates the loop, the worker terminal writes code in the assigned worktree, and the reviewer terminal is separate with full local access for inspection, build, and tests but does not own implementation changes. For Codex PR review, run the review command from the review terminal:

    /review https://github.com/OWNER/REPO/pull/NUMBER

The commander keeps the loop moving:

1. Launch the implementation terminal.
2. Launch the paired local or PR review terminal.
3. Watch build and review state with yard board, yard show, yard lanes, and GitHub checks.
4. Treat P1/P2/P3 TODO comments as required follow-up.
5. Route fixes back to the implementation terminal; do not patch the assigned worktree directly from the commander or reviewer lane.
6. Update the pull request title or body after meaningful commits.
7. Record a structured review result with yard review-result when the reviewer is clear.
8. Repeat until the build is green and yard ready passes.

beads/bd can be used as optional long-lived memory and backlog support for the commander. yard does not require beads; tasks.yaml remains the execution ledger for active tmux/worktree lanes.

## Safety Model

- tmux owns long-running interactive sessions.
- Existing tmux windows are not reused unless `--reuse-idle` or `--replace-window` is explicit.
- git worktrees isolate implementation tasks.
- tasks.yaml is locked during writes and replaced atomically.
- yard board stays cheap for coordinator refreshes; yard status and yard show perform richer probes when detail is needed.
- yard status derives worktree, dirty, tmux, and PR hints from reality when available.
- GitHub mutations require explicit commands; claim comments require --comment, and PR creation supports --dry-run.
- PR creation pushes task branches by default after local preflights; use --no-push only when another process owns pushing.
- Review prompts instruct agents not to push code.

## Roadmap

- Shell completions from Cobra.
- richer task filtering.
- Optional Homebrew formula notes once the CLI shape stabilizes.

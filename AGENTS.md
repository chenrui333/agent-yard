# agent-yard Agent Guidance

agent-yard is a generic local orchestration tool for running multiple coding and review agents through tmux-backed sessions.

## Product Scope

- Keep the tool generic. Terraformer AWS is an example campaign, not the product boundary.
- Use tmux as the durable execution backend for multi-agent lanes.
- Use git worktrees as the isolation boundary for implementation agents.
- Use GitHub issues and pull requests as an optional collaboration boundary.
- Keep the CLI scriptable and boring; do not add a web dashboard, daemon, database, or terminal multiplexer.

## Implementation Guidance

- Prefer small Go packages with clear responsibilities under `internal/`.
- Cobra command files should parse flags and call internal logic; avoid burying core behavior in command setup.
- External tools should stay external: call `git`, `gh`, and `tmux` through wrappers instead of reimplementing them.
- Long-running interactive agent commands belong in tmux, not streamed through the process wrapper.
- Keep prompt defaults generic. Put campaign-specific instructions in local prompt templates when a user needs them.

## Paired Workset Model

The standard agent-yard unit is a paired workset:

- One git worktree.
- One focused branch and pull request.
- One tmux terminal running the implementation agent.
- One separate tmux terminal running the review agent.

Scale by adding independent worksets, not by sharing terminals or worktrees. For example, workset 1 can use terminal 1 for implementation and terminal 2 for review, while workset 2 uses terminal 3 for implementation and terminal 4 for review. Each workset should be able to progress, block, review, and finish without interfering with the others.

The dispatcher coordinates these pairs. It should not become an autonomous supervisor; it keeps the board visible, assigns lanes, launches terminals, and moves review feedback between the implementation and review terminals.

## Agent Loop Running Process

For each workset, run this loop until the pull request is ready:

1. The implementation agent changes code only inside the assigned worktree and produces focused commits.
2. The review agent runs in a separate terminal against the same workset boundary and does not push code.
3. For pull-request review, the review terminal may run:

   ```text
   /review https://github.com/OWNER/REPO/pull/NUMBER
   ```

4. Treat P1/P2/P3 review findings or TODO comments as required follow-up work.
5. Send follow-up work back to the implementation terminal or patch it directly in the assigned worktree.
6. After meaningful commits, update the pull request title or body so reviewers can understand the current scope without reconstructing history.
7. Use `yard ready TASK_ID --review-lane LANE --write` as the final gate once CI is green and the review terminal reports no P1/P2/P3 TODO comments.
8. Repeat until the readiness gate passes.

## Safety

- Protect `tasks.yaml` writes with the task store lock and atomic save path.
- Derive status from the local system where possible instead of trusting stale YAML.
- Treat live `impl-*` tmux windows as reserved lanes when planning or launching waves.
- Keep `yard gc` report-only by default; destructive cleanup requires explicit prune flags.
- Avoid destructive git operations unless the user explicitly asks for them.
- Keep review lanes no-push by prompt and by workflow convention.

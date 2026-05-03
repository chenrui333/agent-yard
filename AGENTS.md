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

## Safety

- Protect `tasks.yaml` writes with the task store lock and atomic save path.
- Derive status from the local system where possible instead of trusting stale YAML.
- Avoid destructive git operations unless the user explicitly asks for them.
- Keep review lanes no-push by prompt and by workflow convention.

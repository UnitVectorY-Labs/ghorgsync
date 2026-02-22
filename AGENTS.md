# ghorgsync Agent Notes

This is a Go based command line application the follows idiomatic Go conventions. Its purpose is to sync a local directory of git repositories with the corresponding repositories on GitHub. The main.go file in the root provides the entry point, all other code is organised into packages under the `internal/` directory. The `docs/` directory contains markdown files that document the behavior and configuration of the application, and should be updated in tandem with code changes to ensure they remain accurate.

## Repo Conventions For The Agent
- Treat `docs/` as required source of truth alongside code. When behavior changes, update the matching doc pages in the same PR.
- Keep dependencies minimal and prefer stdlib. Add third-party packages only when they materially improve correctness or output quality.

## Application Behavior That Must Not Drift

- Startup gate: only run when a repo-root dotfile named after the executable exists (for example `.ghorgsync`). If missing, emit a short message and exit successfully without doing anything else.
- Non-destructive rules (hard constraints):
  - never delete directories
  - never discard local changes
  - never run destructive git commands (no `reset --hard`, no `clean -fd`, no force checkout)
- Default branch is per-repository and must be sourced from GitHub metadata, not assumed.
- Decision rules per repository:
  - Always safe: `fetch`.
  - If working tree is clean: checkout default branch then pull.
  - If working tree is dirty (staged or unstaged): do not checkout or pull; report status instead.

## Output Rules For The Agent To Preserve

- Default output is quiet: only print actions and exceptions (cloned, updated, dirty, branch drift, unknown folders, excluded-but-present, API/auth failures).
- Output must be consistently structured per repo and use color to signal status.

## Testing and Validation

- Testing is limited to unit tests of core logic. No integration tests or mocks of GitHub API or git commands.
- Never interact with real repositories or the GitHub API in tests or when testing.

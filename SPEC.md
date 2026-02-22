# ghorgsync Specification (v1)

## Purpose

`ghorgsync` is a non-interactive Go command-line application that synchronizes a local directory with the repositories in a GitHub organization.

The command is designed to:

- Clone missing repositories from a GitHub organization into the current working directory.
- Fetch updates for existing local repositories.
- Audit repository state (default branch drift and dirty working trees).
- Warn about unexpected local folders and excluded-but-present repositories.
- Present a high-quality, structured, colorized terminal report.

This document defines **requirements for implementation**. It is not an implementation guide and does not prescribe a specific Git or GitHub client library.

## Scope

### In Scope (v1)

- GitHub organization repository inventory (including default branch metadata)
- Local clone syncing in the directory where the command is run
- Non-destructive auditing of local repository state
- Quiet-by-default reporting with structured exception output
- Cross-platform support (macOS, Linux, Windows)
- Configuration via a repo-root dotfile named after the executable

### Out of Scope (v1)

- Interactive terminal UI (TUI)
- Deleting unknown folders or excluded repositories
- Resolving merge conflicts or local branch repair
- Automatic stashing or discarding local changes
- Rewriting git history or any destructive git actions
- Full integration tests against real GitHub or real repositories
- GitHub Enterprise support (unless added in a later spec revision)

## Non-Destructive Contract (Hard Constraints)

The following behaviors are mandatory and must not drift:

- The application **must never delete directories**.
- The application **must never discard local changes**.
- The application **must never run destructive git commands** (including `git reset --hard`, `git clean -fd`, force checkout, or equivalent library operations).
- `fetch` is always allowed and considered safe.
- If a repository is dirty (staged or unstaged changes), the app must not checkout branches or pull.

## Startup Gate and Execution Model

### Startup Gate

- The command only operates when a dotfile named after the executable exists in the current working directory.
- For the `ghorgsync` executable, the required file is `.ghorgsync`.
- On Windows, the required dotfile name is still based on the executable base name (for example, `ghorgsync.exe` still maps to `.ghorgsync`).
- If the dotfile is missing:
  - The command prints a short message indicating configuration is missing.
  - The command exits successfully (`0`).
  - The command performs no further actions (no API calls, no local scanning, no git operations).

### Execution Model

- The command is non-interactive.
- The command runs once, prints output, and exits.
- The default invocation (`ghorgsync`) performs the sync/audit workflow.

## Configuration Requirements

### Configuration File

- The configuration file is `.ghorgsync` in the current working directory.
- v1 configuration format must be **YAML** (to allow stdlib parsing and minimize dependencies).
- The file must be UTF-8 text.

### Minimum Configuration

- `organization` (string) is required. (application fails with config error if missing or empty)

### Optional Configuration (v1)

- `include_public` (boolean, default `true`)
- `include_private` (boolean, default `true`)
- `exclude_repos` (array of exact repository names, default empty; supports regex patterns for matching)

### Configuration Validation

- If both `include_public` and `include_private` are `false`, configuration is invalid. App exits with config error.
- Invalid YAML must produce a clear configuration error.
- Invalid regex patterns must produce a clear configuration error that identifies the offending pattern.
- Exclusion matching is performed against the GitHub repository name only (not full path or URL that includes the organization).

### Example Configuration (v1)

```yaml
organization: example-org
include_public: true
include_private: true
exclude_repos:
  - legacy-repo
  - "^sandbox-"
  - "-archive$"
```

## Authentication and GitHub Inventory Requirements

### GitHub Inventory

- The application must retrieve the repository list for the configured organization from GitHub.
- The inventory must include, at minimum, for each repository:
  - repository name
  - clone URL (HTTPS or SSH, implementation choice)
  - default branch name
  - visibility (public/private) for filtering
- The application must not assume `main` or `master`; default branch must come from GitHub metadata per repository.

### Filtering

- Repository visibility filters (`include_public`, `include_private`) are applied to the GitHub inventory.
- Exclusions are applied after visibility filtering.
- Excluded repositories are considered part of the managed namespace for local classification (see "Unknown and Excluded Local Folders").

### Authentication

- v1 must support token-based GitHub authentication via environment variables (`GITHUB_TOKEN` and/or `GH_TOKEN`).
- If the user is authenticated with the GitHub CLI (`gh`), the application may use the CLI's authentication context as a fallback if direct token env vars are not set.
- Tokens must never be printed in logs or error messages.
- If private repositories are requested and authentication is missing or insufficient, the command must report an auth/API error clearly.
- Public-only operation may proceed without authentication if GitHub API access is possible, but rate-limit or auth failures must be reported clearly.

## Local Directory Classification Requirements

Local inspection is performed against the immediate child entries of the current working directory (non-recursive).

### Managed Repository Paths

- Each included, non-excluded GitHub repository maps to a local directory path `./<repo-name>`.
- If a managed repository path does not exist, it is a clone candidate.

### Unknown and Excluded Local Folders

- `unknown folder`:
  - An immediate child directory that does not correspond to any included repository and does not correspond to an excluded repository.
- `excluded-but-present`:
  - An immediate child directory whose name matches a repository excluded by exact name or pattern.
- These categories must be reported separately in output.
- The application must not delete or modify unknown/excluded folders.

### Path Collisions

- If a managed repository path exists but is not a usable git repository clone for that repo, it must be reported as a path/repo collision and skipped.
- Examples include:
  - regular file at the expected path
  - non-git directory at the expected path
  - git repository whose remote does not match the expected GitHub repository (remote mismatch)

## Per-Repository Sync and Audit Workflow

For each included, non-excluded repository:

1. Determine whether the repository exists locally.
2. If missing, clone it and report the clone action.
3. If present and valid, perform audit/sync processing.

### Clone Behavior

- Missing repositories must be cloned into `./<repo-name>`.
- Clone failures must be reported per repository and processing continues with other repositories.
- A successful clone must be logged (quiet mode exception/action output).

### Existing Repository Processing

#### Required Checks

The command must inspect and determine:

- whether the working tree is dirty
  - dirty includes staged changes
  - dirty includes unstaged changes
  - dirty includes untracked files (treated as unstaged local changes)
- current branch
- expected default branch (from GitHub metadata)

#### Required Git Operation Order

- `fetch` must be run for valid local repositories (safe operation).
- If the repository is dirty:
  - do not checkout another branch
  - do not pull
  - report dirty state and branch/default branch audit info
- If the repository is clean:
  - if not currently on the **default branch** (as defined by GitHub metadata), checkout the default branch (non-force)
  - pull latest changes on the default branch

#### Pull Safety

- Pull operations must be non-destructive.
- v1 pull behavior should require fast-forward only semantics (`--ff-only` or equivalent).
- If checkout or pull fails, the error must be reported and processing continues with other repositories.

## Dirty Repository Audit Requirements

When a repository is dirty, the output must include enough detail for the user to take action manually.

### Required Dirty Output Details

- Repository name/path
- Current branch
- Default branch
- Indication that checkout/pull was skipped
- A list of changed file paths (staged and/or unstaged)
- Distinction between staged and unstaged changes
- A concise line-count summary (additions/deletions) when available from git

The tool does not need to print full diffs.

## Branch Drift Audit Requirements

- A repository is in branch drift when `current_branch != default_branch`.
- Branch drift must be reported when:
  - a dirty repository is not on its default branch (informational warning; no auto-correction)
  - a clean repository required checkout to return to the default branch (action taken and logged)
- Default branch names are per-repository and must not be inferred from local branch names.

## Output and Terminal UX Requirements

### Output Philosophy

- Default output is quiet.
- The command should print:
  - actions taken (cloned, updated, branch checkout/pull when changes occurred)
  - exceptions/findings (dirty repos, branch drift, unknown folders, excluded-but-present, collisions, auth/API/git errors)
  - final summary
- The command should not print a line for every repo that was checked and already up to date with no notable events.

### Structure

- Output must be consistently structured so each line/block clearly identifies:
  - entity type (`repo`, `folder`, `system`)
  - target name/path
  - status category
  - brief detail
- Repository-related findings should be grouped by repository in a readable format.

### Color

- Output must use color to signal status categories when color is enabled and stdout is a TTY.
- Color usage must improve readability but remain legible without color (text labels still required).
- The application should honor `NO_COLOR`.
- The application should provide a `--no-color` override.

### Quiet/Verbose Controls

- Default mode is quiet (as defined above).
- v1 should provide a `--verbose` mode to show additional per-repository processing detail.

### Summary

- A final summary line/block must include counts at minimum for:
  - total repos in inventory (post-filter)
  - cloned
  - updated
  - dirty
  - branch drift findings/actions
  - unknown folders
  - excluded-but-present
  - errors

## Command-Line Interface Requirements (v1)

The core command remains a single non-interactive command invocation. v1 should include:

- `ghorgsync` (default run)
- `--help`
- `--version`
- `--verbose`
- `--no-color`

Subcommands are not required for v1.

## Error Handling and Exit Codes

### Error Handling

- Failures should be isolated per repository whenever possible.
- A failure for one repository must not stop processing of other repositories unless the failure is global (for example, invalid configuration or inability to fetch GitHub inventory).

### Exit Codes (v1)

- `0`: command completed (including runs with audit findings such as dirty repos, branch drift, unknown folders, or missing config dotfile)
- `1`: command failed due to configuration error, authentication/API failure, or other operational errors that prevented normal completion

Note: audit findings are user-facing warnings, not command failures.

## Cross-Platform and Dependency Requirements

### Platform Support

- The binary must support macOS, Linux, and Windows.
- Path handling must use Go's cross-platform filesystem/path semantics.
- Terminal color behavior must degrade gracefully on platforms/terminals without ANSI support.

### Git Backend and GitHub Client

- The implementation may use:
  - local `git` CLI commands, or
  - a Go git library,
  - and either direct GitHub REST API calls or a Go GitHub client library
- The spec does not mandate a specific library choice.
- Dependency use must be minimized; prefer the Go standard library unless a dependency materially improves correctness or output quality.

## Documentation Deliverables (Required with Implementation)

The following documentation must be kept up to date with behavior changes:

- `ghorgsync/README.md`
  - Marketing/overview: what the tool is, why it exists, key benefits
- `ghorgsync/docs/INSTALL.md`
  - Installation methods and prerequisites (including git and authentication requirements)
- `ghorgsync/docs/USAGE.md`
  - Command usage, config format, runtime behavior, output semantics, warnings, and non-destructive behavior
- `ghorgsync/docs/EXAMPLES.md`
  - Configuration examples and common scenarios (clean run, dirty repo, branch drift, unknown folder, excluded-but-present)
- `ghorgsync/docs/README.md`
  - Site landing page summary aligned with README

## Testing and Validation Requirements

Testing is intentionally limited and must avoid real GitHub and real repository interaction.

### Must Have (Unit Tests Only)

- Configuration parsing and validation
- Repository filtering logic (public/private + exclusions)
- Exclusion pattern matching
- Local folder classification logic (unknown vs excluded-but-present vs managed)
- Per-repository decision logic (dirty/clean -> allowed actions)
- Output formatting helpers (status labels, summary rendering) where practical

### Explicitly Out of Scope

- Integration tests against real GitHub API
- Mocks/fakes of GitHub API clients or git command execution as a substitute for integration testing
- Tests that execute real git operations
- Tests that require real credentials

## Implementation Tasks

### Task 1: Core Models and Config

- Define config schema and validation.
- Define repository inventory model and local classification model.
- Define status enums and output event types.
- Add unit tests for config and filtering logic.

### Task 2: GitHub Inventory + Filtering

- Implement GitHub org repository listing with pagination and auth support.
- Apply visibility and exclusion filtering.
- Add unit tests for filtering and exclusion behavior.

### Task 3: Local Scanning + Classification

- Scan current directory (non-recursive).
- Classify managed, unknown, excluded-but-present, and collision paths.
- Add unit tests for classification behavior.

### Task 4: Repository Sync/Audit Engine

- Implement per-repo workflow (clone, fetch, dirty detection, branch audit, checkout/pull rules).
- Enforce non-destructive guardrails in command sequencing.
- Add unit tests for decision logic using static inputs and parser helpers.

### Task 5: Terminal Output and CLI UX

- Implement structured colorized reporting and quiet/verbose modes.
- Add summary output and exit code handling.
- Validate output readability on macOS/Linux/Windows terminals.

### Task 6: Documentation Completion

- Update `README.md` and docs pages to match implemented behavior.
- Add examples for common operational scenarios and failure cases.

## Acceptance Criteria (Review Checklist)

The implementation is considered compliant with this spec when all of the following are true:

- The command does nothing (and exits `0`) when `.ghorgsync` is missing.
- The default branch is sourced per-repository from GitHub metadata.
- Dirty repositories are fetched but never checked out or pulled.
- Clean repositories are checked out to default branch (if needed) and pulled safely.
- Unknown folders and excluded-but-present folders are reported distinctly.
- Output is quiet by default and colorized/structured when enabled.
- No destructive file or git operations are performed.
- Core logic is covered by unit tests only (no real git/GitHub integration tests).
- Docs are updated to reflect behavior and configuration.

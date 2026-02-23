---
layout: default
title: Usage
nav_order: 3
permalink: /usage
---

# Usage
{: .no_toc }

## Table of Contents
{: .no_toc .text-delta }

- TOC
{:toc}

---

## Configuration File

`ghorgsync` is configured with a `.ghorgsync` YAML file placed in the directory where your repositories live. This is the same directory where you run the command.

### Configuration Options

| Option | Type | Default | Description |
|---|---|---|---|
| `organization` | string | *(required)* | GitHub organization name to sync |
| `include_public` | boolean | `true` | Include public repositories |
| `include_private` | boolean | `true` | Include private repositories |
| `exclude_repos` | array | `[]` | Repository names or regex patterns to exclude |

### Exclude Patterns

The `exclude_repos` list supports both exact names and regular expressions. Patterns are matched against the repository name only (not the full URL or organization-qualified path).

```yaml
exclude_repos:
  - legacy-repo          # exact match
  - "^sandbox-"          # regex: repos starting with "sandbox-"
  - "-archive$"          # regex: repos ending with "-archive"
```

Invalid regex patterns produce a clear configuration error that identifies the offending pattern.

### Configuration Validation

- `organization` is required; the command exits with an error if it is missing or empty.
- Setting both `include_public` and `include_private` to `false` is invalid.
- Invalid YAML produces a clear error message.

## Authentication

`ghorgsync` resolves a GitHub token using the following priority order:

1. `GITHUB_TOKEN` environment variable
2. `GH_TOKEN` environment variable
3. GitHub CLI (`gh auth token`) as a fallback

Tokens are never printed in logs or error messages. If private repositories are requested and authentication is missing or insufficient, the command reports an auth/API error. Public-only operation may work without authentication, but rate-limit or auth failures are reported clearly.

## Command-Line Flags

```
ghorgsync [flags]
```

| Flag | Description |
|---|---|
| `--help` | Print usage information and exit |
| `--version` | Print version and exit |
| `--verbose` | Enable verbose output (show per-repository processing detail) |
| `--no-color` | Disable color output |

## Runtime Behavior

### Startup Gate

The command only runs when a dotfile named after the executable exists in the current working directory. For the `ghorgsync` binary, this file is `.ghorgsync`.

If the dotfile is missing, the command prints a short message and exits successfully (`0`) without performing any API calls, scans, or git operations.

### Sync Workflow

When invoked, `ghorgsync` performs the following steps:

1. **Load configuration** from `.ghorgsync` and validate it.
2. **Resolve authentication** and connect to the GitHub API.
3. **Fetch the organization repository list** including default branch metadata.
4. **Filter repositories** by visibility (`include_public`/`include_private`) and exclusion patterns.
5. **Scan the local directory** and classify child entries (see [Local Directory Classification](#local-directory-classification)).
6. **Clone missing repositories**.
7. **Process existing repositories** (fetch, audit, conditionally checkout and pull).
8. **Report findings** (collisions, unknown folders, excluded-but-present).
9. **Print a summary line** with counts.

During the repository clone/process phase, `ghorgsync` shows a live progress bar on TTY output to indicate completion across managed repositories.

### Per-Repository Processing

For each included repository that exists locally:

1. **Fetch** — always performed (safe operation).
2. **Submodule initialization** — `git submodule update --init --recursive` is run after every fetch to initialize any uninitialized submodules, preventing them from appearing as untracked files and causing a false dirty state.
3. **Check dirty state** — detects staged changes, unstaged changes, and untracked files.
4. **If dirty:**
   - Do not checkout or pull.
   - Report the dirty state with current branch, default branch, changed files, and line counts.
5. **If clean:**
   - If not on the default branch, checkout the default branch (branch drift correction).
   - Pull with fast-forward-only semantics (`--ff-only`).
   - Run `git submodule update --init --recursive` again to update submodule pointers to match any new commits brought in by the pull.
   - Report whether the repo was updated or already current.

### Non-Destructive Guarantees

`ghorgsync` enforces hard constraints that must never be violated:

- **Never deletes directories** — unknown folders and excluded-but-present repos are reported but left untouched.
- **Never discards local changes** — dirty repos are skipped for checkout/pull operations.
- **Never runs destructive git commands** — no `git reset --hard`, no `git clean -fd`, no force checkouts.
- `fetch` is always considered safe and is always performed.
- `git submodule update --init --recursive` (without `--force`) is safe and will not overwrite local changes inside submodule directories.

## Submodule Support

`ghorgsync` handles repositories that contain git submodules:

- **Clone** — new repositories are cloned with `--recurse-submodules` so submodules are initialized immediately.
- **Existing repositories** — `git submodule update --init --recursive` is run after every fetch, before the dirty check. This ensures that uninitialized submodule directories are initialized and do not appear as untracked files causing a false dirty state.
- **After pull** — `git submodule update --init --recursive` is run again after a successful pull to update submodule pointers to the commits referenced by the new parent-repo state.

If submodule initialization fails (for example, due to a network error fetching a submodule remote), the error is reported as a `submodule-error` and processing of that repository stops. Other repositories continue normally.

## Output Semantics

### Quiet Default

By default, `ghorgsync` only prints:

- **Actions taken** — cloned, updated, branch checkout/pull
- **Findings** — dirty repos, branch drift, unknown folders, excluded-but-present, collisions, errors
- **Summary line** — counts for all categories

Repositories that are already up to date with no notable events produce no output.

When stdout is a TTY, a live progress bar is shown during repository processing. If an action or finding needs to be logged, the progress line is temporarily cleared, the log message is printed, and the progress bar is redrawn beneath it so the progress indicator remains at the bottom of the active output.

### Verbose Mode

Use `--verbose` to see additional per-repository processing detail, such as the total number of repositories found and filtered.

### Color Control

Output uses ANSI color codes to signal status categories when stdout is a TTY. Color improves readability but text labels are always present so output remains legible without color.

The live progress bar uses the same color settings and is only rendered as an updating line when stdout is a TTY. Non-TTY output remains line-oriented for scripting/log capture.

Color can be disabled by:

- Passing `--no-color`
- Setting the `NO_COLOR` environment variable (any value)

## Exit Codes

| Code | Meaning |
|---|---|
| `0` | Command completed successfully, including runs with audit findings (dirty repos, branch drift, unknown folders, missing config dotfile) |
| `1` | Command failed due to configuration error, authentication/API failure, or other operational error |

Audit findings are user-facing warnings, not command failures.

## Local Directory Classification

`ghorgsync` inspects immediate child entries of the current working directory (non-recursive) and classifies each one:

| Classification | Description |
|---|---|
| **Managed** | Corresponds to an included GitHub repository. Cloned if missing; synced/audited if present. |
| **Unknown** | A directory that does not match any repository (included or excluded) in the organization. |
| **Excluded-but-present** | A directory matching a repository excluded by name or pattern. Reported but not modified. |
| **Collision** | A managed repo path exists but is not a usable git clone (e.g., a regular file, non-git directory, or remote mismatch). Reported and skipped. |

Hidden entries (starting with `.`) are skipped during scanning.

## Branch Drift

A repository is in *branch drift* when its current branch differs from the default branch (as defined by GitHub metadata). Default branch names are per-repository and are never assumed.

- **Dirty repo with drift** — reported as informational; no automatic correction since checkout is unsafe.
- **Clean repo with drift** — the default branch is checked out and pulled; the correction is logged.

## Dirty Repository Reporting

When a repository has a dirty working tree, the output includes:

- Repository name
- Current branch and default branch
- Indication that checkout/pull was skipped
- List of changed file paths with staged/unstaged distinction
- Line-count summary (additions/deletions) when available

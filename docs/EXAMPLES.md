---
layout: default
title: Examples
nav_order: 4
permalink: /examples
---

# Examples
{: .no_toc }

## Table of Contents
{: .no_toc .text-delta }

- TOC
{:toc}

---

## Configuration Examples

### Basic Configuration

Sync all repositories (public and private) in an organization:

```yaml
organization: my-org
```

### Public Repositories Only

Exclude private repositories from syncing:

```yaml
organization: my-org
include_private: false
```

### With Exclusion Patterns

Exclude specific repos by exact name and regex patterns:

```yaml
organization: my-org
exclude_repos:
  - legacy-repo
  - "^sandbox-"
  - "-archive$"
  - "^test-.*-tmp$"
```

This configuration excludes:

- `legacy-repo` (exact match)
- Any repo starting with `sandbox-` (e.g., `sandbox-experiments`)
- Any repo ending with `-archive` (e.g., `old-service-archive`)
- Any repo matching `test-*-tmp` (e.g., `test-api-tmp`)

## Example Output

The examples below show the stable log lines and summary output. In an interactive terminal (TTY), **ghorgsync** also renders a live progress bar during repository processing; that transient line is omitted here for readability.

### Clean Run

When all repositories are already up to date and on their default branches, **ghorgsync** produces minimal output:

```
Summary:
  total: 5 | cloned: 0 | updated: 0 | dirty: 0 | branch-drift: 0 | unknown: 0 | excluded-but-present: 0 | errors: 0
```

### Cloned Repositories

When new repositories are found in the organization that don't exist locally:

```
  repo  my-app  [cloned]
  repo  api-service  [cloned]

Summary:
  total: 5 | cloned: 2 | updated: 0 | dirty: 0 | branch-drift: 0 | unknown: 0 | excluded-but-present: 0 | errors: 0
```

### Updated Repositories

When existing repositories have new commits on their default branch:

```
  repo  api-service  [updated]

Summary:
  total: 5 | cloned: 0 | updated: 1 | dirty: 0 | branch-drift: 0 | unknown: 0 | excluded-but-present: 0 | errors: 0
```

### Dirty Repository

When a repository has staged or unstaged changes, checkout and pull are skipped:

```
  repo  web-frontend  [dirty] on feature-branch (default: main)
       checkout/pull skipped due to dirty working tree
       [unstaged] src/index.ts
       [staged] package.json
       +15 -3 lines

Summary:
  total: 5 | cloned: 0 | updated: 0 | dirty: 1 | branch-drift: 0 | unknown: 0 | excluded-but-present: 0 | errors: 0
```

The output shows:

- The current branch and default branch
- Why checkout/pull was skipped
- Each changed file with its staged/unstaged status
- A summary of line additions and deletions

### Branch Drift Detected and Corrected

When a clean repository is not on its default branch, **ghorgsync** checks out the default branch and pulls:

```
  repo  docs-site  [branch-drift: checked out main, updated]

Summary:
  total: 5 | cloned: 0 | updated: 1 | dirty: 0 | branch-drift: 1 | unknown: 0 | excluded-but-present: 0 | errors: 0
```

If the repository was dirty and on the wrong branch, drift is reported but no correction is made:

```
  repo  docs-site  [dirty] on feature-docs (default: main)
       checkout/pull skipped due to dirty working tree
       [unstaged] docs/guide.md
       +8 -2 lines

Summary:
  total: 5 | cloned: 0 | updated: 0 | dirty: 1 | branch-drift: 0 | unknown: 0 | excluded-but-present: 0 | errors: 0
```

### Unknown Folder Warning

When a directory exists locally that doesn't correspond to any repository in the organization:

```
  folder  personal-project  [unknown]

Summary:
  total: 5 | cloned: 0 | updated: 0 | dirty: 0 | branch-drift: 0 | unknown: 1 | excluded-but-present: 0 | errors: 0
```

### Excluded-but-Present Warning

When a directory exists locally for a repository that is excluded by configuration:

```
  folder  old-tool  [excluded-but-present]

Summary:
  total: 5 | cloned: 0 | updated: 0 | dirty: 0 | branch-drift: 0 | unknown: 0 | excluded-but-present: 1 | errors: 0
```

### Combined Output

A typical run with multiple findings:

```
  repo  my-app  [cloned]
  repo  api-service  [updated]
  repo  web-frontend  [dirty] on feature-branch (default: main)
       checkout/pull skipped due to dirty working tree
       [unstaged] src/index.ts
       [staged] package.json
       +15 -3 lines
  repo  docs-site  [branch-drift: checked out main, updated]
  folder  personal-project  [unknown]
  folder  old-tool  [excluded-but-present]

Summary:
  total: 8 | cloned: 1 | updated: 1 | dirty: 1 | branch-drift: 1 | unknown: 1 | excluded-but-present: 1 | errors: 0
```

### Clone-Only Mode

Using `--clone` to quickly clone only missing repositories without processing existing ones:

```
$ ghorgsync --clone
  repo  new-service  [cloned]
  repo  new-library  [cloned]

Summary:
  total: 10 | cloned: 2 | updated: 0 | dirty: 0 | branch-drift: 0 | unknown: 0 | excluded-but-present: 0 | errors: 0
```

When all repositories are already cloned locally, `--clone` finishes quickly with no output:

```
$ ghorgsync --clone

Summary:
  total: 10 | cloned: 0 | updated: 0 | dirty: 0 | branch-drift: 0 | unknown: 0 | excluded-but-present: 0 | errors: 0
```

This mode skips all per-repository processing (fetch, dirty check, checkout, pull) and directory auditing (collisions, unknown folders, excluded-but-present), making it significantly faster when you only need to pull down new repositories.

## Troubleshooting

### Missing Dotfile

If you run **ghorgsync** in a directory without a `.ghorgsync` file:

```
No .ghorgsync configuration file found in the current directory.
```

The command exits with code `0`. Create a `.ghorgsync` file with at least the `organization` field to proceed.

### Authentication Errors

If no GitHub token is available and the `gh` CLI is not authenticated:

```
  system  auth  [error] failed to list repositories: 401 Unauthorized
```

Fix by setting a token or authenticating with `gh`:

```bash
export GITHUB_TOKEN=ghp_your_token_here
# or
gh auth login
```

### Configuration Errors

If the configuration file is invalid:

```
  system  config  [error] organization is required
```

```
  system  config  [error] invalid exclude pattern "[bad-regex": error parsing regexp: missing closing ]: `[bad-regex`
```

```
  system  config  [error] at least one of include_public or include_private must be true
```

These errors exit with code `1`. Fix the `.ghorgsync` file and re-run.

### Repositories with Submodules

Repositories that contain git submodules are handled automatically. On clone, submodules are initialized via `--recurse-submodules`. For existing repositories, `git submodule update --init --recursive` is run after every fetch so that uninitialized submodule directories do not appear as untracked files and cause a false dirty report.

If a submodule remote is unreachable, the repository is reported as a `submodule-error`:

```
  repo  my-app  [submodule-error] git submodule update: ...
```

Fix the underlying submodule remote issue (network access, SSH keys, token scope) and re-run.

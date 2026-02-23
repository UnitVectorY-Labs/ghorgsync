# ghorgsync

Clone and update all organization repositories in one folder, with clean-state and branch audits plus warnings for stray content.

## Why ghorgsync?

Working with many repositories across a GitHub organization means constantly cloning new repos, pulling updates, and keeping track of local state. **ghorgsync** automates this into a single command that keeps your local directory in sync with your organization's repositories—safely and non-destructively.

## Key Features

- **One command sync** — clones missing repos, fetches and pulls existing ones, all in one pass
- **Non-destructive** — never deletes directories, discards local changes, or runs destructive git commands
- **Dirty repo detection** — reports staged/unstaged changes with file details and line counts
- **Branch drift audit** — detects when a repo isn't on its default branch and corrects clean repos automatically
- **Stray content warnings** — identifies unknown folders and excluded-but-present repos in your directory
- **Quiet by default** — only prints actions taken and findings; verbose mode available for full detail
- **Colorized output** — structured, color-coded terminal output (honors `NO_COLOR`)

## Quick Start

1. **Install**

   ```bash
   go install github.com/UnitVectorY-Labs/ghorgsync@latest
   ```

2. **Configure** — create a `.ghorgsync` file in the directory where your repos live:

   ```yaml
   organization: my-org
   ```

3. **Authenticate** — set a GitHub token:

   ```bash
   export GITHUB_TOKEN=ghp_...
   ```

4. **Run**

   ```bash
   cd ~/repos
   ghorgsync
   ```

## Documentation

- [Installation](https://unitvectory-labs.github.io/ghorgsync/install) — binary downloads, `go install`, build from source
- [Usage](https://unitvectory-labs.github.io/ghorgsync/usage) — configuration, authentication, CLI flags, runtime behavior
- [Examples](https://unitvectory-labs.github.io/ghorgsync/examples) — sample configs and example output

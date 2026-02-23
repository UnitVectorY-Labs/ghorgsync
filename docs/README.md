---
layout: default
title: ghorgsync
nav_order: 1
permalink: /
---

# ghorgsync

Clone and update all organization repositories in one folder, with clean-state and branch audits plus warnings for stray content.

**ghorgsync** is a command-line tool that synchronizes a local directory with the repositories in a GitHub organization. It clones missing repos, fetches and pulls existing ones, and audits their state noting if their state is clean or dirty and if they are not on the default branch — all in a single command.

## Key Features

- **One command sync** — clones missing repos, fetches and pulls existing ones
- **Non-destructive** — never deletes directories, discards local changes, or runs destructive git commands
- **Dirty repo detection** — reports staged/unstaged changes with file details and line counts
- **Branch drift audit** — detects and corrects default branch drift on clean repos
- **Stray content warnings** — identifies unknown folders and excluded-but-present repos
- **Quiet by default** — only prints actions and findings; verbose mode for full detail
- **Colorized output** — structured, color-coded terminal output (honors `NO_COLOR`)

package sync

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/UnitVectorY-Labs/ghorgsync/internal/model"
)

// GitRunner executes git commands. Abstracted for testability.
type GitRunner interface {
	Clone(url, dest string) error
	Fetch(repoDir string) error
	SubmoduleUpdate(repoDir string) error
	CurrentBranch(repoDir string) (string, error)
	IsDirty(repoDir string) (bool, []model.DirtyFile, error)
	DiffStats(repoDir string) (int, int, error) // additions, deletions
	Checkout(repoDir, branch string) error
	PullFF(repoDir string) (bool, error) // returns true if changes were pulled
	RemoteURL(repoDir string) (string, error)
}

// ExecGitRunner runs real git commands.
type ExecGitRunner struct{}

func (g *ExecGitRunner) Clone(url, dest string) error {
	cmd := exec.Command("git", "clone", "--recurse-submodules", url, dest)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git clone: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

func (g *ExecGitRunner) Fetch(repoDir string) error {
	cmd := exec.Command("git", "-C", repoDir, "fetch", "--all", "--prune")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git fetch: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

func (g *ExecGitRunner) SubmoduleUpdate(repoDir string) error {
	cmd := exec.Command("git", "-C", repoDir, "submodule", "update", "--init", "--recursive")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git submodule update: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

func (g *ExecGitRunner) CurrentBranch(repoDir string) (string, error) {
	cmd := exec.Command("git", "-C", repoDir, "rev-parse", "--abbrev-ref", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git current branch: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func (g *ExecGitRunner) IsDirty(repoDir string) (bool, []model.DirtyFile, error) {
	// Use git status --porcelain to detect dirty state
	cmd := exec.Command("git", "-C", repoDir, "status", "--porcelain")
	out, err := cmd.Output()
	if err != nil {
		return false, nil, fmt.Errorf("git status: %w", err)
	}
	files := ParseGitStatus(string(out))
	if len(files) == 0 {
		return false, nil, nil
	}
	return true, files, nil
}

func (g *ExecGitRunner) DiffStats(repoDir string) (int, int, error) {
	// Staged changes
	cmd := exec.Command("git", "-C", repoDir, "diff", "--cached", "--numstat")
	out, err := cmd.Output()
	if err != nil {
		return 0, 0, nil // non-fatal
	}
	adds, dels := parseNumstat(string(out))

	// Unstaged changes
	cmd2 := exec.Command("git", "-C", repoDir, "diff", "--numstat")
	out2, err := cmd2.Output()
	if err != nil {
		return adds, dels, nil
	}
	a2, d2 := parseNumstat(string(out2))
	return adds + a2, dels + d2, nil
}

func parseNumstat(output string) (int, int) {
	var adds, dels int
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		// Binary files show "-" instead of numbers
		if fields[0] != "-" {
			if a, err := strconv.Atoi(fields[0]); err == nil {
				adds += a
			}
		}
		if fields[1] != "-" {
			if d, err := strconv.Atoi(fields[1]); err == nil {
				dels += d
			}
		}
	}
	return adds, dels
}

func (g *ExecGitRunner) Checkout(repoDir, branch string) error {
	cmd := exec.Command("git", "-C", repoDir, "checkout", branch)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git checkout %s: %s: %w", branch, strings.TrimSpace(string(out)), err)
	}
	return nil
}

func (g *ExecGitRunner) PullFF(repoDir string) (bool, error) {
	// Get current HEAD before pull
	headBefore := getHead(repoDir)

	cmd := exec.Command("git", "-C", repoDir, "pull", "--ff-only")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("git pull --ff-only: %s: %w", strings.TrimSpace(string(out)), err)
	}

	headAfter := getHead(repoDir)
	return headBefore != headAfter, nil
}

func getHead(repoDir string) string {
	cmd := exec.Command("git", "-C", repoDir, "rev-parse", "HEAD")
	out, _ := cmd.Output()
	return strings.TrimSpace(string(out))
}

func (g *ExecGitRunner) RemoteURL(repoDir string) (string, error) {
	cmd := exec.Command("git", "-C", repoDir, "remote", "get-url", "origin")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git remote get-url: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

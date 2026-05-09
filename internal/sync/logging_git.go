package sync

import (
	"fmt"
	"strings"

	"github.com/UnitVectorY-Labs/ghorgsync/internal/model"
)

// LoggingGitRunner decorates a GitRunner with verbose command diagnostics.
type LoggingGitRunner struct {
	next GitRunner
	logf func(string, ...interface{})
}

// NewLoggingGitRunner wraps a GitRunner and emits command diagnostics via logf.
func NewLoggingGitRunner(next GitRunner, logf func(string, ...interface{})) GitRunner {
	if logf == nil {
		return next
	}
	return &LoggingGitRunner{next: next, logf: logf}
}

func (g *LoggingGitRunner) Clone(url, dest string) error {
	cmd := fmt.Sprintf("git clone --recurse-submodules %s %s", url, dest)
	g.logf("git cmd: %s", cmd)
	err := g.next.Clone(url, dest)
	g.logResult(cmd, err)
	return err
}

func (g *LoggingGitRunner) Fetch(repoDir string) error {
	cmd := fmt.Sprintf("git -C %s fetch --all --prune", repoDir)
	g.logf("git cmd: %s", cmd)
	err := g.next.Fetch(repoDir)
	g.logResult(cmd, err)
	return err
}

func (g *LoggingGitRunner) SubmoduleUpdate(repoDir string) error {
	cmd := fmt.Sprintf("git -C %s submodule update --init --recursive", repoDir)
	g.logf("git cmd: %s", cmd)
	err := g.next.SubmoduleUpdate(repoDir)
	g.logResult(cmd, err)
	return err
}

func (g *LoggingGitRunner) CurrentBranch(repoDir string) (string, error) {
	cmd := fmt.Sprintf("git -C %s rev-parse --abbrev-ref HEAD", repoDir)
	g.logf("git cmd: %s", cmd)
	branch, err := g.next.CurrentBranch(repoDir)
	if err != nil {
		g.logResult(cmd, err)
		return "", err
	}
	g.logf("git result: %s -> branch=%q", cmd, branch)
	return branch, nil
}

func (g *LoggingGitRunner) IsDirty(repoDir string) (bool, []model.DirtyFile, error) {
	cmd := fmt.Sprintf("git -C %s status --porcelain", repoDir)
	g.logf("git cmd: %s", cmd)
	dirty, files, err := g.next.IsDirty(repoDir)
	if err != nil {
		g.logResult(cmd, err)
		return false, nil, err
	}
	g.logf("git result: %s -> dirty=%t files=%d", cmd, dirty, len(files))
	return dirty, files, nil
}

func (g *LoggingGitRunner) DiffStats(repoDir string) (int, int, error) {
	stagedCmd := fmt.Sprintf("git -C %s diff --cached --numstat", repoDir)
	unstagedCmd := fmt.Sprintf("git -C %s diff --numstat", repoDir)
	g.logf("git cmd: %s", stagedCmd)
	g.logf("git cmd: %s", unstagedCmd)
	additions, deletions, err := g.next.DiffStats(repoDir)
	if err != nil {
		g.logf("git result: diff stats -> error=%v", err)
		return 0, 0, err
	}
	g.logf("git result: diff stats -> additions=%d deletions=%d", additions, deletions)
	return additions, deletions, nil
}

func (g *LoggingGitRunner) Checkout(repoDir, branch string) error {
	cmd := fmt.Sprintf("git -C %s checkout %s", repoDir, branch)
	g.logf("git cmd: %s", cmd)
	err := g.next.Checkout(repoDir, branch)
	g.logResult(cmd, err)
	return err
}

func (g *LoggingGitRunner) PullFF(repoDir string) (bool, error) {
	cmd := fmt.Sprintf("git -C %s pull --ff-only", repoDir)
	g.logf("git cmd: %s", cmd)
	updated, err := g.next.PullFF(repoDir)
	if err != nil {
		g.logResult(cmd, err)
		return false, err
	}
	g.logf("git result: %s -> updated=%t", cmd, updated)
	return updated, nil
}

func (g *LoggingGitRunner) RemoteURL(repoDir string) (string, error) {
	cmd := fmt.Sprintf("git -C %s remote get-url origin", repoDir)
	g.logf("git cmd: %s", cmd)
	remote, err := g.next.RemoteURL(repoDir)
	if err != nil {
		g.logResult(cmd, err)
		return "", err
	}
	g.logf("git result: %s -> remote=%q", cmd, remote)
	return remote, nil
}

func (g *LoggingGitRunner) StatusShort(repoDir string) (string, error) {
	cmd := fmt.Sprintf("git -C %s -c color.status=always status --short", repoDir)
	g.logf("git cmd: %s", cmd)
	status, err := g.next.StatusShort(repoDir)
	if err != nil {
		g.logResult(cmd, err)
		return "", err
	}
	lines := 0
	if trimmed := strings.TrimSpace(status); trimmed != "" {
		lines = len(strings.Split(trimmed, "\n"))
	}
	g.logf("git result: %s -> lines=%d", cmd, lines)
	return status, nil
}

func (g *LoggingGitRunner) logResult(cmd string, err error) {
	if err != nil {
		g.logf("git result: %s -> error=%v", cmd, err)
		return
	}
	g.logf("git result: %s -> ok", cmd)
}

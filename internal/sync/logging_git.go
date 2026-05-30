package sync

import (
	"strings"

	"github.com/UnitVectorY-Labs/ghorgsync/internal/model"
)

// LoggingGitRunner decorates a GitRunner with verbose command diagnostics.
// Before each operation it emits "git cmd: <command>" and afterward
// "git exit: 0" on success or "git exit: 1 error=<message>" on failure,
// along with any structured result values.
type LoggingGitRunner struct {
	next GitRunner
	logf func(string, ...any)
}

// NewLoggingGitRunner wraps a GitRunner and emits command diagnostics via logf.
// If logf is nil the original runner is returned unwrapped.
func NewLoggingGitRunner(next GitRunner, logf func(string, ...any)) GitRunner {
	if logf == nil {
		return next
	}
	return &LoggingGitRunner{next: next, logf: logf}
}

func (g *LoggingGitRunner) Clone(url, dest string) error {
	g.logf("git cmd: git clone --recurse-submodules %s %s", url, dest)
	if err := g.next.Clone(url, dest); err != nil {
		g.logf("git exit: 1 error=%q", err.Error())
		return err
	}
	g.logf("git exit: 0")
	return nil
}

func (g *LoggingGitRunner) Fetch(repoDir string) error {
	g.logf("git cmd: git -C %s fetch --all --prune", repoDir)
	if err := g.next.Fetch(repoDir); err != nil {
		g.logf("git exit: 1 error=%q", err.Error())
		return err
	}
	g.logf("git exit: 0")
	return nil
}

func (g *LoggingGitRunner) SubmoduleUpdate(repoDir string) error {
	g.logf("git cmd: git -C %s submodule update --init --recursive", repoDir)
	if err := g.next.SubmoduleUpdate(repoDir); err != nil {
		g.logf("git exit: 1 error=%q", err.Error())
		return err
	}
	g.logf("git exit: 0")
	return nil
}

func (g *LoggingGitRunner) CurrentBranch(repoDir string) (string, error) {
	g.logf("git cmd: git -C %s rev-parse --abbrev-ref HEAD", repoDir)
	branch, err := g.next.CurrentBranch(repoDir)
	if err != nil {
		g.logf("git exit: 1 error=%q", err.Error())
		return "", err
	}
	g.logf("git exit: 0 branch=%q", branch)
	return branch, nil
}

func (g *LoggingGitRunner) IsDirty(repoDir string) (bool, []model.DirtyFile, error) {
	g.logf("git cmd: git -C %s status --porcelain", repoDir)
	dirty, files, err := g.next.IsDirty(repoDir)
	if err != nil {
		g.logf("git exit: 1 error=%q", err.Error())
		return false, nil, err
	}
	g.logf("git exit: 0 dirty=%t files=%d", dirty, len(files))
	return dirty, files, nil
}

func (g *LoggingGitRunner) DiffStats(repoDir string) (int, int, error) {
	g.logf("git cmd: git -C %s diff --cached --numstat", repoDir)
	g.logf("git cmd: git -C %s diff --numstat", repoDir)
	additions, deletions, err := g.next.DiffStats(repoDir)
	if err != nil {
		g.logf("git exit: 1 error=%q", err.Error())
		return 0, 0, err
	}
	g.logf("git exit: 0 additions=%d deletions=%d", additions, deletions)
	return additions, deletions, nil
}

func (g *LoggingGitRunner) Checkout(repoDir, branch string) error {
	g.logf("git cmd: git -C %s checkout %s", repoDir, branch)
	if err := g.next.Checkout(repoDir, branch); err != nil {
		g.logf("git exit: 1 error=%q", err.Error())
		return err
	}
	g.logf("git exit: 0")
	return nil
}

func (g *LoggingGitRunner) PullFF(repoDir string) (bool, error) {
	g.logf("git cmd: git -C %s pull --ff-only", repoDir)
	updated, err := g.next.PullFF(repoDir)
	if err != nil {
		g.logf("git exit: 1 error=%q", err.Error())
		return false, err
	}
	g.logf("git exit: 0 updated=%t", updated)
	return updated, nil
}

func (g *LoggingGitRunner) RemoteURL(repoDir string) (string, error) {
	g.logf("git cmd: git -C %s remote get-url origin", repoDir)
	remote, err := g.next.RemoteURL(repoDir)
	if err != nil {
		g.logf("git exit: 1 error=%q", err.Error())
		return "", err
	}
	g.logf("git exit: 0 remote=%q", remote)
	return remote, nil
}

func (g *LoggingGitRunner) StatusShort(repoDir string) (string, error) {
	g.logf("git cmd: git -C %s -c color.status=always status --short", repoDir)
	status, err := g.next.StatusShort(repoDir)
	if err != nil {
		g.logf("git exit: 1 error=%q", err.Error())
		return "", err
	}
	lines := 0
	if trimmed := strings.TrimSpace(status); trimmed != "" {
		lines = len(strings.Split(trimmed, "\n"))
	}
	g.logf("git exit: 0 lines=%d", lines)
	return status, nil
}

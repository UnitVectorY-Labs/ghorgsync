package sync

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/UnitVectorY-Labs/ghorgsync/internal/model"
)

type loggingMockGitRunner struct {
	cloneErr      error
	fetchErr      error
	currentBranch string
	currentErr    error
	dirty         bool
	dirtyFiles    []model.DirtyFile
	dirtyErr      error
	statusShort   string
	statusErr     error
}

func (m *loggingMockGitRunner) Clone(url, dest string) error         { return m.cloneErr }
func (m *loggingMockGitRunner) Fetch(repoDir string) error           { return m.fetchErr }
func (m *loggingMockGitRunner) SubmoduleUpdate(repoDir string) error { return nil }
func (m *loggingMockGitRunner) CurrentBranch(repoDir string) (string, error) {
	return m.currentBranch, m.currentErr
}
func (m *loggingMockGitRunner) IsDirty(repoDir string) (bool, []model.DirtyFile, error) {
	return m.dirty, m.dirtyFiles, m.dirtyErr
}
func (m *loggingMockGitRunner) DiffStats(repoDir string) (int, int, error) { return 2, 1, nil }
func (m *loggingMockGitRunner) Checkout(repoDir, branch string) error      { return nil }
func (m *loggingMockGitRunner) PullFF(repoDir string) (bool, error)        { return true, nil }
func (m *loggingMockGitRunner) RemoteURL(repoDir string) (string, error) {
	return "https://github.com/acme/repo.git", nil
}
func (m *loggingMockGitRunner) StatusShort(repoDir string) (string, error) {
	return m.statusShort, m.statusErr
}

func TestNewLoggingGitRunner_WithNilLoggerReturnsOriginalRunner(t *testing.T) {
	base := &loggingMockGitRunner{}
	wrapped := NewLoggingGitRunner(base, nil)
	if wrapped != base {
		t.Fatal("expected original runner when logger is nil")
	}
}

func TestLoggingGitRunner_LogsCommandAndResult(t *testing.T) {
	var logs []string
	runner := NewLoggingGitRunner(&loggingMockGitRunner{}, func(format string, args ...interface{}) {
		logs = append(logs, fmt.Sprintf(format, args...))
	})

	if err := runner.Fetch("/repos/demo"); err != nil {
		t.Fatalf("fetch failed: %v", err)
	}

	joined := strings.Join(logs, "\n")
	if !strings.Contains(joined, "git cmd: git -C /repos/demo fetch --all --prune") {
		t.Fatalf("expected fetch command in logs, got: %s", joined)
	}
	if !strings.Contains(joined, "git result: git -C /repos/demo fetch --all --prune -> ok") {
		t.Fatalf("expected fetch success in logs, got: %s", joined)
	}
}

func TestLoggingGitRunner_LogsErrors(t *testing.T) {
	var logs []string
	runner := NewLoggingGitRunner(&loggingMockGitRunner{cloneErr: errors.New("clone failed")}, func(format string, args ...interface{}) {
		logs = append(logs, fmt.Sprintf(format, args...))
	})

	err := runner.Clone("https://github.com/acme/repo.git", "/repos/repo")
	if err == nil {
		t.Fatal("expected clone error")
	}

	joined := strings.Join(logs, "\n")
	if !strings.Contains(joined, "git cmd: git clone --recurse-submodules https://github.com/acme/repo.git /repos/repo") {
		t.Fatalf("expected clone command in logs, got: %s", joined)
	}
	if !strings.Contains(joined, "error=clone failed") {
		t.Fatalf("expected clone error in logs, got: %s", joined)
	}
}

func TestLoggingGitRunner_LogsStructuredResults(t *testing.T) {
	var logs []string
	runner := NewLoggingGitRunner(&loggingMockGitRunner{
		currentBranch: "main",
		dirty:         true,
		dirtyFiles:    []model.DirtyFile{{Path: "main.go", Unstaged: true}},
		statusShort:   " M main.go\n?? new.txt\n",
	}, func(format string, args ...interface{}) {
		logs = append(logs, fmt.Sprintf(format, args...))
	})

	branch, err := runner.CurrentBranch("/repos/repo")
	if err != nil {
		t.Fatalf("current branch failed: %v", err)
	}
	if branch != "main" {
		t.Fatalf("unexpected branch: %s", branch)
	}

	dirty, files, err := runner.IsDirty("/repos/repo")
	if err != nil {
		t.Fatalf("is dirty failed: %v", err)
	}
	if !dirty || len(files) != 1 {
		t.Fatalf("unexpected dirty result: dirty=%t files=%d", dirty, len(files))
	}

	if _, err := runner.StatusShort("/repos/repo"); err != nil {
		t.Fatalf("status short failed: %v", err)
	}

	joined := strings.Join(logs, "\n")
	if !strings.Contains(joined, `branch="main"`) {
		t.Fatalf("expected branch result in logs, got: %s", joined)
	}
	if !strings.Contains(joined, "dirty=true files=1") {
		t.Fatalf("expected dirty summary in logs, got: %s", joined)
	}
	if !strings.Contains(joined, "lines=2") {
		t.Fatalf("expected status line count in logs, got: %s", joined)
	}
}

package sync

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/UnitVectorY-Labs/ghorgsync/internal/config"
	"github.com/UnitVectorY-Labs/ghorgsync/internal/model"
)

// mockGitRunner is a test double for GitRunner.
type mockGitRunner struct {
	currentBranch  string
	branchErr      error
	dirty          bool
	dirtyFiles     []model.DirtyFile
	dirtyErr       error
	statusOutput   string
	statusErr      error
	pullChanged    bool
	pullErr        error
	checkoutErr    error
	checkoutBranch string
}

func (m *mockGitRunner) Clone(url, dest string) error         { return nil }
func (m *mockGitRunner) Fetch(repoDir string) error           { return nil }
func (m *mockGitRunner) SubmoduleUpdate(repoDir string) error { return nil }
func (m *mockGitRunner) Checkout(repoDir, branch string) error {
	m.checkoutBranch = branch
	return m.checkoutErr
}
func (m *mockGitRunner) PullFF(repoDir string) (bool, error)        { return m.pullChanged, m.pullErr }
func (m *mockGitRunner) RemoteURL(repoDir string) (string, error)   { return "", nil }
func (m *mockGitRunner) DiffStats(repoDir string) (int, int, error) { return 0, 0, nil }
func (m *mockGitRunner) CurrentBranch(repoDir string) (string, error) {
	return m.currentBranch, m.branchErr
}
func (m *mockGitRunner) IsDirty(repoDir string) (bool, []model.DirtyFile, error) {
	return m.dirty, m.dirtyFiles, m.dirtyErr
}
func (m *mockGitRunner) StatusShort(repoDir string) (string, error) {
	return m.statusOutput, m.statusErr
}

func TestStatusRepo_CleanOnDefaultBranch(t *testing.T) {
	eng := &Engine{
		Git:     &mockGitRunner{currentBranch: "main"},
		BaseDir: "/tmp",
	}
	repo := model.RepoInfo{Name: "test-repo", DefaultBranch: "main"}

	result := eng.StatusRepo(repo)

	if result.Action != model.ActionAlreadyCurrent {
		t.Errorf("expected ActionAlreadyCurrent, got %v", result.Action)
	}
}

func TestStatusRepo_DirtyRepo(t *testing.T) {
	eng := &Engine{
		Git: &mockGitRunner{
			currentBranch: "main",
			dirty:         true,
			dirtyFiles:    []model.DirtyFile{{Path: "file.go", Unstaged: true}},
			statusOutput:  " M file.go\n",
		},
		BaseDir: "/tmp",
	}
	repo := model.RepoInfo{Name: "test-repo", DefaultBranch: "main"}

	result := eng.StatusRepo(repo)

	if result.Action != model.ActionDirty {
		t.Errorf("expected ActionDirty, got %v", result.Action)
	}
	if result.StatusOutput != " M file.go\n" {
		t.Errorf("expected status output, got %q", result.StatusOutput)
	}
	if len(result.DirtyFiles) != 1 {
		t.Errorf("expected 1 dirty file, got %d", len(result.DirtyFiles))
	}
}

func TestStatusRepo_BranchDrift(t *testing.T) {
	eng := &Engine{
		Git:     &mockGitRunner{currentBranch: "feature"},
		BaseDir: "/tmp",
	}
	repo := model.RepoInfo{Name: "test-repo", DefaultBranch: "main"}

	result := eng.StatusRepo(repo)

	if result.Action != model.ActionBranchDrift {
		t.Errorf("expected ActionBranchDrift, got %v", result.Action)
	}
	if result.CurrentBranch != "feature" {
		t.Errorf("expected current branch 'feature', got %q", result.CurrentBranch)
	}
}

func TestStatusRepo_DirtyWithBranchDrift(t *testing.T) {
	eng := &Engine{
		Git: &mockGitRunner{
			currentBranch: "feature",
			dirty:         true,
			dirtyFiles:    []model.DirtyFile{{Path: "file.go", Staged: true}},
			statusOutput:  "M  file.go\n",
		},
		BaseDir: "/tmp",
	}
	repo := model.RepoInfo{Name: "test-repo", DefaultBranch: "main"}

	result := eng.StatusRepo(repo)

	// Dirty takes precedence over branch drift
	if result.Action != model.ActionDirty {
		t.Errorf("expected ActionDirty, got %v", result.Action)
	}
	if !result.BranchDrift {
		t.Error("expected BranchDrift to be true")
	}
}

func TestStatusRepo_BranchError(t *testing.T) {
	eng := &Engine{
		Git:     &mockGitRunner{branchErr: errors.New("not a git repo")},
		BaseDir: "/tmp",
	}
	repo := model.RepoInfo{Name: "test-repo", DefaultBranch: "main"}

	result := eng.StatusRepo(repo)

	if result.Action != model.ActionFetchError {
		t.Errorf("expected ActionFetchError, got %v", result.Action)
	}
	if result.Error == nil {
		t.Error("expected error to be set")
	}
}

func TestStatusRepo_DirtyCheckError(t *testing.T) {
	eng := &Engine{
		Git: &mockGitRunner{
			currentBranch: "main",
			dirtyErr:      errors.New("status failed"),
		},
		BaseDir: "/tmp",
	}
	repo := model.RepoInfo{Name: "test-repo", DefaultBranch: "main"}

	result := eng.StatusRepo(repo)

	if result.Action != model.ActionFetchError {
		t.Errorf("expected ActionFetchError, got %v", result.Action)
	}
}

func TestStatusRepo_UsesBranchHintWhenAvailable(t *testing.T) {
	baseDir := t.TempDir()
	repoDir := filepath.Join(baseDir, "test-repo")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatalf("creating repo dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, ".repo-metadata.yaml"), []byte("defaults:\n  branch: release\n"), 0644); err != nil {
		t.Fatalf("writing branch hint: %v", err)
	}

	eng := &Engine{
		Git:     &mockGitRunner{currentBranch: "release"},
		BaseDir: baseDir,
		BranchHint: &config.BranchHint{
			Path:     ".repo-metadata.yaml",
			Type:     "yaml",
			JSONPath: "defaults.branch",
		},
	}
	repo := model.RepoInfo{Name: "test-repo", DefaultBranch: "main"}

	result := eng.StatusRepo(repo)

	if result.DefaultBranch != "release" {
		t.Errorf("DefaultBranch = %q, want %q", result.DefaultBranch, "release")
	}
	if result.Action != model.ActionAlreadyCurrent {
		t.Errorf("expected ActionAlreadyCurrent, got %v", result.Action)
	}
}

func TestProcessRepo_FallsBackWhenBranchHintMissing(t *testing.T) {
	baseDir := t.TempDir()
	repoDir := filepath.Join(baseDir, "test-repo")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatalf("creating repo dir: %v", err)
	}

	git := &mockGitRunner{currentBranch: "feature"}
	eng := &Engine{
		Git:     git,
		BaseDir: baseDir,
		BranchHint: &config.BranchHint{
			Path:     ".repo-metadata.yaml",
			Type:     "yaml",
			JSONPath: "defaults.branch",
		},
	}
	repo := model.RepoInfo{Name: "test-repo", DefaultBranch: "main"}

	result := eng.ProcessRepo(repo)

	if result.DefaultBranch != "main" {
		t.Errorf("DefaultBranch = %q, want %q", result.DefaultBranch, "main")
	}
	if git.checkoutBranch != "main" {
		t.Errorf("checkout branch = %q, want %q", git.checkoutBranch, "main")
	}
	if result.Action != model.ActionBranchDrift {
		t.Errorf("expected ActionBranchDrift, got %v", result.Action)
	}
}

func TestProcessRepo_UsesBranchHintForCheckout(t *testing.T) {
	baseDir := t.TempDir()
	repoDir := filepath.Join(baseDir, "test-repo")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatalf("creating repo dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "config.json"), []byte(`{"default":{"branch":"develop"}}`), 0644); err != nil {
		t.Fatalf("writing branch hint: %v", err)
	}

	git := &mockGitRunner{currentBranch: "feature"}
	eng := &Engine{
		Git:     git,
		BaseDir: baseDir,
		BranchHint: &config.BranchHint{
			Path:     "config.json",
			Type:     "json",
			JSONPath: "default.branch",
		},
	}
	repo := model.RepoInfo{Name: "test-repo", DefaultBranch: "main"}

	result := eng.ProcessRepo(repo)

	if result.DefaultBranch != "develop" {
		t.Errorf("DefaultBranch = %q, want %q", result.DefaultBranch, "develop")
	}
	if git.checkoutBranch != "develop" {
		t.Errorf("checkout branch = %q, want %q", git.checkoutBranch, "develop")
	}
	if result.Action != model.ActionBranchDrift {
		t.Errorf("expected ActionBranchDrift, got %v", result.Action)
	}
}

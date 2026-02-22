package scanner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/UnitVectorY-Labs/ghorgsync/internal/config"
	"github.com/UnitVectorY-Labs/ghorgsync/internal/model"
)

// makeDotGit creates a .git subdirectory inside dir/repoName to simulate a valid git repo.
func makeDotGit(t *testing.T, dir, repoName string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, repoName, ".git"), 0o755); err != nil {
		t.Fatalf("failed to create .git dir: %v", err)
	}
}

// TestScanDirectory_DotPrefixRepoManaged verifies that a repo whose name starts with a dot
// (e.g. ".github") is recognised as managed when it exists locally as a valid git clone,
// and is NOT added to ManagedMissing (which would trigger an erroneous clone attempt).
func TestScanDirectory_DotPrefixRepoManaged(t *testing.T) {
	dir := t.TempDir()
	makeDotGit(t, dir, ".github")

	repos := []model.RepoInfo{{Name: ".github"}}
	cfg := &config.Config{}

	result, err := ScanDirectory(dir, repos, nil, cfg)
	if err != nil {
		t.Fatalf("ScanDirectory returned error: %v", err)
	}

	if len(result.ManagedMissing) != 0 {
		t.Errorf("expected ManagedMissing to be empty, got %v", result.ManagedMissing)
	}
	if len(result.ManagedFound) != 1 || result.ManagedFound[0] != ".github" {
		t.Errorf("expected ManagedFound=[.github], got %v", result.ManagedFound)
	}
}

// TestScanDirectory_DotPrefixRepoMissing verifies that a repo whose name starts with a dot
// is correctly added to ManagedMissing when it does not yet exist locally.
func TestScanDirectory_DotPrefixRepoMissing(t *testing.T) {
	dir := t.TempDir()
	// deliberately do NOT create .github directory

	repos := []model.RepoInfo{{Name: ".github"}}
	cfg := &config.Config{}

	result, err := ScanDirectory(dir, repos, nil, cfg)
	if err != nil {
		t.Fatalf("ScanDirectory returned error: %v", err)
	}

	if len(result.ManagedFound) != 0 {
		t.Errorf("expected ManagedFound to be empty, got %v", result.ManagedFound)
	}
	if len(result.ManagedMissing) != 1 || result.ManagedMissing[0] != ".github" {
		t.Errorf("expected ManagedMissing=[.github], got %v", result.ManagedMissing)
	}
}

// TestScanDirectory_NonManagedDotDirIgnored verifies that hidden directories that are NOT
// managed repos (e.g. ".git", ".DS_Store") are still ignored and do not appear as Unknown.
func TestScanDirectory_NonManagedDotDirIgnored(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatalf("failed to create .git dir: %v", err)
	}

	cfg := &config.Config{}

	result, err := ScanDirectory(dir, nil, nil, cfg)
	if err != nil {
		t.Fatalf("ScanDirectory returned error: %v", err)
	}

	if len(result.Unknown) != 0 {
		t.Errorf("expected Unknown to be empty (hidden non-managed dirs ignored), got %v", result.Unknown)
	}
}

package scanner

import (
	"testing"

	"github.com/UnitVectorY-Labs/ghorgsync/internal/model"
)

func TestClassifyEntry_ManagedDirWithGit(t *testing.T) {
	included := map[string]bool{"my-repo": true}
	isExcluded := func(string) bool { return false }

	entry := ClassifyEntry("my-repo", true, true, included, isExcluded)

	if entry.Classification != model.ClassManaged {
		t.Errorf("expected ClassManaged, got %v", entry.Classification)
	}
	if entry.Name != "my-repo" {
		t.Errorf("expected name my-repo, got %s", entry.Name)
	}
	if entry.Detail != "" {
		t.Errorf("expected empty detail, got %q", entry.Detail)
	}
}

func TestClassifyEntry_ManagedDirWithoutGit(t *testing.T) {
	included := map[string]bool{"my-repo": true}
	isExcluded := func(string) bool { return false }

	entry := ClassifyEntry("my-repo", true, false, included, isExcluded)

	if entry.Classification != model.ClassCollision {
		t.Errorf("expected ClassCollision, got %v", entry.Classification)
	}
	if entry.Detail != "directory exists but is not a git repository" {
		t.Errorf("unexpected detail: %q", entry.Detail)
	}
}

func TestClassifyEntry_FileMatchingManagedName(t *testing.T) {
	included := map[string]bool{"my-repo": true}
	isExcluded := func(string) bool { return false }

	entry := ClassifyEntry("my-repo", false, false, included, isExcluded)

	if entry.Classification != model.ClassCollision {
		t.Errorf("expected ClassCollision, got %v", entry.Classification)
	}
	if entry.Detail != "path is a file, not a directory" {
		t.Errorf("unexpected detail: %q", entry.Detail)
	}
}

func TestClassifyEntry_ExcludedDir(t *testing.T) {
	included := map[string]bool{}
	isExcluded := func(name string) bool { return name == "old-repo" }

	entry := ClassifyEntry("old-repo", true, true, included, isExcluded)

	if entry.Classification != model.ClassExcludedButPresent {
		t.Errorf("expected ClassExcludedButPresent, got %v", entry.Classification)
	}
	if entry.Name != "old-repo" {
		t.Errorf("expected name old-repo, got %s", entry.Name)
	}
}

func TestClassifyEntry_UnknownDir(t *testing.T) {
	included := map[string]bool{}
	isExcluded := func(string) bool { return false }

	entry := ClassifyEntry("random-dir", true, false, included, isExcluded)

	if entry.Classification != model.ClassUnknown {
		t.Errorf("expected ClassUnknown, got %v", entry.Classification)
	}
	if entry.Name != "random-dir" {
		t.Errorf("expected name random-dir, got %s", entry.Name)
	}
}

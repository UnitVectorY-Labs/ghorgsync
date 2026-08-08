package cleanup

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPlanMeasuresAndRemovesIgnoredContent(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "cache", "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "cache", "nested", "build.bin"), []byte("1234"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "debug.log"), []byte("123"), 0o644); err != nil {
		t.Fatal(err)
	}

	targets, err := Plan(root, []string{"cache/", "cache/nested/build.bin", "debug.log"})
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	if len(targets) != 2 {
		t.Fatalf("Plan() targets = %d, want 2", len(targets))
	}
	if got := TotalSize(targets); got != 7 {
		t.Errorf("TotalSize() = %d, want 7", got)
	}
	files, dirs := Counts(targets)
	if files != 2 || dirs != 2 {
		t.Errorf("Counts() = %d files, %d dirs; want 2 files, 2 dirs", files, dirs)
	}
	for _, target := range targets {
		if err := Remove(root, target); err != nil {
			t.Fatalf("Remove(%q) error = %v", target.Path, err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "cache")); !os.IsNotExist(err) {
		t.Error("cache directory was not removed")
	}
}

func TestPlanRejectsPathOutsideRepository(t *testing.T) {
	if _, err := Plan(t.TempDir(), []string{"../outside"}); err == nil {
		t.Fatal("Plan() accepted a path outside the repository")
	}
}

func TestPruneEmptyParentsLeavesNonEmptyDirectories(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "cache", "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "cache", "nested", "ignored"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "cache", "keep"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	target := Target{Path: filepath.Join("cache", "nested", "ignored")}
	if err := Remove(root, target); err != nil {
		t.Fatal(err)
	}
	removed, err := PruneEmptyParents(root, target)
	if err != nil {
		t.Fatal(err)
	}
	if len(removed) != 1 || removed[0] != filepath.Join("cache", "nested") {
		t.Fatalf("removed = %v, want [cache/nested]", removed)
	}
	if _, err := os.Stat(filepath.Join(root, "cache")); err != nil {
		t.Error("non-empty cache directory was removed")
	}
}

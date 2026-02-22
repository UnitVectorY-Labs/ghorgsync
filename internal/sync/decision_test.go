package sync

import (
	"testing"
)

func TestDecideActions_DirtyRepo(t *testing.T) {
	d := DecideActions(true, "feature", "main")
	if !d.ShouldFetch {
		t.Error("should always fetch")
	}
	if d.ShouldCheckout {
		t.Error("should not checkout dirty repo")
	}
	if d.ShouldPull {
		t.Error("should not pull dirty repo")
	}
	if d.SkipReason == "" {
		t.Error("should have skip reason")
	}
}

func TestDecideActions_CleanOnDefaultBranch(t *testing.T) {
	d := DecideActions(false, "main", "main")
	if !d.ShouldFetch {
		t.Error("should fetch")
	}
	if d.ShouldCheckout {
		t.Error("should not checkout when already on default branch")
	}
	if !d.ShouldPull {
		t.Error("should pull clean repo")
	}
}

func TestDecideActions_CleanOnWrongBranch(t *testing.T) {
	d := DecideActions(false, "feature", "main")
	if !d.ShouldFetch {
		t.Error("should fetch")
	}
	if !d.ShouldCheckout {
		t.Error("should checkout when on wrong branch")
	}
	if !d.ShouldPull {
		t.Error("should pull after checkout")
	}
}

func TestDecideActions_DirtyOnDefaultBranch(t *testing.T) {
	d := DecideActions(true, "main", "main")
	if !d.ShouldFetch {
		t.Error("should always fetch")
	}
	if d.ShouldCheckout {
		t.Error("should not checkout dirty repo even on default branch")
	}
	if d.ShouldPull {
		t.Error("should not pull dirty repo")
	}
}

func TestParseGitStatus_Empty(t *testing.T) {
	files := ParseGitStatus("")
	if len(files) != 0 {
		t.Errorf("expected 0 files, got %d", len(files))
	}
}

func TestParseGitStatus_StagedFile(t *testing.T) {
	// "M  file.go" means staged modification
	files := ParseGitStatus("M  file.go\n")
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if !files[0].Staged {
		t.Error("expected staged")
	}
	if files[0].Unstaged {
		t.Error("expected not unstaged")
	}
	if files[0].Path != "file.go" {
		t.Errorf("expected file.go, got %s", files[0].Path)
	}
}

func TestParseGitStatus_UnstagedFile(t *testing.T) {
	// " M file.go" means unstaged modification
	files := ParseGitStatus(" M file.go\n")
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if files[0].Staged {
		t.Error("expected not staged")
	}
	if !files[0].Unstaged {
		t.Error("expected unstaged")
	}
}

func TestParseGitStatus_UntrackedFile(t *testing.T) {
	// "?? new.txt" means untracked
	files := ParseGitStatus("?? new.txt\n")
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if files[0].Staged {
		t.Error("untracked should not be staged")
	}
	if !files[0].Unstaged {
		t.Error("untracked should be unstaged")
	}
}

func TestParseGitStatus_MixedFiles(t *testing.T) {
	input := "M  staged.go\n M unstaged.go\n?? new.txt\n"
	files := ParseGitStatus(input)
	if len(files) != 3 {
		t.Fatalf("expected 3 files, got %d", len(files))
	}
}

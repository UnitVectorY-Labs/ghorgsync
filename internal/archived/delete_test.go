package archived

import (
	"fmt"
	"github.com/UnitVectorY-Labs/ghorgsync/internal/config"
	"github.com/UnitVectorY-Labs/ghorgsync/internal/model"
	"strings"
	"testing"
)

func TestPath(t *testing.T) {
	for _, name := range []string{"", ".", "..", ".hidden", "../repo", "a/b", `a\b`, "/repo"} {
		if _, err := Path("/sync", name); err == nil {
			t.Errorf("accepted unsafe name %q", name)
		}
	}
	if got, err := Path("/sync", "my-repo"); err != nil || got != "/sync/my-repo" {
		t.Fatalf("Path = %q, %v", got, err)
	}
}

func TestMatchRemote(t *testing.T) {
	expected := "https://github.com/Owner/Repo.git"
	for _, remote := range []string{expected, "git@github.com:owner/repo.git", "ssh://git@github.com/OWNER/REPO", "https://github.com/owner/repo"} {
		if !MatchRemote(remote, expected) {
			t.Errorf("rejected matching remote %q", remote)
		}
	}
	for _, remote := range []string{"", "/local/owner/repo", "https://github.com.evil/Owner/Repo.git", "https://github.com/other/Repo.git", "https://github.com/Owner/Repo.git?token=abc", "https://user@github.com/Owner/Repo.git", "https://github.com/Owner/Repo/extra", "git@evil:Owner/Repo.git"} {
		if MatchRemote(remote, expected) {
			t.Errorf("accepted mismatched remote %q", remote)
		}
	}
	if MatchRemote("", "") {
		t.Fatal("empty URLs must not match")
	}
}

func TestValidateContent(t *testing.T) {
	for _, status := range []string{"M  tracked", " M tracked", "?? new", "!! ignored", "D  deleted", "UU conflict"} {
		if validateContent(status, "", "") == nil {
			t.Errorf("accepted local content %q", status)
		}
	}
	for _, flags := range []string{"S sparse", "h hidden", "s sparse", "H normal\nh hidden"} {
		if validateContent("", "", flags) == nil {
			t.Errorf("accepted hidden index content %q", flags)
		}
	}
	if validateContent("", "100644 abc 0\tfile\n160000 def 0\tmodule", "") == nil {
		t.Fatal("accepted submodule")
	}
	if err := validateContent("", "100644 abc 0\tfile", "H file"); err != nil {
		t.Fatalf("rejected ordinary clean index: %v", err)
	}
}

func TestCandidate(t *testing.T) {
	yes, no := true, false
	tests := []struct {
		name string
		repo model.RepoInfo
		cfg  config.Config
		want bool
	}{
		{"archived public", model.RepoInfo{IsArchived: true}, config.Config{}, true},
		{"archived private", model.RepoInfo{IsArchived: true, IsPrivate: true}, config.Config{}, true},
		{"active", model.RepoInfo{}, config.Config{}, false},
		{"empty", model.RepoInfo{IsArchived: true, IsEmpty: true}, config.Config{}, false},
		{"included archives", model.RepoInfo{IsArchived: true}, config.Config{IncludeArchived: &yes}, false},
		{"public filtered", model.RepoInfo{IsArchived: true}, config.Config{IncludePublic: &no, ExcludeRepos: []string{".*"}}, false},
		{"private filtered", model.RepoInfo{IsArchived: true, IsPrivate: true}, config.Config{IncludePrivate: &no}, false},
		{"also name excluded", model.RepoInfo{IsArchived: true}, config.Config{ExcludeRepos: []string{".*"}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Candidate(tt.repo, &tt.cfg); got != tt.want {
				t.Fatalf("Candidate = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestProtectedContentReasons(t *testing.T) {
	for _, tt := range []struct{ status, want string }{
		{"!! .DS_Store\n!! target/output.jar", "2 ignored entries (.DS_Store, target/output.jar)"},
		{" M tracked\n!! ignored", "1 changed or untracked entries"},
		{"?? new", "1 changed or untracked entries"},
	} {
		err := validateContent(tt.status, "", "")
		if !IsRetained(err) || !strings.Contains(err.Error(), tt.want) {
			t.Fatalf("reason = %v, want %s", err, tt.want)
		}
	}
	if IsRetained(fmt.Errorf("command failed")) || IsRetained(nil) {
		t.Fatal("operational errors must not be safety findings")
	}
	if !IsRetained(fmt.Errorf("wrapped: %w", &RetainedError{Reason: "protected"})) {
		t.Fatal("wrapped finding was lost")
	}
}

package github

import (
	"encoding/json"
	"testing"

	"github.com/UnitVectorY-Labs/ghorgsync/internal/config"
	"github.com/UnitVectorY-Labs/ghorgsync/internal/model"
)

func TestRepositoryVisibilityFiltering(t *testing.T) {
	tests := []struct {
		name      string
		metadata  string
		nonPublic bool
	}{
		{"public", `{"visibility":"public","private":false}`, false},
		{"private", `{"visibility":"private","private":true}`, true},
		{"internal", `{"visibility":"internal","private":false}`, true},
		{"internal-private-flag", `{"visibility":"internal","private":true}`, true},
		{"private-visibility", `{"visibility":"private","private":false}`, true},
		{"legacy-private", `{"private":true}`, true},
		{"legacy-public", `{"private":false}`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var raw ghRepo
			if err := json.Unmarshal([]byte(tt.metadata), &raw); err != nil {
				t.Fatal(err)
			}
			raw.Name = tt.name
			raw.CloneURL = "https://github.com/org/" + tt.name + ".git"
			raw.DefaultBranch = "develop"
			repo := raw.repoInfo()
			if repo.IsPrivate != tt.nonPublic {
				t.Fatalf("IsPrivate = %v, want %v", repo.IsPrivate, tt.nonPublic)
			}
			if repo.Name != raw.Name || repo.CloneURL != raw.CloneURL || repo.DefaultBranch != raw.DefaultBranch {
				t.Fatalf("repository metadata lost: %+v", repo)
			}
			for _, mode := range []struct {
				name string
				cfg  config.Config
				want bool
			}{
				{"default", config.Config{}, true},
				{"public-only", config.Config{IncludePrivate: new(false)}, !tt.nonPublic},
				{"non-public-only", config.Config{IncludePublic: new(false)}, tt.nonPublic},
			} {
				t.Run(mode.name, func(t *testing.T) {
					included, _, _ := FilterRepos([]model.RepoInfo{repo}, &mode.cfg)
					if got := len(included) == 1; got != mode.want {
						t.Errorf("included = %v, want %v", got, mode.want)
					}
				})
			}
		})
	}
}

func TestInternalRepositoryOtherFilters(t *testing.T) {
	raw := ghRepo{Name: "internal-repo", Visibility: "internal", Archived: true}
	repo := raw.repoInfo()
	included, excluded, _ := FilterRepos([]model.RepoInfo{repo}, &config.Config{})
	if len(included) != 0 || len(excluded) != 1 || excluded[0] != repo.Name {
		t.Fatalf("archived internal repository should be excluded: %v, %v", included, excluded)
	}
	repo.IsArchived = false
	included, excluded, _ = FilterRepos([]model.RepoInfo{repo}, &config.Config{ExcludeRepos: []string{"^internal-"}})
	if len(included) != 0 || len(excluded) != 1 || excluded[0] != repo.Name {
		t.Fatalf("internal repository should match exclusion patterns: %v, %v", included, excluded)
	}
	repo.IsEmpty = true
	included, _, empty := FilterRepos([]model.RepoInfo{repo}, &config.Config{})
	if len(included) != 0 || len(empty) != 1 || empty[0] != repo.Name {
		t.Fatalf("empty internal repository should be skipped: %v, %v", included, empty)
	}
}

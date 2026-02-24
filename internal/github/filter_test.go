package github

import (
	"testing"

	"github.com/UnitVectorY-Labs/ghorgsync/internal/config"
	"github.com/UnitVectorY-Labs/ghorgsync/internal/model"
)

func boolPtr(b bool) *bool { return &b }

func sampleRepos() []model.RepoInfo {
	return []model.RepoInfo{
		{Name: "public-repo", CloneURL: "https://github.com/org/public-repo.git", DefaultBranch: "main", IsPrivate: false},
		{Name: "private-repo", CloneURL: "https://github.com/org/private-repo.git", DefaultBranch: "main", IsPrivate: true},
		{Name: "another-public", CloneURL: "https://github.com/org/another-public.git", DefaultBranch: "develop", IsPrivate: false},
		{Name: "secret-project", CloneURL: "https://github.com/org/secret-project.git", DefaultBranch: "main", IsPrivate: true},
	}
}

func sampleReposWithArchived() []model.RepoInfo {
	return []model.RepoInfo{
		{Name: "public-repo", CloneURL: "https://github.com/org/public-repo.git", DefaultBranch: "main", IsPrivate: false},
		{Name: "private-repo", CloneURL: "https://github.com/org/private-repo.git", DefaultBranch: "main", IsPrivate: true},
		{Name: "archived-public", CloneURL: "https://github.com/org/archived-public.git", DefaultBranch: "main", IsPrivate: false, IsArchived: true},
		{Name: "archived-private", CloneURL: "https://github.com/org/archived-private.git", DefaultBranch: "main", IsPrivate: true, IsArchived: true},
	}
}

func TestFilterRepos_DefaultConfig(t *testing.T) {
	cfg := &config.Config{Organization: "org"}
	included, excluded := FilterRepos(sampleRepos(), cfg)
	if len(included) != 4 {
		t.Errorf("expected 4 included repos, got %d", len(included))
	}
	if len(excluded) != 0 {
		t.Errorf("expected 0 excluded repos, got %d", len(excluded))
	}
}

func TestFilterRepos_ExcludePublic(t *testing.T) {
	cfg := &config.Config{
		Organization:   "org",
		IncludePublic:  boolPtr(false),
		IncludePrivate: boolPtr(true),
	}
	included, _ := FilterRepos(sampleRepos(), cfg)
	for _, r := range included {
		if !r.IsPrivate {
			t.Errorf("public repo %q should have been filtered out", r.Name)
		}
	}
	if len(included) != 2 {
		t.Errorf("expected 2 private repos, got %d", len(included))
	}
}

func TestFilterRepos_ExcludePrivate(t *testing.T) {
	cfg := &config.Config{
		Organization:   "org",
		IncludePublic:  boolPtr(true),
		IncludePrivate: boolPtr(false),
	}
	included, _ := FilterRepos(sampleRepos(), cfg)
	for _, r := range included {
		if r.IsPrivate {
			t.Errorf("private repo %q should have been filtered out", r.Name)
		}
	}
	if len(included) != 2 {
		t.Errorf("expected 2 public repos, got %d", len(included))
	}
}

func TestFilterRepos_ExcludeByExactName(t *testing.T) {
	cfg := &config.Config{
		Organization: "org",
		ExcludeRepos: []string{"secret-project"},
	}
	included, excluded := FilterRepos(sampleRepos(), cfg)
	if len(included) != 3 {
		t.Errorf("expected 3 included repos, got %d", len(included))
	}
	if len(excluded) != 1 || excluded[0] != "secret-project" {
		t.Errorf("expected excluded=[secret-project], got %v", excluded)
	}
}

func TestFilterRepos_ExcludeByRegex(t *testing.T) {
	cfg := &config.Config{
		Organization: "org",
		ExcludeRepos: []string{"^public-.*"},
	}
	included, excluded := FilterRepos(sampleRepos(), cfg)
	if len(excluded) != 1 || excluded[0] != "public-repo" {
		t.Errorf("expected excluded=[public-repo], got %v", excluded)
	}
	if len(included) != 3 {
		t.Errorf("expected 3 included repos, got %d", len(included))
	}
}

func TestFilterRepos_ExcludedListTracked(t *testing.T) {
	cfg := &config.Config{
		Organization: "org",
		ExcludeRepos: []string{"public-repo", "secret-project"},
	}
	included, excluded := FilterRepos(sampleRepos(), cfg)
	if len(excluded) != 2 {
		t.Errorf("expected 2 excluded repos, got %d", len(excluded))
	}
	if len(included) != 2 {
		t.Errorf("expected 2 included repos, got %d", len(included))
	}
	expectedExcluded := map[string]bool{"public-repo": true, "secret-project": true}
	for _, name := range excluded {
		if !expectedExcluded[name] {
			t.Errorf("unexpected excluded repo: %q", name)
		}
	}
}

func TestFilterRepos_ArchivedExcludedByDefault(t *testing.T) {
cfg := &config.Config{Organization: "org"}
included, excluded := FilterRepos(sampleReposWithArchived(), cfg)
if len(included) != 2 {
t.Errorf("expected 2 included repos (non-archived), got %d", len(included))
}
if len(excluded) != 2 {
t.Errorf("expected 2 excluded repos (archived), got %d", len(excluded))
}
for _, r := range included {
if r.IsArchived {
t.Errorf("archived repo %q should have been filtered out", r.Name)
}
}
expectedExcluded := map[string]bool{"archived-public": true, "archived-private": true}
for _, name := range excluded {
if !expectedExcluded[name] {
t.Errorf("unexpected excluded repo: %q", name)
}
}
}

func TestFilterRepos_ArchivedIncludedWhenConfigured(t *testing.T) {
cfg := &config.Config{
Organization:    "org",
IncludeArchived: boolPtr(true),
}
included, excluded := FilterRepos(sampleReposWithArchived(), cfg)
if len(included) != 4 {
t.Errorf("expected 4 included repos (including archived), got %d", len(included))
}
if len(excluded) != 0 {
t.Errorf("expected 0 excluded repos, got %d", len(excluded))
}
}

package config

import (
	"os"
	"path/filepath"
	"testing"
)

func boolPtr(b bool) *bool {
	return &b
}

func writeTestConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing test config: %v", err)
	}
	return path
}

func TestLoadValidConfigAllFields(t *testing.T) {
	yaml := `
organization: my-org
include_public: false
include_private: true
exclude_repos:
  - legacy-repo
  - "^sandbox-"
branch_hint:
  path: .repo-metadata.yaml
  type: yaml
  json_path: defaults.branch
`
	path := writeTestConfig(t, yaml)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Organization != "my-org" {
		t.Errorf("Organization = %q, want %q", cfg.Organization, "my-org")
	}
	if cfg.IncludePublic == nil || *cfg.IncludePublic != false {
		t.Errorf("IncludePublic = %v, want false", cfg.IncludePublic)
	}
	if cfg.IncludePrivate == nil || *cfg.IncludePrivate != true {
		t.Errorf("IncludePrivate = %v, want true", cfg.IncludePrivate)
	}
	if len(cfg.ExcludeRepos) != 2 {
		t.Errorf("ExcludeRepos length = %d, want 2", len(cfg.ExcludeRepos))
	}
	if cfg.BranchHint == nil {
		t.Fatal("BranchHint = nil, want populated")
	}
	if cfg.BranchHint.Path != ".repo-metadata.yaml" {
		t.Errorf("BranchHint.Path = %q, want %q", cfg.BranchHint.Path, ".repo-metadata.yaml")
	}
	if cfg.BranchHint.Type != "yaml" {
		t.Errorf("BranchHint.Type = %q, want %q", cfg.BranchHint.Type, "yaml")
	}
	if cfg.BranchHint.JSONPath != "defaults.branch" {
		t.Errorf("BranchHint.JSONPath = %q, want %q", cfg.BranchHint.JSONPath, "defaults.branch")
	}
}

func TestDefaults(t *testing.T) {
	yaml := `organization: my-org`
	path := writeTestConfig(t, yaml)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.ShouldIncludePublic() {
		t.Error("ShouldIncludePublic() = false, want true (default)")
	}
	if !cfg.ShouldIncludePrivate() {
		t.Error("ShouldIncludePrivate() = false, want true (default)")
	}
}

func TestValidateMissingOrganization(t *testing.T) {
	cfg := &Config{}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for missing organization and user")
	}
	if err.Error() != "one of organization or user is required" {
		t.Errorf("error = %q, want %q", err.Error(), "one of organization or user is required")
	}
}

func TestValidateBothIncludesFalse(t *testing.T) {
	cfg := &Config{
		Organization:   "my-org",
		IncludePublic:  boolPtr(false),
		IncludePrivate: boolPtr(false),
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error when both includes are false")
	}
	want := "both include_public and include_private are false; no repositories would be included"
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	path := writeTestConfig(t, `organization: [invalid`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestValidateInvalidRegexPattern(t *testing.T) {
	cfg := &Config{
		Organization: "my-org",
		ExcludeRepos: []string{"[invalid"},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for invalid regex pattern")
	}
}

func TestIsExcludedMatching(t *testing.T) {
	cfg := &Config{
		ExcludeRepos: []string{"legacy-repo", "^sandbox-", "-archive$"},
	}

	tests := []struct {
		name     string
		excluded bool
	}{
		{"legacy-repo", true},
		{"sandbox-test", true},
		{"my-archive", true},
		{"production-app", false},
		{"not-excluded", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cfg.IsExcluded(tt.name)
			if got != tt.excluded {
				t.Errorf("IsExcluded(%q) = %v, want %v", tt.name, got, tt.excluded)
			}
		})
	}
}

func TestIsExcludedNonMatching(t *testing.T) {
	cfg := &Config{
		ExcludeRepos: []string{"^sandbox-", "-archive$"},
	}
	if cfg.IsExcluded("production-app") {
		t.Error("IsExcluded(production-app) = true, want false")
	}
	if cfg.IsExcluded("my-sandbox") {
		t.Error("IsExcluded(my-sandbox) = true, want false (prefix pattern should not match suffix)")
	}
}

func TestShouldIncludeArchivedDefault(t *testing.T) {
cfg := &Config{Organization: "my-org"}
if cfg.ShouldIncludeArchived() {
t.Error("ShouldIncludeArchived() = true, want false (default)")
}
}

func TestShouldIncludeArchivedExplicitTrue(t *testing.T) {
cfg := &Config{
Organization:    "my-org",
IncludeArchived: boolPtr(true),
}
if !cfg.ShouldIncludeArchived() {
t.Error("ShouldIncludeArchived() = false, want true when explicitly set")
}
}

func TestShouldIncludeArchivedExplicitFalse(t *testing.T) {
cfg := &Config{
Organization:    "my-org",
IncludeArchived: boolPtr(false),
}
if cfg.ShouldIncludeArchived() {
t.Error("ShouldIncludeArchived() = true, want false when explicitly set to false")
}
}

func TestValidateUserConfig(t *testing.T) {
	cfg := &Config{User: "my-user"}
	err := cfg.Validate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateBothOrganizationAndUser(t *testing.T) {
	cfg := &Config{Organization: "my-org", User: "my-user"}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error when both organization and user are set")
	}
	want := "organization and user are mutually exclusive; specify one but not both"
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}

func TestOwnerReturnsOrganization(t *testing.T) {
	cfg := &Config{Organization: "my-org"}
	if cfg.Owner() != "my-org" {
		t.Errorf("Owner() = %q, want %q", cfg.Owner(), "my-org")
	}
}

func TestOwnerReturnsUser(t *testing.T) {
	cfg := &Config{User: "my-user"}
	if cfg.Owner() != "my-user" {
		t.Errorf("Owner() = %q, want %q", cfg.Owner(), "my-user")
	}
}

func TestIsUserMode(t *testing.T) {
	orgCfg := &Config{Organization: "my-org"}
	if orgCfg.IsUserMode() {
		t.Error("IsUserMode() = true, want false for organization config")
	}
	userCfg := &Config{User: "my-user"}
	if !userCfg.IsUserMode() {
		t.Error("IsUserMode() = false, want true for user config")
	}
}

func TestLoadUserConfig(t *testing.T) {
	yaml := `
user: my-user
include_public: true
include_private: false
`
	path := writeTestConfig(t, yaml)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.User != "my-user" {
		t.Errorf("User = %q, want %q", cfg.User, "my-user")
	}
	if cfg.Organization != "" {
		t.Errorf("Organization = %q, want empty", cfg.Organization)
	}
	if !cfg.IsUserMode() {
		t.Error("IsUserMode() = false, want true")
	}
}

func TestLoadConfigWithIncludeArchived(t *testing.T) {
	yaml := `
organization: my-org
include_archived: true
`
	path := writeTestConfig(t, yaml)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.IncludeArchived == nil || !*cfg.IncludeArchived {
		t.Errorf("IncludeArchived = %v, want true", cfg.IncludeArchived)
	}
	if !cfg.ShouldIncludeArchived() {
		t.Error("ShouldIncludeArchived() = false, want true")
	}
}

func TestValidateBranchHintValid(t *testing.T) {
	cfg := &Config{
		Organization: "my-org",
		BranchHint: &BranchHint{
			Path:     ".repo-metadata.json",
			Type:     "JSON",
			JSONPath: "default.branch",
		},
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.BranchHint.Type != "json" {
		t.Errorf("BranchHint.Type = %q, want %q", cfg.BranchHint.Type, "json")
	}
}

func TestValidateBranchHintMissingPath(t *testing.T) {
	cfg := &Config{
		Organization: "my-org",
		BranchHint: &BranchHint{
			Type:     "yaml",
			JSONPath: "default.branch",
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for missing branch_hint.path")
	}
	if err.Error() != "branch_hint.path is required when branch_hint is specified" {
		t.Errorf("error = %q", err.Error())
	}
}

func TestValidateBranchHintMissingJSONPath(t *testing.T) {
	cfg := &Config{
		Organization: "my-org",
		BranchHint: &BranchHint{
			Path: ".repo-metadata.yaml",
			Type: "yaml",
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for missing branch_hint.json_path")
	}
	if err.Error() != "branch_hint.json_path is required when branch_hint is specified" {
		t.Errorf("error = %q", err.Error())
	}
}

func TestValidateBranchHintInvalidType(t *testing.T) {
	cfg := &Config{
		Organization: "my-org",
		BranchHint: &BranchHint{
			Path:     ".repo-metadata.txt",
			Type:     "toml",
			JSONPath: "default.branch",
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for invalid branch_hint.type")
	}
	if err.Error() != "branch_hint.type must be one of json or yaml" {
		t.Errorf("error = %q", err.Error())
	}
}

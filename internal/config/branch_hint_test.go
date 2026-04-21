package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeBranchHintFile(t *testing.T, relPath, content string) string {
	t.Helper()

	dir := t.TempDir()
	fullPath := filepath.Join(dir, relPath)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		t.Fatalf("creating branch hint dir: %v", err)
	}
	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		t.Fatalf("writing branch hint file: %v", err)
	}

	return dir
}

func TestResolveBranchHintJSON(t *testing.T) {
	repoDir := writeBranchHintFile(t, ".repo-metadata.json", `{"default":{"branch":"develop"}}`)

	branch := ResolveBranchHint(repoDir, &BranchHint{
		Path:     ".repo-metadata.json",
		Type:     "json",
		JSONPath: "default.branch",
	})

	if branch != "develop" {
		t.Errorf("ResolveBranchHint() = %q, want %q", branch, "develop")
	}
}

func TestResolveBranchHintYAML(t *testing.T) {
	repoDir := writeBranchHintFile(t, ".repo-metadata.yaml", "default:\n  branch: release\n")

	branch := ResolveBranchHint(repoDir, &BranchHint{
		Path:     ".repo-metadata.yaml",
		Type:     "yaml",
		JSONPath: "default.branch",
	})

	if branch != "release" {
		t.Errorf("ResolveBranchHint() = %q, want %q", branch, "release")
	}
}

func TestResolveBranchHintTrimsValue(t *testing.T) {
	repoDir := writeBranchHintFile(t, ".repo-metadata.yaml", "default:\n  branch: \"  main  \"\n")

	branch := ResolveBranchHint(repoDir, &BranchHint{
		Path:     ".repo-metadata.yaml",
		Type:     "yaml",
		JSONPath: "default.branch",
	})

	if branch != "main" {
		t.Errorf("ResolveBranchHint() = %q, want %q", branch, "main")
	}
}

func TestResolveBranchHintArrayPath(t *testing.T) {
	repoDir := writeBranchHintFile(t, "config.json", `{"branches":[{"name":"main"},{"name":"develop"}]}`)

	branch := ResolveBranchHint(repoDir, &BranchHint{
		Path:     "config.json",
		Type:     "json",
		JSONPath: "branches.1.name",
	})

	if branch != "develop" {
		t.Errorf("ResolveBranchHint() = %q, want %q", branch, "develop")
	}
}

func TestResolveBranchHintFallsBackOnMissingFile(t *testing.T) {
	branch := ResolveBranchHint(t.TempDir(), &BranchHint{
		Path:     ".repo-metadata.yaml",
		Type:     "yaml",
		JSONPath: "default.branch",
	})

	if branch != "" {
		t.Errorf("ResolveBranchHint() = %q, want empty string", branch)
	}
}

func TestResolveBranchHintFallsBackOnMissingPath(t *testing.T) {
	repoDir := writeBranchHintFile(t, ".repo-metadata.yaml", "default:\n  branch: main\n")

	branch := ResolveBranchHint(repoDir, &BranchHint{
		Path:     ".repo-metadata.yaml",
		Type:     "yaml",
		JSONPath: "default.missing",
	})

	if branch != "" {
		t.Errorf("ResolveBranchHint() = %q, want empty string", branch)
	}
}

func TestResolveBranchHintFallsBackOnBlankValue(t *testing.T) {
	repoDir := writeBranchHintFile(t, ".repo-metadata.yaml", "default:\n  branch: \"   \"\n")

	branch := ResolveBranchHint(repoDir, &BranchHint{
		Path:     ".repo-metadata.yaml",
		Type:     "yaml",
		JSONPath: "default.branch",
	})

	if branch != "" {
		t.Errorf("ResolveBranchHint() = %q, want empty string", branch)
	}
}

func TestResolveBranchHintFallsBackOnParseError(t *testing.T) {
	repoDir := writeBranchHintFile(t, ".repo-metadata.json", "{invalid")

	branch := ResolveBranchHint(repoDir, &BranchHint{
		Path:     ".repo-metadata.json",
		Type:     "json",
		JSONPath: "default.branch",
	})

	if branch != "" {
		t.Errorf("ResolveBranchHint() = %q, want empty string", branch)
	}
}

func TestResolveBranchHintFallsBackOnNonStringValue(t *testing.T) {
	repoDir := writeBranchHintFile(t, ".repo-metadata.json", `{"default":{"branch":123}}`)

	branch := ResolveBranchHint(repoDir, &BranchHint{
		Path:     ".repo-metadata.json",
		Type:     "json",
		JSONPath: "default.branch",
	})

	if branch != "" {
		t.Errorf("ResolveBranchHint() = %q, want empty string", branch)
	}
}

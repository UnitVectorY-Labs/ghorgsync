// Package archived implements conservative, opt-in removal of archived clones.
package archived

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/UnitVectorY-Labs/ghorgsync/internal/config"
	"github.com/UnitVectorY-Labs/ghorgsync/internal/gitidentity"
	"github.com/UnitVectorY-Labs/ghorgsync/internal/model"
)

// Path permits only an immediate, visible child of the sync root.
func Path(root, name string) (string, error) {
	if name == "" || strings.HasPrefix(name, ".") || strings.ContainsAny(name, `/\\`) || filepath.Base(name) != name {
		return "", fmt.Errorf("unsafe repository name %q", name)
	}
	return filepath.Join(root, name), nil
}

// MatchRemote accepts only canonical GitHub clone URLs for the expected repository.
func MatchRemote(remote, expected string) bool {
	return gitidentity.MatchRemote(remote, expected)
}

func git(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	// Status must not refresh the index during a dry run.
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0", "GIT_NO_REPLACE_OBJECTS=1")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s failed: %w", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out)), nil
}

func directory(path string) (os.FileInfo, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("expected a real directory: %s", path)
	}
	return info, nil
}

// Check always verifies identity and layout. Without force it also protects
// local content and unpublished history.
// Fetching into a temporary bare repository verifies current GitHub branches
// without changing the candidate's refs, index, config, or working tree.
func Check(root string, repo model.RepoInfo, force bool) (os.FileInfo, error) {
	if !repo.IsArchived {
		return nil, fmt.Errorf("repository is not archived")
	}
	path, err := Path(root, repo.Name)
	if err != nil {
		return nil, err
	}
	info, err := directory(path)
	if err != nil {
		return nil, err
	}
	if _, err := directory(filepath.Join(path, ".git")); err != nil {
		return nil, fmt.Errorf("worktrees and nonstandard git directories are not eligible: %w", err)
	}
	top, err := git(path, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, err
	}
	actual, err := filepath.EvalSymlinks(path)
	if err != nil || top != actual {
		return nil, fmt.Errorf("unexpected repository root")
	}
	remote, err := git(path, "remote", "get-url", "origin")
	if err != nil {
		return nil, err
	}
	if !MatchRemote(remote, repo.CloneURL) {
		return nil, fmt.Errorf("origin does not match GitHub repository")
	}
	for _, item := range []string{"worktrees", "objects/info/alternates", "info/grafts", "shallow"} {
		if _, err := os.Lstat(filepath.Join(path, ".git", item)); !os.IsNotExist(err) {
			return nil, fmt.Errorf("unsupported git metadata: %s", item)
		}
	}
	// Force overrides local content/history protection, never identity or layout.
	if force {
		return info, nil
	}
	if _, err := os.Lstat(filepath.Join(path, ".git", "modules")); !os.IsNotExist(err) {
		return nil, &RetainedError{Reason: "submodule data present; use --force to delete local content"}
	}
	if err := inspectContent(path); err != nil {
		return nil, err
	}
	// Fetch only from the API-provided URL, never from local remote configuration.
	temp, err := os.MkdirTemp("", "ghorgsync-archived-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(temp)
	if _, err := git(temp, "init", "--bare", "--quiet"); err != nil {
		return nil, err
	}
	if _, err := git(temp, "fetch", "--quiet", "--no-tags", repo.CloneURL, "+refs/heads/*:refs/remotes/github/*"); err != nil {
		return nil, err
	}
	// Read local objects through alternates, but compare against freshly fetched refs only.
	objects := filepath.Join(actual, ".git", "objects")
	if err := os.WriteFile(filepath.Join(temp, "objects", "info", "alternates"), []byte(objects+"\n"), 0600); err != nil {
		return nil, err
	}
	commits, err := git(path, "rev-list", "--all", "--reflog", "HEAD")
	if err != nil {
		return nil, err
	}
	if commits == "" {
		return nil, fmt.Errorf("no local history could be verified")
	}
	cmd := exec.Command("git", "-C", temp, "rev-list", "--stdin", "--not", "--remotes=github")
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_NO_REPLACE_OBJECTS=1")
	cmd.Stdin = strings.NewReader(commits + "\n")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("cannot verify local history: %w", err)
	}
	if len(out) != 0 {
		return nil, &RetainedError{Reason: "local-only history (including stashes and reflogs) is not reachable from GitHub branches; inspect git stash list and git log --all --reflog --not --remotes=origin"}
	}
	// Network verification can take time; inspect local content again afterward.
	if err := inspectContent(path); err != nil {
		return nil, err
	}
	current, err := directory(path)
	if err != nil {
		return nil, err
	}
	if !os.SameFile(info, current) {
		return nil, fmt.Errorf("repository directory changed during inspection")
	}
	return current, nil
}

// Remove rechecks eligibility and rejects directory replacement.
func Remove(root string, repo model.RepoInfo, planned os.FileInfo, force bool) error {
	current, err := Check(root, repo, force)
	if err != nil {
		return err
	}
	if !os.SameFile(planned, current) {
		return fmt.Errorf("repository directory changed after inspection")
	}
	path, err := Path(root, repo.Name)
	if err != nil {
		return err
	}
	return os.RemoveAll(path)
}

// validateContent is deliberately independent of git execution for unit testing.
func validateContent(status, index, flags string) error {
	// Ignored content is intentionally removed with the archived clone. Any
	// tracked change or non-ignored untracked entry still protects the folder.
	changed := 0
	for _, line := range strings.Split(status, "\n") {
		if line != "" && !strings.HasPrefix(line, "!! ") {
			changed++
		}
	}
	if changed > 0 {
		return &RetainedError{Reason: fmt.Sprintf("%d changed or untracked entries; local folder protected", changed)}
	}
	for _, line := range strings.Split(index, "\n") {
		if strings.HasPrefix(line, "160000 ") {
			return &RetainedError{Reason: "submodules present; use --force to delete local content"}
		}
	}
	for _, line := range strings.Split(flags, "\n") {
		if len(line) > 0 && (line[0] == 'S' || (line[0] >= 'a' && line[0] <= 'z')) {
			return &RetainedError{Reason: "index contains skip-worktree or assume-unchanged entries; use --force to delete local content"}
		}
	}
	return nil
}

// Candidate mirrors filtering precedence: visibility and empty repositories are
// skipped before archive exclusions, and must never become removal candidates.
func Candidate(repo model.RepoInfo, cfg *config.Config) bool {
	return repo.IsArchived && !repo.IsEmpty && !cfg.ShouldIncludeArchived() &&
		((repo.IsPrivate && cfg.ShouldIncludePrivate()) || (!repo.IsPrivate && cfg.ShouldIncludePublic()))
}

func inspectContent(path string) error {
	flags, err := git(path, "ls-files", "-v")
	if err != nil {
		return err
	}
	status, err := git(path, "status", "--porcelain=v1", "--untracked-files=all", "--ignored", "--ignore-submodules=none")
	if err != nil {
		return err
	}
	index, err := git(path, "ls-files", "--stage")
	if err != nil {
		return err
	}
	return validateContent(status, index, flags)
}

// RetainedError is an expected safety finding rather than an operational error.
type RetainedError struct{ Reason string }

func (e *RetainedError) Error() string { return e.Reason }

// IsRetained distinguishes protected local data from failed inspection commands.
func IsRetained(err error) bool {
	var retained *RetainedError
	return errors.As(err, &retained)
}

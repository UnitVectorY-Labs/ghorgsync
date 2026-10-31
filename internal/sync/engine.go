package sync

import (
	"path/filepath"

	"github.com/UnitVectorY-Labs/ghorgsync/internal/model"
)

// Engine orchestrates per-repo sync operations.
type Engine struct {
	Git     GitRunner
	BaseDir string
	Verbose bool
}

// NewEngine creates a new sync engine.
// verbosity controls logging: 0=quiet, 1=verbose (cmd/exit), 2=trace (also raw output).
func NewEngine(baseDir string, verbosity int, logf func(string, ...any), tracef func(string, ...any), options ...GitOptions) *Engine {
	settings := GitOptions{}
	if len(options) > 0 {
		settings = options[0]
	}
	inner := &ExecGitRunner{options: settings}
	if verbosity >= 2 {
		inner.tracef = tracef
	}
	if verbosity >= 1 {
		inner.logf = logf
	}

	var gitRunner GitRunner = inner
	if verbosity >= 1 && logf != nil {
		gitRunner = NewLoggingGitRunner(inner, logf)
	}

	return &Engine{
		Git:     gitRunner,
		BaseDir: baseDir,
		Verbose: verbosity >= 1,
	}
}

// CloneRepo clones a missing repository.
func (e *Engine) CloneRepo(repo model.RepoInfo) model.RepoResult {
	dest := filepath.Join(e.BaseDir, repo.Name)
	err := e.Git.Clone(repo.CloneURL, dest)
	if err != nil {
		return model.RepoResult{
			Name:          repo.Name,
			Action:        model.ActionCloneError,
			DefaultBranch: repo.DefaultBranch,
			Error:         err,
		}
	}
	return model.RepoResult{
		Name:          repo.Name,
		Action:        model.ActionCloned,
		DefaultBranch: repo.DefaultBranch,
	}
}

// ProcessRepo audits and syncs an existing local repository.
func (e *Engine) ProcessRepo(repo model.RepoInfo) model.RepoResult {
	repoDir := filepath.Join(e.BaseDir, repo.Name)
	result := model.RepoResult{
		Name:          repo.Name,
		DefaultBranch: repo.DefaultBranch,
	}

	// Always fetch (safe operation)
	if err := e.Git.Fetch(repoDir); err != nil {
		result.Action = model.ActionFetchError
		result.Error = err
		return result
	}

	// Initialize and update submodules to avoid false dirty state from
	// uninitialized submodule directories.
	if err := e.Git.SubmoduleUpdate(repoDir); err != nil {
		result.Action = model.ActionSubmoduleError
		result.Error = err
		return result
	}

	// Get current branch
	branch, err := e.Git.CurrentBranch(repoDir)
	if err != nil {
		result.Action = model.ActionFetchError
		result.Error = err
		return result
	}
	result.CurrentBranch = branch
	result.BranchDrift = branch != repo.DefaultBranch

	// Check dirty state
	dirty, files, err := e.Git.IsDirty(repoDir)
	if err != nil {
		result.Action = model.ActionFetchError
		result.Error = err
		return result
	}

	if dirty {
		result.Action = model.ActionDirty
		result.DirtyFiles = files
		// Get diff stats
		adds, dels, err := e.Git.DiffStats(repoDir)
		if commandInterrupted(err) {
			result.Action = model.ActionFetchError
			result.Error = err
			return result
		}
		result.Additions = adds
		result.Deletions = dels
		return result
	}

	// Clean repo: checkout default branch if needed, then pull
	if result.BranchDrift {
		if err := e.Git.Checkout(repoDir, repo.DefaultBranch); err != nil {
			result.Action = model.ActionCheckoutError
			result.Error = err
			return result
		}
		result.CurrentBranch = repo.DefaultBranch
	}

	// Pull with ff-only
	changed, err := e.Git.PullFF(repoDir)
	if err != nil {
		result.Action = model.ActionPullError
		result.Error = err
		return result
	}

	// Update submodule pointers after pull to keep them in sync with the new commits.
	// Report failures, including inactivity timeouts, even after a successful pull.
	if err := e.Git.SubmoduleUpdate(repoDir); err != nil {
		result.Action = model.ActionSubmoduleError
		result.Error = err
		result.Updated = changed
		return result
	}

	result.Updated = changed

	if changed {
		if result.BranchDrift {
			result.Action = model.ActionBranchDrift
		} else {
			result.Action = model.ActionUpdated
		}
	} else {
		if result.BranchDrift {
			result.Action = model.ActionBranchDrift
		} else {
			result.Action = model.ActionAlreadyCurrent
		}
	}

	return result
}

// StatusRepo reads the current state of a repository without modifying it.
// It returns ActionDirty if the working tree is dirty, ActionBranchDrift if
// the repo is on a non-default branch (and clean), or ActionAlreadyCurrent
// if the repo is clean and on the default branch.
func (e *Engine) StatusRepo(repo model.RepoInfo) model.RepoResult {
	repoDir := filepath.Join(e.BaseDir, repo.Name)
	result := model.RepoResult{
		Name:          repo.Name,
		DefaultBranch: repo.DefaultBranch,
	}

	// Get current branch
	branch, err := e.Git.CurrentBranch(repoDir)
	if err != nil {
		result.Action = model.ActionFetchError
		result.Error = err
		return result
	}
	result.CurrentBranch = branch
	result.BranchDrift = branch != repo.DefaultBranch

	// Check dirty state
	dirty, files, err := e.Git.IsDirty(repoDir)
	if err != nil {
		result.Action = model.ActionFetchError
		result.Error = err
		return result
	}

	if dirty {
		result.Action = model.ActionDirty
		result.DirtyFiles = files
		// Get colorized status output for display; non-fatal if unavailable
		// since the dirty state is already captured in DirtyFiles.
		statusOut, err := e.Git.StatusShort(repoDir)
		if commandInterrupted(err) {
			result.Action = model.ActionFetchError
			result.Error = err
			return result
		}
		result.StatusOutput = statusOut
		return result
	}

	if result.BranchDrift {
		result.Action = model.ActionBranchDrift
		return result
	}

	result.Action = model.ActionAlreadyCurrent
	return result
}

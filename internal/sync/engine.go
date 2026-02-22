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
func NewEngine(baseDir string, verbose bool) *Engine {
	return &Engine{
		Git:     &ExecGitRunner{},
		BaseDir: baseDir,
		Verbose: verbose,
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
		adds, dels, _ := e.Git.DiffStats(repoDir)
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
	// This is non-fatal: if submodule update fails after a successful pull, we still
	// report the pull result and the error will surface on the next sync cycle.
	_ = e.Git.SubmoduleUpdate(repoDir)

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

package sync

import (
	"path/filepath"

	"github.com/UnitVectorY-Labs/ghorgsync/internal/config"
	"github.com/UnitVectorY-Labs/ghorgsync/internal/model"
)

// Engine orchestrates per-repo sync operations.
type Engine struct {
	Git        GitRunner
	BaseDir    string
	Verbose    bool
	BranchHint *config.BranchHint
}

// NewEngine creates a new sync engine.
func NewEngine(baseDir string, verbose bool, branchHint *config.BranchHint) *Engine {
	return &Engine{
		Git:        &ExecGitRunner{},
		BaseDir:    baseDir,
		Verbose:    verbose,
		BranchHint: branchHint,
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
	defaultBranch := e.resolveDefaultBranch(repo, repoDir)
	result := model.RepoResult{
		Name:          repo.Name,
		DefaultBranch: defaultBranch,
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
	result.BranchDrift = branch != defaultBranch

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
		if err := e.Git.Checkout(repoDir, defaultBranch); err != nil {
			result.Action = model.ActionCheckoutError
			result.Error = err
			return result
		}
		result.CurrentBranch = defaultBranch
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

// StatusRepo reads the current state of a repository without modifying it.
// It returns ActionDirty if the working tree is dirty, ActionBranchDrift if
// the repo is on a non-default branch (and clean), or ActionAlreadyCurrent
// if the repo is clean and on the default branch.
func (e *Engine) StatusRepo(repo model.RepoInfo) model.RepoResult {
	repoDir := filepath.Join(e.BaseDir, repo.Name)
	defaultBranch := e.resolveDefaultBranch(repo, repoDir)
	result := model.RepoResult{
		Name:          repo.Name,
		DefaultBranch: defaultBranch,
	}

	// Get current branch
	branch, err := e.Git.CurrentBranch(repoDir)
	if err != nil {
		result.Action = model.ActionFetchError
		result.Error = err
		return result
	}
	result.CurrentBranch = branch
	result.BranchDrift = branch != defaultBranch

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
		statusOut, _ := e.Git.StatusShort(repoDir)
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

func (e *Engine) resolveDefaultBranch(repo model.RepoInfo, repoDir string) string {
	if branch := config.ResolveBranchHint(repoDir, e.BranchHint); branch != "" {
		return branch
	}

	return repo.DefaultBranch
}

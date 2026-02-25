package model

// RepoInfo represents a GitHub repository from the org inventory.
type RepoInfo struct {
	Name          string
	CloneURL      string
	DefaultBranch string
	IsPrivate     bool
	IsArchived    bool
}

// LocalClassification represents the classification of a local directory entry.
type LocalClassification int

const (
	ClassManaged           LocalClassification = iota // Matches an included repo
	ClassUnknown                                      // No matching repo (included or excluded)
	ClassExcludedButPresent                           // Matches an excluded repo name/pattern
	ClassCollision                                    // Path exists but is not a valid clone
)

// String returns a human-readable name for the classification.
func (c LocalClassification) String() string {
	switch c {
	case ClassManaged:
		return "managed"
	case ClassUnknown:
		return "unknown"
	case ClassExcludedButPresent:
		return "excluded-but-present"
	case ClassCollision:
		return "collision"
	default:
		return "unknown"
	}
}

// RepoAction represents the action taken or finding for a repository.
type RepoAction int

const (
	ActionNone          RepoAction = iota
	ActionCloned                   // Repository was cloned
	ActionUpdated                  // Repository was pulled with new changes
	ActionAlreadyCurrent           // Repository was already up to date
	ActionDirty                    // Repository has uncommitted changes
	ActionBranchDrift              // Repository was on wrong branch (checkout performed)
	ActionCloneError               // Clone failed
	ActionFetchError               // Fetch failed
	ActionCheckoutError            // Checkout failed
	ActionPullError                // Pull failed
	ActionSubmoduleError           // Submodule update failed
)

// String returns a human-readable name for the action.
func (a RepoAction) String() string {
	switch a {
	case ActionNone:
		return "none"
	case ActionCloned:
		return "cloned"
	case ActionUpdated:
		return "updated"
	case ActionAlreadyCurrent:
		return "up-to-date"
	case ActionDirty:
		return "dirty"
	case ActionBranchDrift:
		return "branch-drift"
	case ActionCloneError:
		return "clone-error"
	case ActionFetchError:
		return "fetch-error"
	case ActionCheckoutError:
		return "checkout-error"
	case ActionPullError:
		return "pull-error"
	case ActionSubmoduleError:
		return "submodule-error"
	default:
		return "unknown"
	}
}

// DirtyFile represents a single changed file in a dirty repo.
type DirtyFile struct {
	Path     string
	Staged   bool
	Unstaged bool
}

// RepoResult holds the outcome of processing a single repository.
type RepoResult struct {
	Name          string
	Action        RepoAction
	CurrentBranch string
	DefaultBranch string
	Error         error
	DirtyFiles    []DirtyFile
	Additions     int
	Deletions     int
	BranchDrift   bool // true if current != default branch at start
	Updated       bool // true if pull brought new changes
}

// LocalEntry represents a classified local directory entry.
type LocalEntry struct {
	Name           string
	Classification LocalClassification
	Detail         string // additional info (e.g., collision reason)
}

// Summary holds aggregate counts for the final report.
type Summary struct {
	TotalRepos         int
	Cloned             int
	Updated            int
	Dirty              int
	BranchDrift        int
	UnknownFolders     int
	ExcludedButPresent int
	Errors             int
}

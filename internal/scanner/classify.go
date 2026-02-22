package scanner

import (
	"github.com/UnitVectorY-Labs/ghorgsync/internal/model"
)

// ClassifyEntry classifies a single directory name given the included repos and excluded names.
// isDir: whether the entry is a directory
// isGitRepo: whether the directory contains a .git subdirectory (only relevant if isDir)
// Returns the classification and detail string.
func ClassifyEntry(name string, isDir bool, isGitRepo bool, includedSet map[string]bool, isExcluded func(string) bool) model.LocalEntry {
	if !isDir {
		if includedSet[name] {
			return model.LocalEntry{
				Name:           name,
				Classification: model.ClassCollision,
				Detail:         "path is a file, not a directory",
			}
		}
		// Non-directory non-managed entries are ignored (not classified)
		return model.LocalEntry{Name: name, Classification: model.ClassUnknown}
	}

	if includedSet[name] {
		if !isGitRepo {
			return model.LocalEntry{
				Name:           name,
				Classification: model.ClassCollision,
				Detail:         "directory exists but is not a git repository",
			}
		}
		return model.LocalEntry{
			Name:           name,
			Classification: model.ClassManaged,
		}
	}

	if isExcluded(name) {
		return model.LocalEntry{
			Name:           name,
			Classification: model.ClassExcludedButPresent,
		}
	}

	return model.LocalEntry{
		Name:           name,
		Classification: model.ClassUnknown,
	}
}

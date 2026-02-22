package scanner

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/UnitVectorY-Labs/ghorgsync/internal/config"
	"github.com/UnitVectorY-Labs/ghorgsync/internal/model"
)

// ScanResult holds the result of scanning the local directory.
type ScanResult struct {
	// ManagedFound are repos that exist locally and match an included repo
	ManagedFound []string
	// ManagedMissing are repos that don't exist locally (clone candidates)
	ManagedMissing []string
	// Collisions are repos where the path exists but isn't a valid git clone
	Collisions []model.LocalEntry
	// Unknown are directories that don't match any repo (included or excluded)
	Unknown []model.LocalEntry
	// ExcludedButPresent are directories matching excluded repos
	ExcludedButPresent []model.LocalEntry
}

// ScanDirectory scans the given directory and classifies each immediate child entry.
// includedRepos: repos after visibility+exclusion filtering (the ones we want to manage)
// excludedNames: repo names that were excluded by config
// allRepoNames: set of all org repo names (before exclusion) for unknown classification
func ScanDirectory(dir string, includedRepos []model.RepoInfo, excludedNames []string, cfg *config.Config) (*ScanResult, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading directory: %w", err)
	}

	// Build lookup maps
	includedMap := make(map[string]model.RepoInfo)
	for _, r := range includedRepos {
		includedMap[r.Name] = r
	}
	excludedSet := make(map[string]bool)
	for _, name := range excludedNames {
		excludedSet[name] = true
	}

	result := &ScanResult{}
	localDirs := make(map[string]bool)

	for _, entry := range entries {
		name := entry.Name()
		// Skip hidden files/directories (starting with .)
		if len(name) > 0 && name[0] == '.' {
			continue
		}
		// Only consider directories
		if !entry.IsDir() {
			// If a regular file matches a managed repo name, it's a collision
			if _, ok := includedMap[name]; ok {
				result.Collisions = append(result.Collisions, model.LocalEntry{
					Name:           name,
					Classification: model.ClassCollision,
					Detail:         "path is a file, not a directory",
				})
			}
			continue
		}

		localDirs[name] = true

		if _, ok := includedMap[name]; ok {
			// Check if it's a valid git repo
			gitDir := filepath.Join(dir, name, ".git")
			info, err := os.Stat(gitDir)
			if err != nil || !info.IsDir() {
				result.Collisions = append(result.Collisions, model.LocalEntry{
					Name:           name,
					Classification: model.ClassCollision,
					Detail:         "directory exists but is not a git repository",
				})
				continue
			}
			result.ManagedFound = append(result.ManagedFound, name)
		} else if excludedSet[name] || cfg.IsExcluded(name) {
			result.ExcludedButPresent = append(result.ExcludedButPresent, model.LocalEntry{
				Name:           name,
				Classification: model.ClassExcludedButPresent,
			})
		} else {
			result.Unknown = append(result.Unknown, model.LocalEntry{
				Name:           name,
				Classification: model.ClassUnknown,
			})
		}
	}

	// Determine missing repos (included but not found locally)
	for _, r := range includedRepos {
		if !localDirs[r.Name] {
			// Also check it's not a collision (file at path)
			isCollision := false
			for _, c := range result.Collisions {
				if c.Name == r.Name {
					isCollision = true
					break
				}
			}
			if !isCollision {
				result.ManagedMissing = append(result.ManagedMissing, r.Name)
			}
		}
	}

	return result, nil
}

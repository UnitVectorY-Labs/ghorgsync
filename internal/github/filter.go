package github

import (
	"github.com/UnitVectorY-Labs/ghorgsync/internal/config"
	"github.com/UnitVectorY-Labs/ghorgsync/internal/model"
)

// FilterRepos applies visibility, archived, and exclusion filters to the repo list.
func FilterRepos(repos []model.RepoInfo, cfg *config.Config) (included []model.RepoInfo, excluded []string) {
	for _, r := range repos {
		// Visibility filter
		if r.IsPrivate && !cfg.ShouldIncludePrivate() {
			continue
		}
		if !r.IsPrivate && !cfg.ShouldIncludePublic() {
			continue
		}
		// Archived filter: skip archived repos unless explicitly included
		if r.IsArchived && !cfg.ShouldIncludeArchived() {
			excluded = append(excluded, r.Name)
			continue
		}
		// Exclusion filter
		if cfg.IsExcluded(r.Name) {
			excluded = append(excluded, r.Name)
			continue
		}
		included = append(included, r)
	}
	return
}

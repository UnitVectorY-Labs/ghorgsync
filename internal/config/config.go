package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// BranchHint configures optional local default-branch discovery.
type BranchHint struct {
	Path     string `yaml:"path"`
	Type     string `yaml:"type"`
	JSONPath string `yaml:"json_path"`
}

// Config represents the application configuration loaded from a YAML file.
type Config struct {
	Organization    string      `yaml:"organization"`
	User            string      `yaml:"user"`
	IncludePublic   *bool       `yaml:"include_public"`
	IncludePrivate  *bool       `yaml:"include_private"`
	IncludeArchived *bool       `yaml:"include_archived"`
	ExcludeRepos    []string    `yaml:"exclude_repos"`
	BranchHint      *BranchHint `yaml:"branch_hint"`

	// compiledExcludes caches compiled regex patterns for ExcludeRepos.
	compiledExcludes []*regexp.Regexp
}

// Load reads a YAML configuration file from the given path and returns a parsed Config.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	return &cfg, nil
}

// Validate checks the configuration for logical errors.
func (c *Config) Validate() error {
	if c.Organization != "" && c.User != "" {
		return fmt.Errorf("organization and user are mutually exclusive; specify one but not both")
	}

	if c.Organization == "" && c.User == "" {
		return fmt.Errorf("one of organization or user is required")
	}

	if c.IncludePublic != nil && !*c.IncludePublic &&
		c.IncludePrivate != nil && !*c.IncludePrivate {
		return fmt.Errorf("both include_public and include_private are false; no repositories would be included")
	}

	if c.BranchHint != nil {
		if strings.TrimSpace(c.BranchHint.Path) == "" {
			return fmt.Errorf("branch_hint.path is required when branch_hint is specified")
		}
		if strings.TrimSpace(c.BranchHint.JSONPath) == "" {
			return fmt.Errorf("branch_hint.json_path is required when branch_hint is specified")
		}

		hintType := strings.ToLower(strings.TrimSpace(c.BranchHint.Type))
		switch hintType {
		case "json", "yaml", "yml":
			c.BranchHint.Type = hintType
		default:
			return fmt.Errorf("branch_hint.type must be one of json or yaml")
		}
	}

	for _, pattern := range c.ExcludeRepos {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return fmt.Errorf("invalid exclude_repos pattern %q: %w", pattern, err)
		}
		c.compiledExcludes = append(c.compiledExcludes, re)
	}

	return nil
}

// Owner returns the configured organization or user name.
// This should only be called after Validate() has confirmed that exactly one of
// Organization or User is set.
func (c *Config) Owner() string {
	if c.Organization != "" {
		return c.Organization
	}
	return c.User
}

// IsUserMode returns true if the configuration targets a user account rather than an organization.
func (c *Config) IsUserMode() bool {
	return c.User != ""
}

// ShouldIncludePublic returns true if public repositories should be included.
// Defaults to true when not explicitly set.
func (c *Config) ShouldIncludePublic() bool {
	return c.IncludePublic == nil || *c.IncludePublic
}

// ShouldIncludePrivate returns true if private repositories should be included.
// Defaults to true when not explicitly set.
func (c *Config) ShouldIncludePrivate() bool {
	return c.IncludePrivate == nil || *c.IncludePrivate
}

// ShouldIncludeArchived returns true if archived repositories should be included.
// Defaults to false when not explicitly set.
func (c *Config) ShouldIncludeArchived() bool {
	return c.IncludeArchived != nil && *c.IncludeArchived
}

// IsExcluded checks whether the given repository name matches any pattern in ExcludeRepos.
func (c *Config) IsExcluded(repoName string) bool {
	// Use cached compiled patterns if available (after Validate has been called)
	if len(c.compiledExcludes) > 0 {
		for _, re := range c.compiledExcludes {
			if re.MatchString(repoName) {
				return true
			}
		}
		return false
	}
	// Fallback: compile on the fly (before Validate is called)
	for _, pattern := range c.ExcludeRepos {
		re, err := regexp.Compile(pattern)
		if err != nil {
			continue
		}
		if re.MatchString(repoName) {
			return true
		}
	}
	return false
}

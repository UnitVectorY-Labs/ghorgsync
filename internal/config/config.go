package config

import (
	"fmt"
	"os"
	"regexp"

	"gopkg.in/yaml.v3"
)

// Config represents the application configuration loaded from a YAML file.
type Config struct {
	Organization   string   `yaml:"organization"`
	IncludePublic  *bool    `yaml:"include_public"`
	IncludePrivate *bool    `yaml:"include_private"`
	ExcludeRepos   []string `yaml:"exclude_repos"`

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
	if c.Organization == "" {
		return fmt.Errorf("organization is required")
	}

	if c.IncludePublic != nil && !*c.IncludePublic &&
		c.IncludePrivate != nil && !*c.IncludePrivate {
		return fmt.Errorf("both include_public and include_private are false; no repositories would be included")
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

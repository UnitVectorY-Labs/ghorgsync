// Package gitidentity compares GitHub clone identities without executing git.
package gitidentity

import "strings"

// MatchRemote accepts only canonical GitHub clone URLs for the expected repository.
func MatchRemote(remote, expected string) bool {
	a, b := Repository(remote), Repository(expected)
	return a != "" && a == b
}

// Repository returns a normalized owner/repository for a canonical GitHub URL,
// or an empty string when the URL cannot be verified. Credentials are rejected.
func Repository(s string) string {
	switch {
	case strings.HasPrefix(s, "https://github.com/"):
		s = strings.TrimPrefix(s, "https://github.com/")
	case strings.HasPrefix(s, "git@github.com:"):
		s = strings.TrimPrefix(s, "git@github.com:")
	case strings.HasPrefix(s, "ssh://git@github.com/"):
		s = strings.TrimPrefix(s, "ssh://git@github.com/")
	default:
		return ""
	}
	s = strings.TrimSuffix(s, ".git")
	parts := strings.Split(s, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" || strings.ContainsAny(s, "?#@\\\n\r") {
		return ""
	}
	return strings.ToLower(s)
}

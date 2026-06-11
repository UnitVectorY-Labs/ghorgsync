package github

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/UnitVectorY-Labs/ghorgsync/internal/model"
)

// Client wraps GitHub API access.
type Client struct {
	token      string
	httpClient *http.Client
	verbosef   func(string, ...any)
	tracef     func(string, ...any)

	// cached authenticated user (populated lazily by GetAuthenticatedUser)
	authUserOnce  sync.Once
	authUserLogin string
	authUserErr   error
}

// NewClient creates a new GitHub API client.
// It resolves a token from GITHUB_TOKEN, GH_TOKEN env vars, or gh CLI auth.
// logf is called for verbose (level-1) messages; tracef for trace (level-2) messages.
func NewClient(token string, logf func(string, ...any), tracef func(string, ...any)) *Client {
	return &Client{
		token: token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		verbosef: logf,
		tracef:   tracef,
	}
}

// ResolveToken finds a GitHub token from environment variables or gh CLI.
// Priority: GITHUB_TOKEN > GH_TOKEN > gh auth token
func ResolveToken() string {
	if t := os.Getenv("GITHUB_TOKEN"); t != "" {
		return t
	}
	if t := os.Getenv("GH_TOKEN"); t != "" {
		return t
	}
	out, err := exec.Command("gh", "auth", "token").Output()
	if err == nil {
		t := strings.TrimSpace(string(out))
		if t != "" {
			return t
		}
	}
	return ""
}

// ghRepo is the JSON shape returned by the GitHub repos API.
type ghRepo struct {
	Name          string `json:"name"`
	CloneURL      string `json:"clone_url"`
	DefaultBranch string `json:"default_branch"`
	Private       bool   `json:"private"`
	Archived      bool   `json:"archived"`
}

// listRepos fetches all repositories from the given paginated GitHub API URL.
func (c *Client) listRepos(url string) ([]model.RepoInfo, error) {
	var repos []model.RepoInfo

	for url != "" {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("creating request: %w", err)
		}
		req.Header.Set("Accept", "application/vnd.github+json")
		if c.token != "" {
			req.Header.Set("Authorization", "Bearer "+c.token)
		}
		c.verbosefSafe("api request: %s %s headers={Accept:%q Authorization:%t}",
			req.Method, sanitizeRequestURL(url), req.Header.Get("Accept"), c.token != "")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("requesting repos: %w", err)
		}
		c.verbosefSafe("api response: %s %s status=%d", req.Method, sanitizeRequestURL(url), resp.StatusCode)

		bodyBytes, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("reading response body: %w", err)
		}
		c.tracefSafe("api body: %s", bytes.TrimSpace(bodyBytes))

		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return nil, fmt.Errorf("GitHub API auth error (HTTP %d): check your token", resp.StatusCode)
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("GitHub API error (HTTP %d)", resp.StatusCode)
		}

		var page []ghRepo
		if err := json.Unmarshal(bodyBytes, &page); err != nil {
			return nil, fmt.Errorf("decoding response: %w", err)
		}

		for _, r := range page {
			repos = append(repos, model.RepoInfo{
				Name:          r.Name,
				CloneURL:      r.CloneURL,
				DefaultBranch: r.DefaultBranch,
				IsPrivate:     r.Private,
				IsArchived:    r.Archived,
			})
		}

		url = nextLink(resp.Header.Get("Link"))
		if url != "" {
			c.verbosefSafe("api pagination: next=%s", sanitizeRequestURL(url))
		}
	}

	return repos, nil
}

func (c *Client) verbosefSafe(format string, args ...any) {
	if c.verbosef == nil {
		return
	}
	c.verbosef(format, args...)
}

func (c *Client) tracefSafe(format string, args ...any) {
	if c.tracef == nil {
		return
	}
	c.tracef(format, args...)
}

func sanitizeRequestURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}

	q := parsed.Query()
	redacted := false
	// Redact common auth-related query parameter names to avoid leaking secrets
	// in verbose logs when users run against custom API gateways or proxies.
	for _, key := range []string{"access_token", "token", "auth", "authorization"} {
		if q.Has(key) {
			q.Set(key, "[REDACTED]")
			redacted = true
		}
	}
	if redacted {
		parsed.RawQuery = q.Encode()
	}

	return parsed.String()
}

// ListOrgRepos lists all repositories for the given organisation.
func (c *Client) ListOrgRepos(org string) ([]model.RepoInfo, error) {
	url := fmt.Sprintf("https://api.github.com/orgs/%s/repos?per_page=100&page=1", org)
	return c.listRepos(url)
}

// ListUserRepos lists all public repositories for the given user account.
// Use ListOwnRepos to fetch all repositories (including private) for the authenticated user.
func (c *Client) ListUserRepos(username string) ([]model.RepoInfo, error) {
	url := fmt.Sprintf("https://api.github.com/users/%s/repos?per_page=100&page=1", username)
	return c.listRepos(url)
}

// ghUser is the JSON shape returned by the GitHub /user API.
type ghUser struct {
	Login string `json:"login"`
}

// GetAuthenticatedUser returns the login of the authenticated token owner.
// The result is cached after the first call. Safe for concurrent use.
func (c *Client) GetAuthenticatedUser() (string, error) {
	c.authUserOnce.Do(func() {
		const apiURL = "https://api.github.com/user"
		req, err := http.NewRequest("GET", apiURL, nil)
		if err != nil {
			c.authUserErr = fmt.Errorf("creating request: %w", err)
			return
		}
		req.Header.Set("Accept", "application/vnd.github+json")
		if c.token != "" {
			req.Header.Set("Authorization", "Bearer "+c.token)
		}
		c.verbosefSafe("api request: %s %s headers={Accept:%q Authorization:%t}",
			req.Method, apiURL, req.Header.Get("Accept"), c.token != "")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			c.authUserErr = fmt.Errorf("requesting user: %w", err)
			return
		}
		c.verbosefSafe("api response: %s %s status=%d", req.Method, apiURL, resp.StatusCode)

		bodyBytes, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			c.authUserErr = fmt.Errorf("reading response body: %w", err)
			return
		}
		c.tracefSafe("api body: %s", bytes.TrimSpace(bodyBytes))

		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			c.authUserErr = fmt.Errorf("GitHub API auth error (HTTP %d): check your token", resp.StatusCode)
			return
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			c.authUserErr = fmt.Errorf("GitHub API error (HTTP %d)", resp.StatusCode)
			return
		}

		var u ghUser
		if err := json.Unmarshal(bodyBytes, &u); err != nil {
			c.authUserErr = fmt.Errorf("decoding response: %w", err)
			return
		}

		c.authUserLogin = u.Login
	})
	return c.authUserLogin, c.authUserErr
}

// ListOwnRepos lists all repositories (public and private) for the authenticated user.
func (c *Client) ListOwnRepos() ([]model.RepoInfo, error) {
	return c.listRepos("https://api.github.com/user/repos?per_page=100&page=1")
}

// nextLink parses the GitHub Link header and returns the URL for rel="next", or "".
func nextLink(header string) string {
	if header == "" {
		return ""
	}
	for part := range strings.SplitSeq(header, ",") {
		part = strings.TrimSpace(part)
		if strings.Contains(part, `rel="next"`) {
			// Extract URL between < and >
			start := strings.Index(part, "<")
			end := strings.Index(part, ">")
			if start >= 0 && end > start {
				return part[start+1 : end]
			}
		}
	}
	return ""
}

package github

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/UnitVectorY-Labs/ghorgsync/internal/model"
)

// Client wraps GitHub API access.
type Client struct {
	token      string
	httpClient *http.Client
}

// NewClient creates a new GitHub API client.
// It resolves a token from GITHUB_TOKEN, GH_TOKEN env vars, or gh CLI auth.
func NewClient(token string) *Client {
	return &Client{
		token: token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
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
}

// ListOrgRepos lists all repositories for the given organisation.
func (c *Client) ListOrgRepos(org string) ([]model.RepoInfo, error) {
	var repos []model.RepoInfo
	url := fmt.Sprintf("https://api.github.com/orgs/%s/repos?per_page=100&page=1", org)

	for url != "" {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("creating request: %w", err)
		}
		req.Header.Set("Accept", "application/vnd.github+json")
		if c.token != "" {
			req.Header.Set("Authorization", "Bearer "+c.token)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("requesting repos: %w", err)
		}

		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			resp.Body.Close()
			return nil, fmt.Errorf("GitHub API auth error (HTTP %d): check your token", resp.StatusCode)
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			resp.Body.Close()
			return nil, fmt.Errorf("GitHub API error (HTTP %d)", resp.StatusCode)
		}

		var page []ghRepo
		if err := json.NewDecoder(resp.Body).Decode(&page); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("decoding response: %w", err)
		}
		resp.Body.Close()

		for _, r := range page {
			repos = append(repos, model.RepoInfo{
				Name:          r.Name,
				CloneURL:      r.CloneURL,
				DefaultBranch: r.DefaultBranch,
				IsPrivate:     r.Private,
			})
		}

		url = nextLink(resp.Header.Get("Link"))
	}

	return repos, nil
}

// nextLink parses the GitHub Link header and returns the URL for rel="next", or "".
func nextLink(header string) string {
	if header == "" {
		return ""
	}
	for _, part := range strings.Split(header, ",") {
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

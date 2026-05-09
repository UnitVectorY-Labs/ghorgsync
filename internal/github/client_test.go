package github

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestListRepos_VerboseLogsRequestsAndResponses(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer super-secret-token" {
			t.Fatalf("unexpected Authorization header: %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `[{"name":"repo-one","clone_url":"https://github.com/acme/repo-one.git","default_branch":"main","private":false,"archived":false}]`)
	}))
	defer server.Close()

	var logs []string
	client := NewClient("super-secret-token", func(format string, args ...interface{}) {
		logs = append(logs, fmt.Sprintf(format, args...))
	})
	client.httpClient = server.Client()

	repos, err := client.listRepos(server.URL + "?per_page=100&page=1")
	if err != nil {
		t.Fatalf("listRepos returned error: %v", err)
	}
	if len(repos) != 1 {
		t.Fatalf("expected 1 repo, got %d", len(repos))
	}
	if repos[0].Name != "repo-one" {
		t.Fatalf("unexpected repo name: %q", repos[0].Name)
	}

	joined := strings.Join(logs, "\n")
	if !strings.Contains(joined, "api request: GET "+server.URL+"?page=1&per_page=100") {
		t.Fatalf("missing request verbose log: %s", joined)
	}
	if !strings.Contains(joined, "Authorization:true") {
		t.Fatalf("missing auth-presence indicator in logs: %s", joined)
	}
	if !strings.Contains(joined, "api response: GET "+server.URL+"?page=1&per_page=100 status=200") {
		t.Fatalf("missing response verbose log: %s", joined)
	}
	if strings.Contains(joined, "super-secret-token") {
		t.Fatalf("token leaked in verbose logs: %s", joined)
	}
}

func TestSanitizeRequestURL_RedactsSensitiveQueryValues(t *testing.T) {
	url := "https://api.github.com/orgs/acme/repos?page=2&access_token=abc123&token=def456&other=value"
	sanitized := sanitizeRequestURL(url)

	if strings.Contains(sanitized, "abc123") || strings.Contains(sanitized, "def456") {
		t.Fatalf("sensitive query values leaked: %s", sanitized)
	}
	if !strings.Contains(sanitized, "access_token=%5BREDACTED%5D") {
		t.Fatalf("expected access_token to be redacted, got: %s", sanitized)
	}
	if !strings.Contains(sanitized, "token=%5BREDACTED%5D") {
		t.Fatalf("expected token to be redacted, got: %s", sanitized)
	}
}

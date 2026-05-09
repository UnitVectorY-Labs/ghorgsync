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
	client := NewClient("super-secret-token",
		func(format string, args ...interface{}) {
			logs = append(logs, fmt.Sprintf(format, args...))
		},
		nil, // no trace logger for this test
	)
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
	if !strings.Contains(joined, "api request: GET "+server.URL+"?") ||
		!strings.Contains(joined, "per_page=100") ||
		!strings.Contains(joined, "page=1") {
		t.Fatalf("missing request verbose log: %s", joined)
	}
	if !strings.Contains(joined, "Authorization:true") {
		t.Fatalf("missing auth-presence indicator in logs: %s", joined)
	}
	if !strings.Contains(joined, "api response: GET "+server.URL+"?") ||
		!strings.Contains(joined, "status=200") {
		t.Fatalf("missing response verbose log: %s", joined)
	}
	if strings.Contains(joined, "super-secret-token") {
		t.Fatalf("token leaked in verbose logs: %s", joined)
	}
}

func TestListRepos_TraceLogsResponseBody(t *testing.T) {
	repoJSON := `[{"name":"trace-repo","clone_url":"https://github.com/acme/trace-repo.git","default_branch":"main","private":false,"archived":false}]`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, repoJSON)
	}))
	defer server.Close()

	var traceLogs []string
	client := NewClient("token",
		func(format string, args ...interface{}) {}, // level-1 logger (discarded)
		func(format string, args ...interface{}) {
			traceLogs = append(traceLogs, fmt.Sprintf(format, args...))
		},
	)
	client.httpClient = server.Client()

	repos, err := client.listRepos(server.URL + "?per_page=100&page=1")
	if err != nil {
		t.Fatalf("listRepos returned error: %v", err)
	}
	if len(repos) != 1 || repos[0].Name != "trace-repo" {
		t.Fatalf("unexpected repos: %+v", repos)
	}

	joined := strings.Join(traceLogs, "\n")
	if !strings.Contains(joined, "api body:") {
		t.Fatalf("expected 'api body:' prefix in trace logs, got: %s", joined)
	}
	if !strings.Contains(joined, "trace-repo") {
		t.Fatalf("expected response body content in trace logs, got: %s", joined)
	}
}

func TestListRepos_NoTraceLogger_BodyNotLogged(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `[{"name":"repo-x","clone_url":"https://github.com/acme/repo-x.git","default_branch":"main","private":false,"archived":false}]`)
	}))
	defer server.Close()

	var logs []string
	// Pass nil tracef — body must not appear in level-1 logs
	client := NewClient("token",
		func(format string, args ...interface{}) {
			logs = append(logs, fmt.Sprintf(format, args...))
		},
		nil,
	)
	client.httpClient = server.Client()

	_, err := client.listRepos(server.URL + "?per_page=100&page=1")
	if err != nil {
		t.Fatalf("listRepos returned error: %v", err)
	}
	joined := strings.Join(logs, "\n")
	if strings.Contains(joined, "api body:") {
		t.Fatalf("api body should not appear in level-1 logs when tracef is nil, got: %s", joined)
	}
}

func TestSanitizeRequestURL_RedactsSensitiveQueryValues(t *testing.T) {
	url := "https://api.github.com/orgs/acme/repos?page=2&access_token=abc123&token=super-secret-token&other=value"
	sanitized := sanitizeRequestURL(url)

	if strings.Contains(sanitized, "abc123") || strings.Contains(sanitized, "super-secret-token") {
		t.Fatalf("sensitive query values leaked: %s", sanitized)
	}
	if !strings.Contains(sanitized, "access_token=%5BREDACTED%5D") {
		t.Fatalf("expected access_token to be redacted, got: %s", sanitized)
	}
	if !strings.Contains(sanitized, "token=%5BREDACTED%5D") {
		t.Fatalf("expected token to be redacted, got: %s", sanitized)
	}
}

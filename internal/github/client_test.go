package github

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestListRepos_VerboseLogsRequestsAndResponses(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer super-secret-token" {
			t.Fatalf("unexpected Authorization header: %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `[{"name":"repo-one","full_name":"acme/repo-one","clone_url":"https://github.com/acme/repo-one.git","default_branch":"main","private":false,"archived":false,"size":10}]`)
	}))
	defer server.Close()

	var logs []string
	client := NewClient("super-secret-token",
		func(format string, args ...any) {
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
	repoJSON := `[{"name":"trace-repo","full_name":"acme/trace-repo","clone_url":"https://github.com/acme/trace-repo.git","default_branch":"main","private":false,"archived":false,"size":10}]`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, repoJSON)
	}))
	defer server.Close()

	var traceLogs []string
	client := NewClient("token",
		func(format string, args ...any) {}, // level-1 logger (discarded)
		func(format string, args ...any) {
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
		fmt.Fprintln(w, `[{"name":"repo-x","full_name":"acme/repo-x","clone_url":"https://github.com/acme/repo-x.git","default_branch":"main","private":false,"archived":false,"size":10}]`)
	}))
	defer server.Close()

	var logs []string
	// Pass nil tracef — body must not appear in level-1 logs
	client := NewClient("token",
		func(format string, args ...any) {
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

func TestListRepos_EmptyRepoDetection(t *testing.T) {
	var commitsCalls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/repos/acme/initial-commit/commits":
			commitsCalls = append(commitsCalls, r.URL.Path)
			fmt.Fprintln(w, `[{"sha":"a897946dc7e30b9e9a1d447c0ce25d6076a27cef"}]`)
		case "/repos/acme/never-pushed/commits":
			commitsCalls = append(commitsCalls, r.URL.Path)
			w.WriteHeader(http.StatusConflict)
			fmt.Fprintln(w, `{"message":"Git Repository is empty."}`)
		case "/repos/acme/populated/commits":
			t.Errorf("commits endpoint must not be called for non-zero-size repos, got %s", r.URL.Path)
		default:
			fmt.Fprintln(w, `[
				{"name":"populated","full_name":"acme/populated","clone_url":"https://github.com/acme/populated.git","default_branch":"main","private":false,"archived":false,"size":1234},
				{"name":"initial-commit","full_name":"acme/initial-commit","clone_url":"https://github.com/acme/initial-commit.git","default_branch":"main","private":false,"archived":false,"size":0},
				{"name":"never-pushed","full_name":"acme/never-pushed","clone_url":"https://github.com/acme/never-pushed.git","default_branch":"main","private":false,"archived":false,"size":0}
			]`)
		}
	}))
	defer server.Close()

	client := NewClient("token", nil, nil)
	client.httpClient = &http.Client{Transport: rewriteHostTransport{target: server.URL}}

	repos, err := client.listRepos(server.URL)
	if err != nil {
		t.Fatalf("listRepos returned error: %v", err)
	}
	if len(repos) != 3 {
		t.Fatalf("expected 3 repos, got %d", len(repos))
	}
	if repos[0].IsEmpty {
		t.Fatal("expected non-zero-size repository not to be empty")
	}
	if repos[1].IsEmpty {
		t.Fatal("expected zero-size repository with an initial commit not to be empty")
	}
	if !repos[2].IsEmpty {
		t.Fatal("expected zero-size repository with no commits to be empty")
	}
	wantCalls := []string{"/repos/acme/initial-commit/commits", "/repos/acme/never-pushed/commits"}
	if len(commitsCalls) != len(wantCalls) {
		t.Fatalf("expected commits calls %v, got %v", wantCalls, commitsCalls)
	}
	for i := range wantCalls {
		if commitsCalls[i] != wantCalls[i] {
			t.Fatalf("expected commits calls %v, got %v", wantCalls, commitsCalls)
		}
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

func TestGetAuthenticatedUser_ReturnsLogin(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/user" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"login":"octocat","id":1}`)
	}))
	defer server.Close()

	client := NewClient("test-token", nil, nil)
	client.httpClient = &http.Client{Transport: rewriteHostTransport{target: server.URL}}

	login, err := client.GetAuthenticatedUser()
	if err != nil {
		t.Fatalf("GetAuthenticatedUser returned error: %v", err)
	}
	if login != "octocat" {
		t.Fatalf("expected login %q, got %q", "octocat", login)
	}
}

func TestGetAuthenticatedUser_CachesResult(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"login":"octocat"}`)
	}))
	defer server.Close()

	client := NewClient("token", nil, nil)
	client.httpClient = &http.Client{Transport: rewriteHostTransport{target: server.URL}}

	for i := 0; i < 3; i++ {
		login, err := client.GetAuthenticatedUser()
		if err != nil {
			t.Fatalf("call %d: GetAuthenticatedUser returned error: %v", i, err)
		}
		if login != "octocat" {
			t.Fatalf("call %d: expected login %q, got %q", i, "octocat", login)
		}
	}
	if callCount != 1 {
		t.Fatalf("expected exactly 1 HTTP call, got %d", callCount)
	}
}

func TestGetAuthenticatedUser_AuthError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprintln(w, `{"message":"Bad credentials"}`)
	}))
	defer server.Close()

	client := NewClient("bad-token", nil, nil)
	client.httpClient = &http.Client{Transport: rewriteHostTransport{target: server.URL}}

	_, err := client.GetAuthenticatedUser()
	if err == nil {
		t.Fatal("expected error for 401 response, got nil")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Fatalf("expected error to mention 401, got: %v", err)
	}
}

func TestListOwnRepos_ReturnsAllRepos(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/user/repos" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `[{"name":"private-repo","full_name":"octocat/private-repo","clone_url":"https://github.com/octocat/private-repo.git","default_branch":"main","private":true,"archived":false,"size":10}]`)
	}))
	defer server.Close()

	client := NewClient("token", nil, nil)
	client.httpClient = &http.Client{Transport: rewriteHostTransport{target: server.URL}}

	repos, err := client.ListOwnRepos()
	if err != nil {
		t.Fatalf("ListOwnRepos returned error: %v", err)
	}
	if len(repos) != 1 {
		t.Fatalf("expected 1 repo, got %d", len(repos))
	}
	if repos[0].Name != "private-repo" {
		t.Fatalf("unexpected repo name: %q", repos[0].Name)
	}
	if !repos[0].IsPrivate {
		t.Fatal("expected repo to be private")
	}
}

// rewriteHostTransport redirects all HTTP requests to a fixed target base URL.
// This lets tests exercise methods that use hardcoded API URLs (e.g. https://api.github.com/user).
type rewriteHostTransport struct {
	target string // e.g. "http://127.0.0.1:PORT"
}

func (t rewriteHostTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	parsed, err := url.Parse(t.target)
	if err != nil {
		return nil, fmt.Errorf("rewriteHostTransport: invalid target %q: %w", t.target, err)
	}
	cloned := req.Clone(req.Context())
	cloned.URL.Scheme = parsed.Scheme
	cloned.URL.Host = parsed.Host
	cloned.Host = parsed.Host
	return http.DefaultTransport.RoundTrip(cloned)
}

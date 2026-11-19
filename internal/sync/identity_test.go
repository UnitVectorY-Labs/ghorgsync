package sync

import (
	"errors"
	"strings"
	"testing"

	"github.com/UnitVectorY-Labs/ghorgsync/internal/model"
)

func TestIdentityResult(t *testing.T) {
	readErr := errors.New("origin is missing or unreadable")
	repo := model.RepoInfo{Name: "project", DefaultBranch: "main", CloneURL: "https://github.com/acme/project.git"}
	for _, tc := range []struct {
		name        string
		remote      string
		readErr     error
		wantMessage string
	}{
		{name: "matching HTTPS", remote: repo.CloneURL},
		{name: "matching SSH", remote: "git@github.com:ACME/project.git"},
		{name: "different owner", remote: "https://github.com/other/project.git", wantMessage: "origin does not match GitHub repository acme/project"},
		{name: "different repository", remote: "https://github.com/acme/other.git", wantMessage: "origin does not match GitHub repository acme/project"},
		{name: "empty origin", wantMessage: "origin does not match GitHub repository acme/project"},
		{name: "unreadable origin", readErr: readErr, wantMessage: "cannot verify origin"},
		{name: "credentials in origin", remote: "https://secret-token@github.com/other/project.git", wantMessage: "origin does not match GitHub repository acme/project"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := identityResult(repo, tc.remote, tc.readErr)
			if result.Name != repo.Name || result.DefaultBranch != repo.DefaultBranch {
				t.Fatalf("lost repository context: %+v", result)
			}
			if tc.wantMessage == "" {
				if result.Error != nil || result.Action != model.ActionNone {
					t.Fatalf("matching origin rejected: %+v", result)
				}
				return
			}
			if result.Action != model.ActionIdentityError || result.Error == nil {
				t.Fatalf("unsafe origin did not produce identity error: %+v", result)
			}
			message := result.Error.Error()
			if !strings.Contains(message, tc.wantMessage) || !strings.Contains(message, "repository left untouched") {
				t.Fatalf("unexpected diagnostic: %s", message)
			}
			if strings.Contains(message, "secret-token") || (tc.remote != "" && strings.Contains(message, tc.remote)) {
				t.Fatalf("actual origin leaked: %s", message)
			}
			if tc.readErr != nil && !errors.Is(result.Error, tc.readErr) {
				t.Fatal("origin inspection error was not preserved")
			}
		})
	}
}

func TestIdentityResultUnverifiableMetadataDoesNotLeakCredentials(t *testing.T) {
	repo := model.RepoInfo{Name: "project", CloneURL: "https://metadata-secret@github.com/acme/project.git"}
	result := identityResult(repo, "https://github.com/acme/project.git", nil)
	if result.Action != model.ActionIdentityError || result.Error == nil {
		t.Fatalf("invalid metadata accepted: %+v", result)
	}
	if strings.Contains(result.Error.Error(), "metadata-secret") {
		t.Fatal("metadata credentials leaked")
	}
}

package model

import "testing"

func TestLocalClassificationString(t *testing.T) {
	tests := []struct {
		c    LocalClassification
		want string
	}{
		{ClassManaged, "managed"},
		{ClassUnknown, "unknown"},
		{ClassExcludedButPresent, "excluded-but-present"},
		{ClassCollision, "collision"},
		{LocalClassification(99), "unknown"},
	}
	for _, tt := range tests {
		if got := tt.c.String(); got != tt.want {
			t.Errorf("LocalClassification(%d).String() = %q, want %q", int(tt.c), got, tt.want)
		}
	}
}

func TestRepoActionString(t *testing.T) {
	tests := []struct {
		a    RepoAction
		want string
	}{
		{ActionNone, "none"},
		{ActionCloned, "cloned"},
		{ActionUpdated, "updated"},
		{ActionAlreadyCurrent, "up-to-date"},
		{ActionDirty, "dirty"},
		{ActionBranchDrift, "branch-drift"},
		{ActionCloneError, "clone-error"},
		{ActionFetchError, "fetch-error"},
		{ActionCheckoutError, "checkout-error"},
		{ActionPullError, "pull-error"},
		{ActionSubmoduleError, "submodule-error"},
		{RepoAction(99), "unknown"},
	}
	for _, tt := range tests {
		if got := tt.a.String(); got != tt.want {
			t.Errorf("RepoAction(%d).String() = %q, want %q", int(tt.a), got, tt.want)
		}
	}
}

package gitidentity

import "testing"

func TestMatchRemote(t *testing.T) {
	expected := "https://github.com/acme/project.git"
	for _, tc := range []struct {
		remote string
		want   bool
	}{
		{expected, true}, {"git@github.com:acme/project.git", true},
		{"ssh://git@github.com/Acme/PROJECT", true},
		{"https://github.com/other/project.git", false},
		{"https://github.com/acme/other.git", false},
		{"https://example.com/acme/project.git", false},
		{"/tmp/project", false}, {"", false},
		{"https://github.com/acme/project.git?x=y", false},
		{"https://github.com/acme/project/extra", false},
	} {
		if got := MatchRemote(tc.remote, expected); got != tc.want {
			t.Errorf("MatchRemote(%q)=%t, want %t", tc.remote, got, tc.want)
		}
	}
}

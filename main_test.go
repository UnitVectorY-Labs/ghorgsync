package main

import (
	"runtime"
	"strings"
	"testing"
)

func TestNormalizedVersion(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		expects string
	}{
		{name: "dev unchanged", input: "dev", expects: "dev"},
		{name: "empty unchanged", input: "", expects: ""},
		{name: "prefixed unchanged", input: "v1.2.3", expects: "v1.2.3"},
		{name: "adds prefix", input: "1.2.3", expects: "v1.2.3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizedVersion(tt.input)
			if got != tt.expects {
				t.Fatalf("normalizedVersion(%q) = %q, want %q", tt.input, got, tt.expects)
			}
		})
	}
}

func TestVersionString(t *testing.T) {
	got := versionString("1.2.3")

	if !strings.HasPrefix(got, "ghorgsync version v1.2.3 (") {
		t.Fatalf("versionString prefix mismatch: %q", got)
	}

	if !strings.Contains(got, runtime.Version()) {
		t.Fatalf("versionString should include Go version %q: %q", runtime.Version(), got)
	}

	platform := runtime.GOOS + "/" + runtime.GOARCH
	if !strings.Contains(got, platform) {
		t.Fatalf("versionString should include platform %q: %q", platform, got)
	}

	if !strings.HasSuffix(got, ")") {
		t.Fatalf("versionString should end with ')': %q", got)
	}
}

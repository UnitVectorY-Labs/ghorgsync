package output

import (
	"strings"
	"testing"
)

func TestFormatSummaryLine(t *testing.T) {
	line := FormatSummaryLine(10, 2, 3, 1, 1, 2, 1, 0)
	if !strings.Contains(line, "total: 10") {
		t.Error("should contain total")
	}
	if !strings.Contains(line, "cloned: 2") {
		t.Error("should contain cloned")
	}
	if !strings.Contains(line, "dirty: 1") {
		t.Error("should contain dirty")
	}
	if !strings.Contains(line, "errors: 0") {
		t.Error("should contain errors")
	}
}

func TestFormatSummaryLine_AllZeros(t *testing.T) {
	line := FormatSummaryLine(0, 0, 0, 0, 0, 0, 0, 0)
	if !strings.Contains(line, "total: 0") {
		t.Error("should contain total: 0")
	}
}

func TestFormatStatusLabel(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"cloned", "[cloned]"},
		{"dirty", "[dirty]"},
		{"updated", "[updated]"},
	}
	for _, tt := range tests {
		if got := FormatStatusLabel(tt.input); got != tt.expected {
			t.Errorf("FormatStatusLabel(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestShouldColor_WithNoColorEnv(t *testing.T) {
	// When NO_COLOR is set, ShouldColor should return false
	t.Setenv("NO_COLOR", "1")
	if ShouldColor() {
		t.Error("should return false when NO_COLOR is set")
	}
}

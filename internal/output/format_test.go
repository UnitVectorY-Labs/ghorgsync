package output

import (
	"os"
	"strings"
	"testing"
)

func TestFormatSummaryLine(t *testing.T) {
	line := FormatSummaryLine(10, 2, 3, 1, 1, 2, 1, 0, 0)
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
	line := FormatSummaryLine(0, 0, 0, 0, 0, 0, 0, 0, 0)
	if !strings.Contains(line, "total: 0") {
		t.Error("should contain total: 0")
	}
}

func TestFormatSummaryLine_WithEmpty(t *testing.T) {
	line := FormatSummaryLine(10, 2, 3, 1, 1, 2, 1, 0, 3)
	if !strings.Contains(line, "empty: 3") {
		t.Error("should contain empty: 3")
	}
}

func TestFormatSummaryLine_NoEmptyWhenZero(t *testing.T) {
	line := FormatSummaryLine(10, 2, 3, 1, 1, 2, 1, 0, 0)
	if strings.Contains(line, "empty") {
		t.Error("should not contain empty when count is 0")
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

func TestStatusOutputColorAndErrorSummary(t *testing.T) {
	for _, color := range []bool{false, true} {
		t.Run(map[bool]string{false: "plain", true: "color"}[color], func(t *testing.T) {
			file, err := os.CreateTemp(t.TempDir(), "output")
			if err != nil {
				t.Fatal(err)
			}
			original := os.Stdout
			os.Stdout = file
			defer func() { os.Stdout = original; file.Close() }()
			p := NewPrinter(color, 0, true)
			p.RepoStatusDirty("app", "topic", "main", "\x1b[31m M\x1b[m app.go\n")
			p.StatusSummary(2, 1, 0, 1, 3)
			data, err := os.ReadFile(file.Name())
			if err != nil {
				t.Fatal(err)
			}
			out := string(data)
			if strings.Contains(out, "\x1b[") != color {
				t.Errorf("unexpected color output: %q", out)
			}
			for _, want := range []string{"app.go", "default: main", "total: 2", "errors: 1", "empty: 3"} {
				if !strings.Contains(out, want) {
					t.Errorf("output missing %q: %q", want, out)
				}
			}
		})
	}
}

package output

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func newTestPrinter() *Printer {
	return &Printer{color: false, verbose: false, interactive: false}
}

func TestDigitCount(t *testing.T) {
	tests := []struct {
		input    int
		expected int
	}{
		{0, 1},
		{1, 1},
		{9, 1},
		{10, 2},
		{99, 2},
		{100, 3},
		{999, 3},
		{1000, 4},
		{-5, 1},
	}
	for _, tt := range tests {
		if got := digitCount(tt.input); got != tt.expected {
			t.Errorf("digitCount(%d) = %d, want %d", tt.input, got, tt.expected)
		}
	}
}

func TestRenderProgressLine_ZeroPercent(t *testing.T) {
	p := newTestPrinter()
	p.repoProgress = repoProgressState{
		active:     true,
		total:      10,
		current:    0,
		totalWidth: 2,
	}

	line := p.renderProgressLine(80)

	// Should contain "repo" label
	if !strings.Contains(line, "repo") {
		t.Error("should contain 'repo' label")
	}
	// Should contain counter " 0/10"
	if !strings.Contains(line, " 0/10") {
		t.Errorf("should contain padded counter ' 0/10', got: %q", line)
	}
	// Should contain 0%
	if !strings.Contains(line, "0%") {
		t.Error("should contain 0%")
	}
	// Should contain brackets
	if !strings.Contains(line, "[") || !strings.Contains(line, "]") {
		t.Error("should contain brackets around progress bar")
	}
	// Should not contain any full blocks at 0%
	if strings.Contains(line, fullBlock) {
		t.Error("should not contain full blocks at 0%")
	}
}

func TestRenderProgressLine_HundredPercent(t *testing.T) {
	p := newTestPrinter()
	p.repoProgress = repoProgressState{
		active:     true,
		total:      10,
		current:    10,
		totalWidth: 2,
	}

	line := p.renderProgressLine(80)

	// Should contain "10/10"
	if !strings.Contains(line, "10/10") {
		t.Errorf("should contain '10/10', got: %q", line)
	}
	// Should contain 100%
	if !strings.Contains(line, "100%") {
		t.Error("should contain 100%")
	}
	// Should contain only full blocks in the bar (no spaces between brackets)
	bracketStart := strings.Index(line, "[")
	bracketEnd := strings.Index(line, "]")
	if bracketStart < 0 || bracketEnd < 0 {
		t.Fatal("missing brackets")
	}
	barContent := line[bracketStart+1 : bracketEnd]
	if strings.Contains(barContent, " ") {
		t.Errorf("at 100%%, bar should have no spaces, got: %q", barContent)
	}
}

func TestRenderProgressLine_MidProgress(t *testing.T) {
	p := newTestPrinter()
	p.repoProgress = repoProgressState{
		active:     true,
		total:      10,
		current:    5,
		totalWidth: 2,
	}

	line := p.renderProgressLine(80)

	// Should contain " 5/10"
	if !strings.Contains(line, " 5/10") {
		t.Errorf("should contain ' 5/10', got: %q", line)
	}
	// Should contain 50%
	if !strings.Contains(line, "50%") {
		t.Error("should contain 50%")
	}
	// Bar should have both filled blocks and spaces
	bracketStart := strings.Index(line, "[")
	bracketEnd := strings.Index(line, "]")
	if bracketStart < 0 || bracketEnd < 0 {
		t.Fatal("missing brackets")
	}
	barContent := line[bracketStart+1 : bracketEnd]
	if !strings.Contains(barContent, fullBlock) {
		t.Error("50% progress should have some full blocks")
	}
	if !strings.Contains(barContent, " ") {
		t.Error("50% progress should have some empty space")
	}
}

func TestRenderProgressLine_PaddedCurrentWidth(t *testing.T) {
	p := newTestPrinter()
	p.repoProgress = repoProgressState{
		active:     true,
		total:      100,
		current:    3,
		totalWidth: 3,
	}

	line := p.renderProgressLine(80)

	// current should be padded to 3 digits width: "  3/100"
	if !strings.Contains(line, "  3/100") {
		t.Errorf("should contain '  3/100' (padded to 3 digits), got: %q", line)
	}
}

func TestRenderProgressLine_ConsistentWidthAcrossProgress(t *testing.T) {
	p := newTestPrinter()
	p.repoProgress = repoProgressState{
		active:     true,
		total:      100,
		current:    0,
		totalWidth: 3,
	}

	// Collect display widths (rune counts) at different progress points
	widths := make(map[int]bool)
	for i := 0; i <= 100; i++ {
		p.repoProgress.current = i
		line := p.renderProgressLine(80)
		widths[utf8.RuneCountInString(line)] = true
	}

	// All lines should have the same display width
	if len(widths) != 1 {
		t.Errorf("progress lines should have consistent display width, got %d different widths", len(widths))
	}
}

func TestRenderProgressLine_ScalesToTermWidth(t *testing.T) {
	p := newTestPrinter()
	p.repoProgress = repoProgressState{
		active:     true,
		total:      10,
		current:    5,
		totalWidth: 2,
	}

	line80 := p.renderProgressLine(80)
	line120 := p.renderProgressLine(120)

	// Wider terminal should produce a longer line
	if len(line120) <= len(line80) {
		t.Errorf("120-col line (%d) should be longer than 80-col line (%d)", len(line120), len(line80))
	}
}

func TestRenderProgressLine_NarrowTerminal(t *testing.T) {
	p := newTestPrinter()
	p.repoProgress = repoProgressState{
		active:     true,
		total:      10,
		current:    5,
		totalWidth: 2,
	}

	// Very narrow terminal should still produce valid output (min bar width enforced)
	line := p.renderProgressLine(20)
	if !strings.Contains(line, "repo") {
		t.Error("narrow terminal should still contain 'repo' label")
	}
	if !strings.Contains(line, "50%") {
		t.Error("narrow terminal should still contain percentage")
	}
}

func TestRenderProgressLine_SmoothPartialBlocks(t *testing.T) {
	p := newTestPrinter()
	p.repoProgress = repoProgressState{
		active:     true,
		total:      100,
		current:    0,
		totalWidth: 3,
	}

	// Advance one by one and collect all partial block characters used
	partials := make(map[string]bool)
	for i := 1; i < 100; i++ {
		p.repoProgress.current = i
		line := p.renderProgressLine(80)
		bracketStart := strings.Index(line, "[")
		bracketEnd := strings.Index(line, "]")
		if bracketStart < 0 || bracketEnd < 0 {
			continue
		}
		barContent := line[bracketStart+1 : bracketEnd]
		for _, r := range barContent {
			s := string(r)
			if s != fullBlock && s != " " {
				partials[s] = true
			}
		}
	}

	// With 100 steps over a wide bar, we should see at least some partial blocks
	if len(partials) == 0 {
		t.Error("expected partial block characters to appear for smooth progress")
	}
}

func TestRenderProgressLine_SingleRepo(t *testing.T) {
	p := newTestPrinter()
	p.repoProgress = repoProgressState{
		active:     true,
		total:      1,
		current:    0,
		totalWidth: 1,
	}

	line0 := p.renderProgressLine(80)
	if !strings.Contains(line0, "0/1") {
		t.Errorf("should contain '0/1', got: %q", line0)
	}

	p.repoProgress.current = 1
	line1 := p.renderProgressLine(80)
	if !strings.Contains(line1, "1/1") {
		t.Errorf("should contain '1/1', got: %q", line1)
	}
	if !strings.Contains(line1, "100%") {
		t.Error("1/1 should show 100%")
	}
}

func TestRenderProgressLine_Layout(t *testing.T) {
	p := newTestPrinter()
	p.repoProgress = repoProgressState{
		active:     true,
		total:      10,
		current:    3,
		totalWidth: 2,
	}

	line := p.renderProgressLine(80)

	// Verify overall layout order: "repo" comes before counter, counter before "[", "[" before "]", "]" before "%"
	repoIdx := strings.Index(line, "repo")
	counterIdx := strings.Index(line, "3/10")
	bracketOpen := strings.Index(line, "[")
	bracketClose := strings.Index(line, "]")
	pctIdx := strings.Index(line, "30%")

	if repoIdx < 0 || counterIdx < 0 || bracketOpen < 0 || bracketClose < 0 || pctIdx < 0 {
		t.Fatalf("missing expected elements in: %q", line)
	}
	if !(repoIdx < counterIdx && counterIdx < bracketOpen && bracketOpen < bracketClose && bracketClose < pctIdx) {
		t.Errorf("layout order should be: repo < counter < [ < ] < %%, got positions: repo=%d counter=%d [=%d ]=%d %%=%d",
			repoIdx, counterIdx, bracketOpen, bracketClose, pctIdx)
	}
}

package sync

import (
	"strings"

	"github.com/UnitVectorY-Labs/ghorgsync/internal/model"
)

// Decision represents what actions should be taken for a repository.
type Decision struct {
	ShouldFetch    bool
	ShouldCheckout bool
	ShouldPull     bool
	SkipReason     string
}

// DecideActions determines what git operations to perform based on repo state.
// This is a pure function for testability.
func DecideActions(isDirty bool, currentBranch string, defaultBranch string) Decision {
	d := Decision{
		ShouldFetch: true, // Always fetch
	}

	if isDirty {
		d.SkipReason = "working tree is dirty"
		return d
	}

	if currentBranch != defaultBranch {
		d.ShouldCheckout = true
	}
	d.ShouldPull = true
	return d
}

// ParseGitStatus parses git status --porcelain output into DirtyFile entries.
// This is a pure function for testability.
func ParseGitStatus(output string) []model.DirtyFile {
	if output == "" {
		return nil
	}
	var files []model.DirtyFile
	lines := splitLines(output)
	for _, line := range lines {
		if len(line) < 4 {
			continue
		}
		x := line[0]
		y := line[1]
		path := strings.TrimSpace(line[3:])

		f := model.DirtyFile{Path: path}
		if x != ' ' && x != '?' {
			f.Staged = true
		}
		if y != ' ' || x == '?' {
			f.Unstaged = true
		}
		files = append(files, f)
	}
	return files
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

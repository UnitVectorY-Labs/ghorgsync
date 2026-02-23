package output

import (
	"fmt"
	"os"
	"strings"
)

// ANSI color codes
const (
	reset   = "\033[0m"
	red     = "\033[31m"
	green   = "\033[32m"
	yellow  = "\033[33m"
	blue    = "\033[34m"
	magenta = "\033[35m"
	cyan    = "\033[36m"
	gray    = "\033[90m"
	bold    = "\033[1m"

	clearLine = "\r\033[2K"
)

const repoProgressBarWidth = 28

type repoProgressState struct {
	active  bool
	live    bool
	total   int
	current int
}

// Printer handles formatted output with optional color support.
type Printer struct {
	color        bool
	verbose      bool
	interactive  bool
	repoProgress repoProgressState
}

// NewPrinter creates a new Printer.
// color: whether to enable ANSI color output
// verbose: whether to show verbose output
func NewPrinter(color bool, verbose bool) *Printer {
	return &Printer{
		color:       color,
		verbose:     verbose,
		interactive: IsTerminalOutput(),
	}
}

// IsTerminalOutput returns true when stdout is a terminal.
func IsTerminalOutput() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// ShouldColor returns true if color output is enabled.
func ShouldColor() bool {
	// Honor NO_COLOR environment variable
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		return false
	}
	return IsTerminalOutput()
}

func (p *Printer) colorize(color, text string) string {
	if !p.color {
		return text
	}
	return color + text + reset
}

func (p *Printer) withProgressSuspended(fn func()) {
	redraw := p.clearLiveProgressLine()
	fn()
	if redraw {
		p.drawRepoProgressLine()
	}
}

func (p *Printer) clearLiveProgressLine() bool {
	if !p.repoProgress.active || !p.repoProgress.live {
		return false
	}
	fmt.Print(clearLine)
	return true
}

func (p *Printer) drawRepoProgressLine() {
	if !p.repoProgress.active || !p.repoProgress.live {
		return
	}
	fmt.Print(clearLine)
	fmt.Print(p.repoProgressLine())
}

func (p *Printer) repoProgressLine() string {
	total := p.repoProgress.total
	current := p.repoProgress.current

	if total <= 0 {
		total = 1
	}
	if current < 0 {
		current = 0
	}
	if current > total {
		current = total
	}

	percent := (current * 100) / total
	filled := (current * repoProgressBarWidth) / total
	if current == total {
		filled = repoProgressBarWidth
	}
	if filled > repoProgressBarWidth {
		filled = repoProgressBarWidth
	}

	remaining := repoProgressBarWidth - filled
	head := ""
	if current > 0 && current < total && remaining > 0 {
		head = ">"
		remaining--
	}

	var b strings.Builder
	b.WriteString("  ")
	b.WriteString(p.colorize(cyan, "progress"))
	b.WriteString(" ")
	b.WriteString(p.colorize(gray, "["))
	b.WriteString(p.colorize(green, strings.Repeat("=", filled)))
	if head != "" {
		b.WriteString(p.colorize(cyan, head))
	}
	b.WriteString(p.colorize(gray, strings.Repeat("-", remaining)))
	b.WriteString(p.colorize(gray, "]"))
	b.WriteString(" ")

	percentColor := cyan
	if percent >= 100 {
		percentColor = green
	} else if percent >= 60 {
		percentColor = yellow
	}
	b.WriteString(p.colorize(percentColor, fmt.Sprintf("%3d%%", percent)))
	b.WriteString(" ")
	b.WriteString(p.colorize(gray, fmt.Sprintf("%d/%d repos", current, total)))

	return b.String()
}

// StartRepoProgress starts a live progress line for repository processing.
func (p *Printer) StartRepoProgress(total int) {
	if total <= 0 {
		p.repoProgress = repoProgressState{}
		return
	}
	p.repoProgress = repoProgressState{
		active:  true,
		live:    p.interactive,
		total:   total,
		current: 0,
	}
	if p.repoProgress.live {
		p.drawRepoProgressLine()
	}
}

// AdvanceRepoProgress increments the repository progress bar by one.
func (p *Printer) AdvanceRepoProgress() {
	if !p.repoProgress.active {
		return
	}
	if p.repoProgress.current < p.repoProgress.total {
		p.repoProgress.current++
	}
	if p.repoProgress.live {
		p.drawRepoProgressLine()
	}
}

// FinishRepoProgress renders the completed progress line and moves to the next line.
func (p *Printer) FinishRepoProgress() {
	if !p.repoProgress.active {
		return
	}
	if p.repoProgress.current < p.repoProgress.total {
		p.repoProgress.current = p.repoProgress.total
	}
	if p.repoProgress.live {
		p.drawRepoProgressLine()
		fmt.Println()
	}
	p.repoProgress = repoProgressState{}
}

// Header prints a section header.
func (p *Printer) Header(text string) {
	p.withProgressSuspended(func() {
		fmt.Println(p.colorize(bold, text))
	})
}

// Verbose prints a message only in verbose mode.
func (p *Printer) Verbose(format string, args ...interface{}) {
	if !p.verbose {
		return
	}
	msg := fmt.Sprintf(format, args...)
	p.withProgressSuspended(func() {
		fmt.Println(p.colorize(gray, "  "+msg))
	})
}

// RepoCloned prints a clone action.
func (p *Printer) RepoCloned(name string) {
	p.withProgressSuspended(func() {
		fmt.Printf("  %s %s %s\n",
			p.colorize(cyan, "repo"),
			p.colorize(bold, name),
			p.colorize(green, "[cloned]"))
	})
}

// RepoUpdated prints an update action.
func (p *Printer) RepoUpdated(name string) {
	p.withProgressSuspended(func() {
		fmt.Printf("  %s %s %s\n",
			p.colorize(cyan, "repo"),
			p.colorize(bold, name),
			p.colorize(green, "[updated]"))
	})
}

// RepoBranchDrift prints a branch drift finding with checkout action.
func (p *Printer) RepoBranchDrift(name, fromBranch, toBranch string, updated bool) {
	status := "[branch-drift: checked out " + toBranch + "]"
	if updated {
		status = "[branch-drift: checked out " + toBranch + ", updated]"
	}
	p.withProgressSuspended(func() {
		fmt.Printf("  %s %s %s\n",
			p.colorize(cyan, "repo"),
			p.colorize(bold, name),
			p.colorize(yellow, status))
	})
}

// RepoDirty prints a dirty repo finding.
func (p *Printer) RepoDirty(name, currentBranch, defaultBranch string, files []DirtyFileInfo, additions, deletions int) {
	branchInfo := currentBranch
	if currentBranch != defaultBranch {
		branchInfo = currentBranch + " (default: " + defaultBranch + ")"
	}
	p.withProgressSuspended(func() {
		fmt.Printf("  %s %s %s on %s\n",
			p.colorize(cyan, "repo"),
			p.colorize(bold, name),
			p.colorize(yellow, "[dirty]"),
			branchInfo)
		fmt.Printf("       %s\n",
			p.colorize(yellow, "checkout/pull skipped due to dirty working tree"))

		// Print changed files
		for _, f := range files {
			label := ""
			if f.Staged && f.Unstaged {
				label = "staged+unstaged"
			} else if f.Staged {
				label = "staged"
			} else {
				label = "unstaged"
			}
			fmt.Printf("       %s %s\n",
				p.colorize(gray, "["+label+"]"),
				f.Path)
		}

		// Print line count summary
		if additions > 0 || deletions > 0 {
			fmt.Printf("       %s\n",
				p.colorize(gray, fmt.Sprintf("+%d -%d lines", additions, deletions)))
		}
	})
}

// RepoError prints a repo-level error.
func (p *Printer) RepoError(name, action string, err error) {
	p.withProgressSuspended(func() {
		fmt.Printf("  %s %s %s %s\n",
			p.colorize(cyan, "repo"),
			p.colorize(bold, name),
			p.colorize(red, "["+action+"]"),
			p.colorize(red, err.Error()))
	})
}

// UnknownFolder prints an unknown folder finding.
func (p *Printer) UnknownFolder(name string) {
	p.withProgressSuspended(func() {
		fmt.Printf("  %s %s %s\n",
			p.colorize(magenta, "folder"),
			p.colorize(bold, name),
			p.colorize(yellow, "[unknown]"))
	})
}

// ExcludedButPresent prints an excluded-but-present finding.
func (p *Printer) ExcludedButPresent(name string) {
	p.withProgressSuspended(func() {
		fmt.Printf("  %s %s %s\n",
			p.colorize(magenta, "folder"),
			p.colorize(bold, name),
			p.colorize(yellow, "[excluded-but-present]"))
	})
}

// Collision prints a path collision finding.
func (p *Printer) Collision(name, detail string) {
	p.withProgressSuspended(func() {
		fmt.Printf("  %s %s %s %s\n",
			p.colorize(cyan, "repo"),
			p.colorize(bold, name),
			p.colorize(red, "[collision]"),
			detail)
	})
}

// SystemError prints a system-level error.
func (p *Printer) SystemError(context string, err error) {
	p.withProgressSuspended(func() {
		fmt.Printf("  %s %s %s\n",
			p.colorize(red, "system"),
			p.colorize(bold, context),
			p.colorize(red, err.Error()))
	})
}

// DirtyFileInfo is a simple struct for passing to Printer.
type DirtyFileInfo struct {
	Path     string
	Staged   bool
	Unstaged bool
}

// Summary prints the final summary block.
func (p *Printer) Summary(total, cloned, updated, dirty, branchDrift, unknown, excluded, errors int) {
	p.withProgressSuspended(func() {
		fmt.Println()
		fmt.Println(p.colorize(bold, "Summary:"))

		parts := []string{
			fmt.Sprintf("total: %d", total),
		}

		if cloned > 0 {
			parts = append(parts, p.colorize(green, fmt.Sprintf("cloned: %d", cloned)))
		} else {
			parts = append(parts, fmt.Sprintf("cloned: %d", cloned))
		}
		if updated > 0 {
			parts = append(parts, p.colorize(green, fmt.Sprintf("updated: %d", updated)))
		} else {
			parts = append(parts, fmt.Sprintf("updated: %d", updated))
		}
		if dirty > 0 {
			parts = append(parts, p.colorize(yellow, fmt.Sprintf("dirty: %d", dirty)))
		} else {
			parts = append(parts, fmt.Sprintf("dirty: %d", dirty))
		}
		if branchDrift > 0 {
			parts = append(parts, p.colorize(yellow, fmt.Sprintf("branch-drift: %d", branchDrift)))
		} else {
			parts = append(parts, fmt.Sprintf("branch-drift: %d", branchDrift))
		}
		if unknown > 0 {
			parts = append(parts, p.colorize(yellow, fmt.Sprintf("unknown: %d", unknown)))
		} else {
			parts = append(parts, fmt.Sprintf("unknown: %d", unknown))
		}
		if excluded > 0 {
			parts = append(parts, p.colorize(yellow, fmt.Sprintf("excluded-but-present: %d", excluded)))
		} else {
			parts = append(parts, fmt.Sprintf("excluded-but-present: %d", excluded))
		}
		if errors > 0 {
			parts = append(parts, p.colorize(red, fmt.Sprintf("errors: %d", errors)))
		} else {
			parts = append(parts, fmt.Sprintf("errors: %d", errors))
		}

		fmt.Println("  " + strings.Join(parts, " | "))
	})
}

// ConfigError prints a configuration error message.
func (p *Printer) ConfigError(err error) {
	p.withProgressSuspended(func() {
		fmt.Printf("%s %s\n",
			p.colorize(red, "config error:"),
			err.Error())
	})
}

// MissingDotfile prints the message when .ghorgsync is not found.
func (p *Printer) MissingDotfile(filename string) {
	p.withProgressSuspended(func() {
		fmt.Printf("No %s configuration file found in current directory. Nothing to do.\n", filename)
	})
}

// AuthError prints an authentication error message.
func (p *Printer) AuthError(err error) {
	p.withProgressSuspended(func() {
		fmt.Printf("  %s %s %s\n",
			p.colorize(red, "system"),
			p.colorize(bold, "authentication"),
			p.colorize(red, err.Error()))
	})
}

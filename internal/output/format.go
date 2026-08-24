package output

import "fmt"

// FormatSummaryLine builds a plain-text summary line (for testing).
func FormatSummaryLine(total, cloned, updated, dirty, branchDrift, unknown, excluded, errors, empty int) string {
	line := fmt.Sprintf("total: %d | cloned: %d | updated: %d | dirty: %d | branch-drift: %d | unknown: %d | excluded-but-present: %d | errors: %d",
		total, cloned, updated, dirty, branchDrift, unknown, excluded, errors)
	if empty > 0 {
		line += fmt.Sprintf(" | empty: %d", empty)
	}
	return line
}

// FormatStatusLabel returns the text label for a repo action.
func FormatStatusLabel(action string) string {
	return "[" + action + "]"
}

package output

import "fmt"

// FormatSummaryLine builds a plain-text summary line (for testing).
func FormatSummaryLine(total, cloned, updated, dirty, branchDrift, unknown, excluded, errors int) string {
	return fmt.Sprintf("total: %d | cloned: %d | updated: %d | dirty: %d | branch-drift: %d | unknown: %d | excluded-but-present: %d | errors: %d",
		total, cloned, updated, dirty, branchDrift, unknown, excluded, errors)
}

// FormatStatusLabel returns the text label for a repo action.
func FormatStatusLabel(action string) string {
	return "[" + action + "]"
}

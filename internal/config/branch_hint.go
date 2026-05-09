package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// ResolveBranchHint returns the hinted default branch for repoDir, or "" when
// the hint cannot be resolved and callers should fall back to GitHub metadata.
func ResolveBranchHint(repoDir string, hint *BranchHint) string {
	if hint == nil {
		return ""
	}

	data, err := os.ReadFile(filepath.Join(repoDir, hint.Path))
	if err != nil {
		return ""
	}

	var parsedData any
	switch strings.ToLower(strings.TrimSpace(hint.Type)) {
	case "json":
		if err := json.Unmarshal(data, &parsedData); err != nil {
			return ""
		}
	case "yaml", "yml":
		if err := yaml.Unmarshal(data, &parsedData); err != nil {
			return ""
		}
	default:
		return ""
	}

	return lookupHintString(parsedData, hint.JSONPath)
}

func lookupHintString(parsedData any, path string) string {
	current := parsedData
	for _, segment := range strings.Split(path, ".") {
		if segment == "" {
			return ""
		}

		switch typed := current.(type) {
		case map[string]any, map[any]any:
			next, ok := lookupHintMapValue(typed, segment)
			if !ok {
				return ""
			}
			current = next
		case []any:
			index, err := strconv.Atoi(segment)
			if err != nil || index < 0 || index >= len(typed) {
				return ""
			}
			current = typed[index]
		default:
			return ""
		}
	}

	value, ok := current.(string)
	if !ok {
		return ""
	}

	return strings.TrimSpace(value)
}

func lookupHintMapValue(current any, segment string) (any, bool) {
	switch typed := current.(type) {
	case map[string]any:
		value, ok := typed[segment]
		return value, ok
	case map[any]any:
		value, ok := typed[segment]
		return value, ok
	default:
		return nil, false
	}
}

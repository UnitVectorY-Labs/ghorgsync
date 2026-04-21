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

	var document any
	switch strings.ToLower(strings.TrimSpace(hint.Type)) {
	case "json":
		if err := json.Unmarshal(data, &document); err != nil {
			return ""
		}
	case "yaml", "yml":
		if err := yaml.Unmarshal(data, &document); err != nil {
			return ""
		}
	default:
		return ""
	}

	return lookupHintString(document, hint.JSONPath)
}

func lookupHintString(document any, path string) string {
	current := document
	for _, segment := range strings.Split(path, ".") {
		if segment == "" {
			return ""
		}

		switch typed := current.(type) {
		case map[string]any:
			var ok bool
			current, ok = typed[segment]
			if !ok {
				return ""
			}
		case map[any]any:
			var ok bool
			current, ok = typed[segment]
			if !ok {
				return ""
			}
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

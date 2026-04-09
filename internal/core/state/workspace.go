package state

import (
	"path/filepath"
	"strings"
)

func NormalizeWorkspaceKey(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	normalized := filepath.Clean(value)
	if normalized == "." {
		return ""
	}
	return normalized
}

func ResolveWorkspaceKey(values ...string) string {
	for _, value := range values {
		if normalized := NormalizeWorkspaceKey(value); normalized != "" {
			return normalized
		}
	}
	return ""
}

func WorkspaceShortName(value string) string {
	key := ResolveWorkspaceKey(value)
	if key == "" {
		return ""
	}
	short := strings.TrimSpace(filepath.Base(key))
	if short == "" || short == "." || short == string(filepath.Separator) {
		return key
	}
	return short
}

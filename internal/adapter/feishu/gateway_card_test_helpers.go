package feishu

import (
	"fmt"
	"strings"
	"testing"
)

func markdownContent(element map[string]any) string {
	if cardStringValue(element["tag"]) != "markdown" {
		return ""
	}
	return cardStringValue(element["content"])
}

func lastButtonLabel(elements []map[string]any) string {
	for i := len(elements) - 1; i >= 0; i-- {
		if cardStringValue(elements[i]["tag"]) != "button" {
			continue
		}
		text, _ := elements[i]["text"].(map[string]any)
		if label := cardStringValue(text["content"]); label != "" {
			return label
		}
	}
	return ""
}

func containsButtonLabel(elements []map[string]any, want string) bool {
	for _, element := range elements {
		if cardStringValue(element["tag"]) != "button" {
			continue
		}
		text, _ := element["text"].(map[string]any)
		if cardStringValue(text["content"]) == want {
			return true
		}
	}
	return false
}

func containsMarkdownWithPrefix(elements []map[string]any, prefix string) bool {
	for _, element := range elements {
		if strings.HasPrefix(markdownContent(element), prefix) {
			return true
		}
	}
	return false
}

func containsMarkdownExact(elements []map[string]any, want string) bool {
	for _, element := range elements {
		if markdownContent(element) == want {
			return true
		}
	}
	return false
}

func lastMarkdownWithPrefix(elements []map[string]any, prefix string) string {
	for i := len(elements) - 1; i >= 0; i-- {
		if content := markdownContent(elements[i]); strings.HasPrefix(content, prefix) {
			return content
		}
	}
	return ""
}

func parseWorkspaceIndexFromLabel(t *testing.T, label string) int {
	t.Helper()
	var index int
	if _, err := fmt.Sscanf(label, "查看全部 · ws-%03d", &index); err != nil {
		t.Fatalf("parse workspace label %q: %v", label, err)
	}
	return index
}

func parseWorkspaceIndexFromRestoreLabel(t *testing.T, label string) int {
	t.Helper()
	var index int
	if _, err := fmt.Sscanf(label, "恢复 · ws-%03d", &index); err != nil {
		t.Fatalf("parse workspace label %q: %v", label, err)
	}
	return index
}

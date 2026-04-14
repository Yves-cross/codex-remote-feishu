package control

import "strings"

func ParseFeishuTextAction(text string) (Action, bool) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return Action{}, false
	}
	fields := strings.Fields(trimmed)
	if len(fields) > 0 {
		first := strings.ToLower(fields[0])
		for _, spec := range feishuCommandSpecs {
			for _, prefix := range spec.textPrefixes {
				if first == prefix.alias {
					return Action{Kind: prefix.kind, Text: trimmed}, true
				}
			}
		}
	}
	for _, spec := range feishuCommandSpecs {
		for _, match := range spec.textExact {
			if trimmed == match.alias {
				return match.action, true
			}
		}
	}
	return Action{}, false
}

func ParseFeishuMenuAction(eventKey string) (Action, bool) {
	trimmed := strings.TrimSpace(eventKey)
	if trimmed == "" {
		return Action{}, false
	}
	lower := strings.ToLower(trimmed)
	for _, spec := range feishuCommandSpecs {
		for _, dynamic := range spec.menuDynamic {
			if strings.HasPrefix(lower, dynamic.prefix) {
				text, ok := dynamic.build(trimmed[len(dynamic.prefix):])
				if !ok {
					return Action{}, false
				}
				return Action{Kind: dynamic.kind, Text: text}, true
			}
		}
	}
	normalized := NormalizeFeishuMenuEventKey(trimmed)
	for _, spec := range feishuCommandSpecs {
		for _, match := range spec.menuExact {
			if normalized == NormalizeFeishuMenuEventKey(match.alias) {
				return match.action, true
			}
		}
	}
	return Action{}, false
}

func NormalizeFeishuMenuEventKey(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(value))
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

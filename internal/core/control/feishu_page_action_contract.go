package control

import "strings"

// BuildFeishuActionText builds canonical slash text from a structured action.
// Page-card callbacks should use structured action payloads and derive text
// only at the gateway boundary when reducers still consume text arguments.
func BuildFeishuActionText(kind ActionKind, argument string) string {
	base := canonicalSlashForActionKind(kind)
	if base == "" {
		return ""
	}
	argument = strings.TrimSpace(argument)
	if argument == "" {
		return base
	}
	return base + " " + argument
}

func FeishuActionArgumentText(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	parts := strings.Fields(text)
	if len(parts) <= 1 {
		return ""
	}
	return strings.TrimSpace(strings.Join(parts[1:], " "))
}

func ActionKindForFeishuCommandID(commandID string) (ActionKind, bool) {
	spec, ok := feishuCommandSpecByID(strings.TrimSpace(commandID))
	if !ok {
		return "", false
	}
	return feishuCommandPrimaryActionKind(spec)
}

func canonicalSlashForActionKind(kind ActionKind) string {
	_, route, ok := feishuCommandActionRouteByKind(kind)
	if !ok {
		return ""
	}
	return route.canonicalSlash
}

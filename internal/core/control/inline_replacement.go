package control

import "strings"

// SupportsInlineCardReplacement is kept as a compatibility wrapper while the
// runtime moves to the explicit Feishu UI lifecycle policy.
func SupportsInlineCardReplacement(action Action) bool {
	policy, ok := InlineCardReplacementPolicy(action)
	return ok && policy.ReplaceCurrentCard
}

func isBareInlineCommand(text, command string) bool {
	fields := strings.Fields(strings.TrimSpace(text))
	return len(fields) == 1 && strings.EqualFold(fields[0], strings.TrimSpace(command))
}

package codex

import "github.com/kxn/codex-remote-feishu/internal/core/agentproto"

func extractThreadTokenUsageNotification(message map[string]any) (string, string, *agentproto.ThreadTokenUsage) {
	params := lookupMap(message, "params")
	if len(params) == 0 {
		return "", "", nil
	}
	threadID := lookupStringFromAny(params["threadId"])
	turnID := lookupStringFromAny(params["turnId"])
	usageMap := lookupMapFromAny(params["tokenUsage"])
	if len(usageMap) == 0 {
		return threadID, turnID, nil
	}
	usage := &agentproto.ThreadTokenUsage{
		Total: extractTokenUsageBreakdown(lookupMapFromAny(usageMap["total"])),
		Last:  extractTokenUsageBreakdown(lookupMapFromAny(usageMap["last"])),
	}
	if windowRaw := usageMap["modelContextWindow"]; windowRaw != nil {
		value := lookupIntFromAny(windowRaw)
		usage.ModelContextWindow = &value
	}
	return threadID, turnID, usage
}

func extractTokenUsageBreakdown(value map[string]any) agentproto.TokenUsageBreakdown {
	if len(value) == 0 {
		return agentproto.TokenUsageBreakdown{}
	}
	return agentproto.TokenUsageBreakdown{
		InputTokens:           lookupIntFromAny(value["inputTokens"]),
		CachedInputTokens:     lookupIntFromAny(value["cachedInputTokens"]),
		OutputTokens:          lookupIntFromAny(value["outputTokens"]),
		ReasoningOutputTokens: lookupIntFromAny(value["reasoningOutputTokens"]),
		TotalTokens:           lookupIntFromAny(value["totalTokens"]),
	}
}

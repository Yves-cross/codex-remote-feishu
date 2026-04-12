package agentproto

type TokenUsageBreakdown struct {
	InputTokens           int `json:"inputTokens,omitempty"`
	CachedInputTokens     int `json:"cachedInputTokens,omitempty"`
	OutputTokens          int `json:"outputTokens,omitempty"`
	ReasoningOutputTokens int `json:"reasoningOutputTokens,omitempty"`
	TotalTokens           int `json:"totalTokens,omitempty"`
}

type ThreadTokenUsage struct {
	Total              TokenUsageBreakdown `json:"total,omitempty"`
	Last               TokenUsageBreakdown `json:"last,omitempty"`
	ModelContextWindow *int                `json:"modelContextWindow,omitempty"`
}

func CloneThreadTokenUsage(usage *ThreadTokenUsage) *ThreadTokenUsage {
	if usage == nil {
		return nil
	}
	cloned := *usage
	if usage.ModelContextWindow != nil {
		value := *usage.ModelContextWindow
		cloned.ModelContextWindow = &value
	}
	return &cloned
}

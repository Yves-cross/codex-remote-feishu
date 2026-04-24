package issueworkflow

import (
	"strings"
	"time"
)

func canReclaimStaleProcessing(now time.Time, updatedAt time.Time, staleAfter time.Duration, enabled bool) bool {
	if !enabled || staleAfter <= 0 || updatedAt.IsZero() {
		return false
	}
	return !updatedAt.After(now.Add(-staleAfter))
}

type executionSnapshotState struct {
	CloseoutTailOnly bool
	Contradictions   []string
}

func analyzeExecutionSnapshot(body string) executionSnapshotState {
	fields := extractSnapshotFields(body)
	unfinished := splitSnapshotItems(fields["未完成尾项"])
	if len(unfinished) == 0 || !tailOnlyItems(unfinished) {
		return executionSnapshotState{}
	}
	state := executionSnapshotState{CloseoutTailOnly: true}
	if value := strings.TrimSpace(fields["当前执行点"]); looksLikeImplementationContinuation(value) {
		state.Contradictions = append(state.Contradictions, "当前执行点 still points at implementation/validation work while `未完成尾项` is close-out only")
	}
	if value := strings.TrimSpace(fields["下一步"]); looksLikeImplementationContinuation(value) {
		state.Contradictions = append(state.Contradictions, "下一步 still points at implementation/validation work while `未完成尾项` is close-out only")
	}
	return state
}

func extractSnapshotFields(body string) map[string]string {
	fields := map[string]string{}
	for _, rawLine := range strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}
		line = stripBulletPrefix(line)
		value, ok := extractLabeledField(line)
		if !ok {
			continue
		}
		fields[value.key] = value.value
	}
	return fields
}

type labeledField struct {
	key   string
	value string
}

func extractLabeledField(line string) (labeledField, bool) {
	separator := strings.IndexAny(line, ":：")
	if separator <= 0 {
		return labeledField{}, false
	}
	key := strings.TrimSpace(line[:separator])
	value := strings.TrimSpace(line[separator+1:])
	if key == "" {
		return labeledField{}, false
	}
	return labeledField{key: key, value: value}, true
}

func stripBulletPrefix(line string) string {
	trimmed := strings.TrimSpace(line)
	switch {
	case strings.HasPrefix(trimmed, "- "):
		return strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))
	case strings.HasPrefix(trimmed, "* "):
		return strings.TrimSpace(strings.TrimPrefix(trimmed, "* "))
	}
	for i := 0; i < len(trimmed); i++ {
		if trimmed[i] < '0' || trimmed[i] > '9' {
			if i > 0 && i+1 < len(trimmed) && trimmed[i] == '.' && trimmed[i+1] == ' ' {
				return strings.TrimSpace(trimmed[i+2:])
			}
			break
		}
	}
	return trimmed
}

func splitSnapshotItems(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	replacer := strings.NewReplacer("\n", ",", "，", ",", "、", ",", ";", ",", "；", ",")
	parts := strings.Split(replacer.Replace(value), ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		items = append(items, part)
	}
	return items
}

func tailOnlyItems(items []string) bool {
	if len(items) == 0 {
		return false
	}
	for _, item := range items {
		if !matchesTailOnlyItem(item) {
			return false
		}
	}
	return true
}

func matchesTailOnlyItem(item string) bool {
	normalized := normalizeForContains(item)
	return containsAny(normalized,
		"verifier",
		"commit",
		"push",
		"finish",
		"close",
		"closeplan",
		"发布",
		"收尾",
		"关闭",
		"comment",
		"rollup",
		"回卷",
		"同步issue",
	)
}

func looksLikeImplementationContinuation(value string) bool {
	normalized := normalizeForContains(value)
	if normalized == "" {
		return false
	}
	return containsAny(normalized,
		"实现",
		"编码",
		"改代码",
		"补测试",
		"补回归",
		"测试",
		"收口",
		"路由",
		"contract",
		"patch",
		"implement",
		"wire",
	)
}

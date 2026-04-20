package orchestrator

import (
	"strings"

	"github.com/kxn/codex-remote-feishu/internal/core/control"
	"github.com/kxn/codex-remote-feishu/internal/core/state"
)

func buildApprovalRequestSections(body string) []state.RequestPromptTextSectionRecord {
	body = strings.TrimSpace(body)
	if body == "" {
		body = "本地 Codex 正在等待你的确认。"
	}
	return appendRequestPromptSection(nil, "", body)
}

func buildRequestUserInputSections(body string) []state.RequestPromptTextSectionRecord {
	body = strings.TrimSpace(body)
	if body == "" {
		body = "本地 Codex 正在等待你补充参数或说明。"
	}
	return appendRequestPromptSection(nil, "", body)
}

func buildGenericRequestSections(body string) []state.RequestPromptTextSectionRecord {
	body = strings.TrimSpace(body)
	if body == "" {
		body = "本地 Codex 正在等待处理新的交互请求。"
	}
	return appendRequestPromptSection(nil, "", body)
}

func appendRequestPromptSection(sections []state.RequestPromptTextSectionRecord, label string, lines ...string) []state.RequestPromptTextSectionRecord {
	section := state.RequestPromptTextSectionRecord{
		Label: strings.TrimSpace(label),
		Lines: append([]string(nil), lines...),
	}.Normalized()
	if section.Label == "" && len(section.Lines) == 0 {
		return sections
	}
	return append(sections, section)
}

func requestPromptSectionsToControl(sections []state.RequestPromptTextSectionRecord) []control.FeishuCardTextSection {
	if len(sections) == 0 {
		return nil
	}
	out := make([]control.FeishuCardTextSection, 0, len(sections))
	for _, section := range sections {
		normalized := section.Normalized()
		if normalized.Label == "" && len(normalized.Lines) == 0 {
			continue
		}
		out = append(out, control.FeishuCardTextSection{
			Label: normalized.Label,
			Lines: append([]string(nil), normalized.Lines...),
		})
	}
	return out
}

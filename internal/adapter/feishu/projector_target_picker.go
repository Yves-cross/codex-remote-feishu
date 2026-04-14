package feishu

import (
	"strings"

	"github.com/kxn/codex-remote-feishu/internal/core/control"
)

func targetPickerElements(view control.FeishuTargetPickerView, daemonLifecycleID string) []map[string]any {
	elements := make([]map[string]any, 0, 8)
	if summary := targetPickerSummaryMarkdown(view); summary != "" {
		elements = append(elements, map[string]any{
			"tag":     "markdown",
			"content": summary,
		})
	}
	elements = append(elements, map[string]any{
		"tag":     "markdown",
		"content": "**工作区**",
	})
	elements = append(elements, pathPickerSelectStaticElement(
		cardTargetPickerWorkspaceFieldName,
		firstNonEmpty(strings.TrimSpace(view.WorkspacePlaceholder), "选择工作区"),
		stampActionValue(actionPayloadTargetPicker(cardActionKindTargetPickerSelectWorkspace, view.PickerID), daemonLifecycleID),
		targetPickerWorkspaceOptions(view.WorkspaceOptions),
		strings.TrimSpace(view.SelectedWorkspaceKey),
	))
	elements = append(elements, map[string]any{
		"tag":     "markdown",
		"content": "**会话**",
	})
	elements = append(elements, pathPickerSelectStaticElement(
		cardTargetPickerSessionFieldName,
		firstNonEmpty(strings.TrimSpace(view.SessionPlaceholder), "选择会话"),
		stampActionValue(actionPayloadTargetPicker(cardActionKindTargetPickerSelectSession, view.PickerID), daemonLifecycleID),
		targetPickerSessionOptions(view.SessionOptions),
		strings.TrimSpace(view.SelectedSessionValue),
	))
	if hint := strings.TrimSpace(view.Hint); hint != "" {
		elements = append(elements, map[string]any{
			"tag":     "markdown",
			"content": renderSystemInlineTags(hint),
		})
	}
	elements = append(elements, cardButtonGroupElement([]map[string]any{
		cardCallbackButtonElement(strings.TrimSpace(firstNonEmpty(view.ConfirmLabel, "确认")), "primary", stampActionValue(actionPayloadTargetPicker(cardActionKindTargetPickerConfirm, view.PickerID), daemonLifecycleID), !view.CanConfirm, "fill"),
	}))
	return elements
}

func targetPickerSummaryMarkdown(view control.FeishuTargetPickerView) string {
	lines := make([]string, 0, 2)
	if label := strings.TrimSpace(view.SelectedWorkspaceLabel); label != "" {
		line := "**当前工作区**\n" + formatNeutralTextTag(label)
		if meta := strings.TrimSpace(view.SelectedWorkspaceMeta); meta != "" {
			line += "\n" + renderSystemInlineTags(meta)
		}
		lines = append(lines, line)
	}
	if label := strings.TrimSpace(view.SelectedSessionLabel); label != "" {
		line := "**当前会话**\n" + formatNeutralTextTag(label)
		if meta := strings.TrimSpace(view.SelectedSessionMeta); meta != "" {
			line += "\n" + renderSystemInlineTags(meta)
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func targetPickerWorkspaceOptions(options []control.FeishuTargetPickerWorkspaceOption) []map[string]any {
	result := make([]map[string]any, 0, len(options))
	for _, option := range options {
		value := strings.TrimSpace(option.Value)
		if value == "" {
			continue
		}
		result = append(result, map[string]any{
			"text":  cardPlainText(targetPickerOptionLabel(option.Label, option.MetaText)),
			"value": value,
		})
	}
	return result
}

func targetPickerSessionOptions(options []control.FeishuTargetPickerSessionOption) []map[string]any {
	result := make([]map[string]any, 0, len(options))
	for _, option := range options {
		value := strings.TrimSpace(option.Value)
		if value == "" {
			continue
		}
		result = append(result, map[string]any{
			"text":  cardPlainText(targetPickerOptionLabel(option.Label, option.MetaText)),
			"value": value,
		})
	}
	return result
}

func targetPickerOptionLabel(label, meta string) string {
	label = strings.TrimSpace(label)
	meta = strings.TrimSpace(meta)
	if label == "" {
		return meta
	}
	if meta == "" {
		return label
	}
	if line := strings.TrimSpace(strings.Split(meta, "\n")[0]); line != "" {
		return label + " · " + line
	}
	return label
}

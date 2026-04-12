package feishu

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/kxn/codex-remote-feishu/internal/core/control"
)

func pathPickerElements(view control.FeishuPathPickerView, daemonLifecycleID string) []map[string]any {
	elements := make([]map[string]any, 0, len(view.Entries)*2+8)
	elements = append(elements, map[string]any{
		"tag":     "markdown",
		"content": "**允许范围**\n" + formatNeutralTextTag(view.RootPath),
	})
	elements = append(elements, map[string]any{
		"tag":     "markdown",
		"content": "**当前目录**\n" + formatNeutralTextTag(view.CurrentPath),
	})
	if selected := strings.TrimSpace(view.SelectedPath); selected != "" {
		elements = append(elements, map[string]any{
			"tag":     "markdown",
			"content": "**当前选择**\n" + formatNeutralTextTag(selected),
		})
	}
	elements = append(elements, cardButtonGroupElement([]map[string]any{
		cardCallbackButtonElement("上一级", "default", stampActionValue(actionPayloadPathPicker(cardActionKindPathPickerUp, view.PickerID, ""), daemonLifecycleID), !view.CanGoUp, ""),
		cardCallbackButtonElement(strings.TrimSpace(firstNonEmpty(view.ConfirmLabel, "确认")), "primary", stampActionValue(actionPayloadPathPicker(cardActionKindPathPickerConfirm, view.PickerID, ""), daemonLifecycleID), !view.CanConfirm, ""),
		cardCallbackButtonElement(strings.TrimSpace(firstNonEmpty(view.CancelLabel, "取消")), "default", stampActionValue(actionPayloadPathPicker(cardActionKindPathPickerCancel, view.PickerID, ""), daemonLifecycleID), false, ""),
	}))
	if len(view.Entries) == 0 {
		if hint := strings.TrimSpace(view.Hint); hint != "" {
			elements = append(elements, map[string]any{
				"tag":     "markdown",
				"content": renderSystemInlineTags(hint),
			})
		}
		return elements
	}
	elements = append(elements, map[string]any{
		"tag":     "markdown",
		"content": "**目录内容**",
	})
	for index, entry := range view.Entries {
		meta := fmt.Sprintf("%d. %s", index+1, pathPickerEntryLine(entry))
		elements = append(elements, map[string]any{
			"tag":     "markdown",
			"content": renderSystemInlineTags(meta),
		})
		if button := pathPickerEntryButton(view, entry, daemonLifecycleID); len(button) != 0 {
			elements = append(elements, cardButtonGroupElement([]map[string]any{button}))
		}
	}
	if hint := strings.TrimSpace(view.Hint); hint != "" {
		elements = append(elements, map[string]any{
			"tag":     "markdown",
			"content": renderSystemInlineTags(hint),
		})
	}
	return elements
}

func pathPickerEntryLine(entry control.FeishuPathPickerEntry) string {
	name := strings.TrimSpace(firstNonEmpty(entry.Label, entry.Name))
	kind := "文件"
	if entry.Kind == control.PathPickerEntryDirectory {
		kind = "目录"
	}
	parts := []string{name, formatNeutralTextTag(kind)}
	if entry.Selected {
		parts = append(parts, "[已选]")
	}
	if reason := strings.TrimSpace(entry.DisabledReason); reason != "" {
		parts = append(parts, reason)
	}
	return strings.Join(parts, " · ")
}

func pathPickerEntryButton(view control.FeishuPathPickerView, entry control.FeishuPathPickerEntry, daemonLifecycleID string) map[string]any {
	if entry.Disabled || entry.ActionKind == control.PathPickerEntryActionNone {
		return nil
	}
	buttonType := "default"
	label := "选择"
	kind := cardActionKindPathPickerSelect
	switch entry.ActionKind {
	case control.PathPickerEntryActionEnter:
		buttonType = "primary"
		label = "进入 · " + filepath.Base(strings.TrimSpace(firstNonEmpty(entry.Label, entry.Name)))
		kind = cardActionKindPathPickerEnter
	case control.PathPickerEntryActionSelect:
		buttonType = "primary"
		if entry.Selected {
			buttonType = "default"
		}
		label = "选择 · " + filepath.Base(strings.TrimSpace(firstNonEmpty(entry.Label, entry.Name)))
	}
	return cardCallbackButtonElement(label, buttonType, stampActionValue(actionPayloadPathPicker(kind, view.PickerID, entry.Name), daemonLifecycleID), false, "fill")
}

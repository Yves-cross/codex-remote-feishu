package orchestrator

import (
	"fmt"
	"strings"

	"github.com/kxn/codex-remote-feishu/internal/core/control"
)

func steerAllOwnerCardEvent(surfaceID, messageID, title, theme string, lines ...string) control.UIEvent {
	sections := make([]control.FeishuCardTextSection, 0, 1)
	bodyLines := make([]string, 0, len(lines))
	for _, line := range lines {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			bodyLines = append(bodyLines, trimmed)
		}
	}
	if len(bodyLines) != 0 {
		sections = append(sections, control.FeishuCardTextSection{Lines: bodyLines})
	}
	return control.UIEvent{
		Kind:             control.UIEventFeishuDirectCommandCatalog,
		SurfaceSessionID: strings.TrimSpace(surfaceID),
		FeishuDirectCommandCatalog: &control.FeishuDirectCommandCatalog{
			MessageID:       strings.TrimSpace(messageID),
			Title:           strings.TrimSpace(title),
			ThemeKey:        strings.TrimSpace(theme),
			Interactive:     false,
			SummarySections: sections,
		},
	}
}

func steerAllNoopOwnerCardEvent(surfaceID, messageID string) control.UIEvent {
	return steerAllOwnerCardEvent(surfaceID, messageID, "没有可并入的排队输入", "system", "当前没有可并入本轮执行的排队消息。")
}

func steerAllRequestedOwnerCardEvent(surfaceID, messageID string, count int) control.UIEvent {
	return steerAllOwnerCardEvent(
		surfaceID,
		messageID,
		"正在并入排队输入",
		"progress",
		fmt.Sprintf("正在把 %d 条排队输入并入当前执行。", count),
	)
}

func steerAllCompletedOwnerCardEvent(surfaceID, messageID string, count int) control.UIEvent {
	return steerAllOwnerCardEvent(
		surfaceID,
		messageID,
		"已并入排队输入",
		"success",
		fmt.Sprintf("已把 %d 条排队输入并入当前执行。", count),
	)
}

func steerAllFailedOwnerCardEvent(surfaceID, messageID, text string) control.UIEvent {
	if strings.TrimSpace(text) == "" {
		text = "追加输入失败，已恢复原排队位置。"
	}
	return steerAllOwnerCardEvent(surfaceID, messageID, "并入失败", "error", text)
}

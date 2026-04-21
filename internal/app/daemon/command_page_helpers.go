package daemon

import (
	"strings"

	"github.com/kxn/codex-remote-feishu/internal/core/control"
)

func commandPageEvent(surfaceID string, view control.FeishuCommandPageView) control.UIEvent {
	return control.UIEvent{
		Kind:             control.UIEventFeishuCommandView,
		SurfaceSessionID: strings.TrimSpace(surfaceID),
		FeishuCommandView: &control.FeishuCommandView{
			Page: &view,
		},
	}
}

func commandPageEvents(surfaceID string, view control.FeishuCommandPageView) []control.UIEvent {
	return []control.UIEvent{commandPageEvent(surfaceID, view)}
}

func commandArgumentText(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	idx := strings.IndexAny(text, " \t")
	if idx < 0 || idx+1 >= len(text) {
		return ""
	}
	return strings.TrimSpace(text[idx+1:])
}

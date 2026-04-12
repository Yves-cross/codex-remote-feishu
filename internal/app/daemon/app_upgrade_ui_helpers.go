package daemon

import (
	"strings"

	"github.com/kxn/codex-remote-feishu/internal/app/install"
	"github.com/kxn/codex-remote-feishu/internal/core/control"
)

func debugUsageEvents(surfaceID, message string) []control.UIEvent {
	events := []control.UIEvent{}
	if strings.TrimSpace(message) != "" {
		events = append(events, debugNoticeEvent(surfaceID, "debug_usage_error", message))
	}
	events = append(events, control.UIEvent{
		Kind:                       control.UIEventFeishuDirectCommandCatalog,
		SurfaceSessionID:           surfaceID,
		FeishuDirectCommandCatalog: buildDebugStatusCatalog(install.InstallState{}, false),
	})
	return events
}

func upgradeUsageEvents(surfaceID, message string) []control.UIEvent {
	events := []control.UIEvent{}
	if strings.TrimSpace(message) != "" {
		events = append(events, upgradeNoticeEvent(surfaceID, "upgrade_usage_error", message))
	}
	events = append(events, control.UIEvent{
		Kind:                       control.UIEventFeishuDirectCommandCatalog,
		SurfaceSessionID:           surfaceID,
		FeishuDirectCommandCatalog: buildUpgradeStatusCatalog(install.InstallState{}, false),
	})
	return events
}

func runCommandButton(label, commandText, style string, disabled bool) control.CommandCatalogButton {
	return control.CommandCatalogButton{
		Label:       label,
		Kind:        control.CommandCatalogButtonRunCommand,
		CommandText: commandText,
		Style:       style,
		Disabled:    disabled,
	}
}

func debugNoticeEvent(surfaceID, code, text string) control.UIEvent {
	return control.UIEvent{
		Kind:             control.UIEventNotice,
		SurfaceSessionID: surfaceID,
		Notice: &control.Notice{
			Code:  code,
			Title: "Debug",
			Text:  text,
		},
	}
}

func upgradeNoticeEvent(surfaceID, code, text string) control.UIEvent {
	return control.UIEvent{
		Kind:             control.UIEventNotice,
		SurfaceSessionID: surfaceID,
		Notice: &control.Notice{
			Code:  code,
			Title: "Upgrade",
			Text:  text,
		},
	}
}

package daemon

import (
	"fmt"
	"strings"

	"github.com/kxn/codex-remote-feishu/internal/app/install"
	"github.com/kxn/codex-remote-feishu/internal/core/control"
)

func buildDebugRootPageView(stateValue install.InstallState, checkInFlight bool, formDefault, statusKind, statusText string) control.FeishuPageView {
	def, _ := control.FeishuCommandDefinitionByID(control.FeishuCommandDebug)
	return control.FeishuPageView{
		CommandID:    control.FeishuCommandDebug,
		StatusKind:   strings.TrimSpace(statusKind),
		StatusText:   strings.TrimSpace(statusText),
		Interactive:  true,
		DisplayStyle: control.CommandCatalogDisplayCompactButtons,
		Sections: []control.CommandCatalogSection{
			{
				Title: "调试",
				Entries: []control.CommandCatalogEntry{{
					Buttons: directSubcommandButtons(def, def.CanonicalSlash, "/debug admin", ""),
				}},
			},
		},
	}
}

func buildUpgradeRootPageView(stateValue install.InstallState, showCodexUpgrade bool, formDefault, statusKind, statusText string) control.FeishuPageView {
	def, _ := control.FeishuCommandDefinitionByID(control.FeishuCommandUpgrade)
	quickButtons := directSubcommandButtons(def, def.CanonicalSlash, "/upgrade latest", "")
	if showCodexUpgrade {
		quickButtons = append(quickButtons, runCommandButton("Codex 升级", "/upgrade codex", "", false))
	}
	return control.FeishuPageView{
		CommandID:    control.FeishuCommandUpgrade,
		StatusKind:   strings.TrimSpace(statusKind),
		StatusText:   strings.TrimSpace(statusText),
		Interactive:  true,
		DisplayStyle: control.CommandCatalogDisplayCompactButtons,
		Sections: []control.CommandCatalogSection{
			{
				Title: "升级系统",
				Entries: []control.CommandCatalogEntry{{
					Buttons: quickButtons,
				}},
			},
		},
	}
}

func buildUpgradeTrackPageView(stateValue install.InstallState) control.FeishuPageView {
	currentTrack := strings.TrimSpace(string(stateValue.CurrentTrack))
	def, _ := control.FeishuCommandDefinitionByID(control.FeishuCommandUpgrade)
	return control.FeishuPageView{
		CommandID:       control.FeishuCommandUpgrade,
		Title:           "Upgrade Track",
		Breadcrumbs:     control.FeishuCommandBreadcrumbsForCommand(control.FeishuCommandUpgrade, "Track"),
		SummarySections: commandCatalogSummarySections(buildTrackSummaryLines(stateValue)...),
		Interactive:     true,
		DisplayStyle:    control.CommandCatalogDisplayCompactButtons,
		Sections: []control.CommandCatalogSection{
			{
				Title: "切换 Track",
				Entries: []control.CommandCatalogEntry{{
					Buttons: directSubcommandButtons(def, "/upgrade track", "/upgrade track "+currentTrack, "/upgrade track "+currentTrack),
				}},
			},
		},
		RelatedButtons: control.FeishuCommandBackToRootButtons(control.FeishuCommandUpgrade),
	}
}

func buildTrackSummaryLines(stateValue install.InstallState) []string {
	return []string{
		fmt.Sprintf("当前 Track：%s", firstNonEmpty(string(stateValue.CurrentTrack), "unknown")),
	}
}

func directSubcommandButtons(def control.FeishuCommandDefinition, prefix, primaryCommand, disabledCommand string) []control.CommandCatalogButton {
	prefixFields := strings.Fields(strings.ToLower(strings.TrimSpace(prefix)))
	if len(prefixFields) == 0 {
		return nil
	}
	buttons := make([]control.CommandCatalogButton, 0, len(def.Options))
	for _, option := range def.Options {
		commandText := strings.TrimSpace(option.CommandText)
		fields := strings.Fields(strings.ToLower(commandText))
		if len(fields) != len(prefixFields)+1 {
			continue
		}
		match := true
		for i, field := range prefixFields {
			if fields[i] != field {
				match = false
				break
			}
		}
		if !match {
			continue
		}
		label := strings.TrimSpace(option.Label)
		if label == "" {
			label = commandText
		}
		style := ""
		if commandText == strings.TrimSpace(primaryCommand) {
			style = "primary"
		}
		buttons = append(buttons, runCommandButton(label, commandText, style, commandText == strings.TrimSpace(disabledCommand)))
	}
	return buttons
}

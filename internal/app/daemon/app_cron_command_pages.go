package daemon

import (
	"strings"

	"github.com/kxn/codex-remote-feishu/internal/core/control"
)

func buildCronRootPageView(stateValue *cronStateFile, ownerView cronOwnerView, extraSummary string, configReady bool, formDefault, statusKind, statusText string) control.FeishuCommandPageView {
	primaryCommand := cronPrimaryMenuCommand(stateValue, ownerView)
	canEdit := cronCanEdit(stateValue) && configReady
	canReload := cronCanReload(stateValue, ownerView)
	var summarySections []control.FeishuCardTextSection
	if line := strings.TrimSpace(extraSummary); line != "" {
		summarySections = commandCatalogSummarySections(line)
	}
	return control.FeishuCommandPageView{
		CommandID:       control.FeishuCommandCron,
		SummarySections: summarySections,
		StatusKind:      strings.TrimSpace(statusKind),
		StatusText:      strings.TrimSpace(statusText),
		Interactive:     true,
		DisplayStyle:    control.CommandCatalogDisplayCompactButtons,
		Sections: []control.CommandCatalogSection{
			{
				Title: "查看与编辑",
				Entries: []control.CommandCatalogEntry{{
					Buttons: []control.CommandCatalogButton{
						runCommandButton("当前状态", "/cron status", cronPrimaryButtonStyle(primaryCommand, "/cron status"), false),
						runCommandButton("当前任务", "/cron list", cronPrimaryButtonStyle(primaryCommand, "/cron list"), false),
						runCommandButton("打开配置", "/cron edit", cronPrimaryButtonStyle(primaryCommand, "/cron edit"), !canEdit),
					},
				}},
			},
			{
				Title: "应用与维护",
				Entries: []control.CommandCatalogEntry{{
					Buttons: []control.CommandCatalogButton{
						runCommandButton("重新加载", "/cron reload", cronPrimaryButtonStyle(primaryCommand, "/cron reload"), !canReload),
						runCommandButton("修复配置", "/cron repair", cronPrimaryButtonStyle(primaryCommand, "/cron repair"), false),
					},
				}},
			},
		},
	}
}

func buildCronStatusPageView(stateValue *cronStateFile, ownerView cronOwnerView, extraSummary string, configReady bool) control.FeishuCommandPageView {
	return commandPageViewFromCatalog(
		control.FeishuCommandCron,
		buildCronStatusCatalog(stateValue, ownerView, extraSummary, configReady),
		control.FeishuCommandBreadcrumbsForCommand(control.FeishuCommandCron, "当前状态"),
		control.FeishuCommandBackToRootButtons(control.FeishuCommandCron),
	)
}

func buildCronListPageView(stateValue *cronStateFile, ownerView cronOwnerView, extraSummary string) control.FeishuCommandPageView {
	return commandPageViewFromCatalog(
		control.FeishuCommandCron,
		buildCronListCatalog(stateValue, ownerView, extraSummary),
		control.FeishuCommandBreadcrumbsForCommand(control.FeishuCommandCron, "当前任务"),
		control.FeishuCommandBackToRootButtons(control.FeishuCommandCron),
	)
}

func buildCronEditPageView(stateValue *cronStateFile, ownerView cronOwnerView, extraSummary string, configReady bool) control.FeishuCommandPageView {
	return commandPageViewFromCatalog(
		control.FeishuCommandCron,
		buildCronEditCatalog(stateValue, ownerView, extraSummary, configReady),
		control.FeishuCommandBreadcrumbsForCommand(control.FeishuCommandCron, "打开配置"),
		control.FeishuCommandBackToRootButtons(control.FeishuCommandCron),
	)
}

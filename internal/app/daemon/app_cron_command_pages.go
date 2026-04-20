package daemon

import (
	"strings"

	"github.com/kxn/codex-remote-feishu/internal/core/control"
)

func buildCronRootPageView(stateValue *cronStateFile, ownerView cronOwnerView, extraSummary string, configReady bool, formDefault, statusKind, statusText string) control.FeishuCommandPageView {
	primaryCommand := cronPrimaryMenuCommand(stateValue, ownerView)
	canEdit := cronCanEdit(stateValue) && configReady
	canReload := cronCanReload(stateValue, ownerView)
	summaryLines := []string{
		"选择 Cron 的下一步操作。",
		"根页不再默认展开状态；查看运行状态、任务列表或配置入口，请进入对应子页。",
	}
	if strings.TrimSpace(extraSummary) != "" {
		summaryLines = append(summaryLines, strings.TrimSpace(extraSummary))
	}
	return control.FeishuCommandPageView{
		CommandID:       control.FeishuCommandCron,
		SummarySections: commandCatalogSummarySections(summaryLines...),
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
			cronManualCommandSectionWithDefault(formDefault),
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

func cronManualCommandSectionWithDefault(defaultValue string) control.CommandCatalogSection {
	section := cronManualCommandSection()
	if len(section.Entries) == 0 {
		return section
	}
	form := control.FeishuCommandFormWithDefault(control.FeishuCommandCron, defaultValue)
	section.Entries[0].Form = form
	return section
}

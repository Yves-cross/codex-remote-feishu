package daemon

import (
	"fmt"
	"strings"

	"github.com/kxn/codex-remote-feishu/internal/app/install"
	"github.com/kxn/codex-remote-feishu/internal/core/control"
)

func buildDebugRootPageView(stateValue install.InstallState, checkInFlight bool, formDefault, statusKind, statusText string) control.FeishuCommandPageView {
	quickButtons := []control.CommandCatalogButton{
		runCommandButton("管理页外链", "/debug admin", "", false),
		runCommandButton("升级入口", "/upgrade", "", false),
		runCommandButton("查看 Track", "/upgrade track", "", false),
		runCommandButton("检查/继续升级", "/upgrade latest", "primary", false),
	}
	if install.CurrentBuildAllowsLocalUpgrade() {
		quickButtons = append(quickButtons, runCommandButton("本地升级", "/upgrade local", "", false))
	}
	return control.FeishuCommandPageView{
		CommandID:       control.FeishuCommandDebug,
		SummarySections: commandCatalogSummarySections(debugRootSummaryLines(checkInFlight)...),
		StatusKind:      strings.TrimSpace(statusKind),
		StatusText:      strings.TrimSpace(statusText),
		Interactive:     true,
		DisplayStyle:    control.CommandCatalogDisplayCompactButtons,
		Sections: []control.CommandCatalogSection{
			{
				Title: "快捷操作",
				Entries: []control.CommandCatalogEntry{{
					Buttons: quickButtons,
				}},
			},
			{
				Title: "手动输入",
				Entries: []control.CommandCatalogEntry{{
					Commands:    []string{"/debug", "/debug admin"},
					Description: "发送 /debug 打开调试根页；管理页外链请使用 /debug admin。历史兼容的 /debug track 请改用 /upgrade track。",
					Form:        control.FeishuCommandFormWithDefault(control.FeishuCommandDebug, formDefault),
				}},
			},
		},
	}
}

func debugRootSummaryLines(checkInFlight bool) []string {
	lines := []string{
		"选择 Debug 的下一步操作。",
		"升级相关状态与 track 已统一收敛到 Upgrade 子页；这里只保留调试入口和兼容命令入口。",
	}
	if checkInFlight {
		lines = append(lines, "当前有升级检查正在进行；结果会在完成后异步更新。")
	}
	return lines
}

func buildUpgradeRootPageView(stateValue install.InstallState, formDefault, statusKind, statusText string) control.FeishuCommandPageView {
	currentTrack := strings.TrimSpace(string(stateValue.CurrentTrack))
	quickButtons := []control.CommandCatalogButton{
		runCommandButton("查看 Track", "/upgrade track", "", false),
		runCommandButton("检查/继续升级", "/upgrade latest", "primary", false),
		runCommandButton("开发构建", "/upgrade dev", "", false),
	}
	if install.CurrentBuildAllowsLocalUpgrade() {
		quickButtons = append(quickButtons, runCommandButton("本地升级", "/upgrade local", "", false))
	}
	return control.FeishuCommandPageView{
		CommandID:       control.FeishuCommandUpgrade,
		SummarySections: commandCatalogSummarySections(upgradeRootSummaryLines()...),
		StatusKind:      strings.TrimSpace(statusKind),
		StatusText:      strings.TrimSpace(statusText),
		Interactive:     true,
		DisplayStyle:    control.CommandCatalogDisplayCompactButtons,
		Sections: []control.CommandCatalogSection{
			{
				Title: "快捷操作",
				Entries: []control.CommandCatalogEntry{{
					Buttons: quickButtons,
				}},
			},
			{
				Title: "切换 Track",
				Entries: []control.CommandCatalogEntry{{
					Buttons: buildTrackCommandButtons(currentTrack),
				}},
			},
			{
				Title: "手动输入",
				Entries: []control.CommandCatalogEntry{{
					Commands:    []string{"/upgrade", "/upgrade track", "/upgrade latest", "/upgrade dev"},
					Description: "发送 /upgrade 打开升级根页；/upgrade track 查看或切换 track；/upgrade latest 继续 release 升级；/upgrade dev 继续开发构建升级。",
					Form:        control.FeishuCommandFormWithDefault(control.FeishuCommandUpgrade, formDefault),
				}},
			},
		},
	}
}

func upgradeRootSummaryLines() []string {
	lines := []string{
		"选择升级的下一步操作。",
		"根页不再默认展开状态；查看当前 track 或版本信息，请进入 /upgrade track 子页。",
	}
	if install.CurrentBuildAllowsLocalUpgrade() {
		lines = append(lines, "当前构建支持 /upgrade local。")
	}
	return lines
}

func buildUpgradeTrackPageView(stateValue install.InstallState, legacyAlias bool) control.FeishuCommandPageView {
	statusKind := ""
	statusText := ""
	if legacyAlias {
		statusKind = "info"
		statusText = "当前使用了兼容 alias；后续请改用 /upgrade track。"
	}
	currentTrack := strings.TrimSpace(string(stateValue.CurrentTrack))
	return control.FeishuCommandPageView{
		CommandID:       control.FeishuCommandUpgrade,
		Title:           "Upgrade Track",
		Breadcrumbs:     control.FeishuCommandBreadcrumbsForCommand(control.FeishuCommandUpgrade, "Track"),
		SummarySections: commandCatalogSummarySections(buildTrackSummaryLines(stateValue)...),
		StatusKind:      statusKind,
		StatusText:      statusText,
		Interactive:     true,
		DisplayStyle:    control.CommandCatalogDisplayCompactButtons,
		Sections: []control.CommandCatalogSection{
			{
				Title: "立即切换",
				Entries: []control.CommandCatalogEntry{{
					Buttons: buildTrackCommandButtons(currentTrack),
				}},
			},
			{
				Title: "下一步",
				Entries: []control.CommandCatalogEntry{{
					Buttons: []control.CommandCatalogButton{
						runCommandButton("检查/继续升级", "/upgrade latest", "primary", false),
					},
				}},
			},
		},
		RelatedButtons: control.FeishuCommandBackToRootButtons(control.FeishuCommandUpgrade),
	}
}

func buildTrackSummaryLines(stateValue install.InstallState) []string {
	lines := []string{
		fmt.Sprintf("当前 track：%s", firstNonEmpty(string(stateValue.CurrentTrack), "unknown")),
		fmt.Sprintf("安装来源：%s", displayInstallSource(stateValue.InstallSource)),
		fmt.Sprintf("当前构建允许的 track：%s", strings.Join(currentBuildTrackNames(), "、")),
	}
	if latest := strings.TrimSpace(stateValue.LastKnownLatestVersion); latest != "" {
		lines = append(lines, fmt.Sprintf("最近看到的最新版本：%s", latest))
	}
	lines = append(lines, "切换 track 不会自动触发升级；需要立即检查时请发送 /upgrade latest。滚动开发构建请使用 /upgrade dev。")
	return lines
}

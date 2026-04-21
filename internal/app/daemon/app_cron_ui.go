package daemon

import (
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/kxn/codex-remote-feishu/internal/core/control"
)

const (
	cronScheduleTypeDaily    = "每天定时"
	cronScheduleTypeInterval = "间隔运行"
)

type cronIntervalChoice struct {
	Label   string
	Minutes int
}

var cronIntervalChoices = []cronIntervalChoice{
	{Label: "5分钟", Minutes: 5},
	{Label: "10分钟", Minutes: 10},
	{Label: "15分钟", Minutes: 15},
	{Label: "30分钟", Minutes: 30},
	{Label: "1小时", Minutes: 60},
	{Label: "2小时", Minutes: 120},
	{Label: "4小时", Minutes: 240},
	{Label: "6小时", Minutes: 360},
	{Label: "12小时", Minutes: 720},
	{Label: "24小时", Minutes: 1440},
}

type cronCommandMode string

const (
	cronCommandMenu   cronCommandMode = "menu"
	cronCommandStatus cronCommandMode = "status"
	cronCommandList   cronCommandMode = "list"
	cronCommandRun    cronCommandMode = "run"
	cronCommandEdit   cronCommandMode = "edit"
	cronCommandRepair cronCommandMode = "repair"
	cronCommandReload cronCommandMode = "reload"
)

type parsedCronCommand struct {
	Mode        cronCommandMode
	JobRecordID string
}

func parseCronCommandText(text string) (parsedCronCommand, error) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return parsedCronCommand{}, fmt.Errorf("缺少 /cron 子命令。")
	}
	fields := strings.Fields(trimmed)
	if len(fields) == 0 || strings.ToLower(fields[0]) != "/cron" {
		return parsedCronCommand{}, fmt.Errorf("不支持的 /cron 子命令。")
	}
	switch len(fields) {
	case 1:
		return parsedCronCommand{Mode: cronCommandMenu}, nil
	case 2:
		switch strings.ToLower(fields[1]) {
		case "status":
			return parsedCronCommand{Mode: cronCommandStatus}, nil
		case "list":
			return parsedCronCommand{Mode: cronCommandList}, nil
		case "edit":
			return parsedCronCommand{Mode: cronCommandEdit}, nil
		case "repair":
			return parsedCronCommand{Mode: cronCommandRepair}, nil
		case "reload":
			return parsedCronCommand{Mode: cronCommandReload}, nil
		}
		return parsedCronCommand{}, fmt.Errorf("`/cron` 只支持 `/cron status`、`/cron list`、`/cron edit`、`/cron reload`、`/cron repair`，以及按钮触发的 `/cron run <任务记录ID>`。")
	case 3:
		if strings.ToLower(fields[1]) == "run" {
			jobRecordID := strings.TrimSpace(fields[2])
			if jobRecordID == "" {
				return parsedCronCommand{}, fmt.Errorf("`/cron run` 需要任务记录 ID。")
			}
			return parsedCronCommand{Mode: cronCommandRun, JobRecordID: jobRecordID}, nil
		}
		return parsedCronCommand{}, fmt.Errorf("`/cron` 只支持 `/cron status`、`/cron list`、`/cron edit`、`/cron reload`、`/cron repair`，以及按钮触发的 `/cron run <任务记录ID>`。")
	default:
		return parsedCronCommand{}, fmt.Errorf("`/cron status` / `/cron list` / `/cron edit` / `/cron reload` / `/cron repair` 不接受额外参数；单任务触发请使用 `/cron run <任务记录ID>`。")
	}
}

func cronUsageEvents(surfaceID, formDefault, message string) []control.UIEvent {
	return commandPageEvents(surfaceID, buildCronRootPageView(nil, cronOwnerView{}, "", false, formDefault, "error", message))
}

func cronBindingSummaryLines(stateValue *cronStateFile, configReady bool) []string {
	if stateValue == nil || stateValue.Bitable == nil {
		return []string{"配置表：未初始化"}
	}
	lines := []string{cronConfigSummaryLine(stateValue, configReady)}
	if line := cronRunsSummaryLine(stateValue, configReady); line != "" {
		lines = append(lines, line)
	}
	return lines
}

func cronConfigSummaryLine(stateValue *cronStateFile, configReady bool) string {
	if stateValue == nil || stateValue.Bitable == nil {
		return "配置表：未初始化"
	}
	if !configReady {
		return "配置表：工作区清单未同步，暂不开放配置入口"
	}
	if url := cronBitableTableURL(stateValue.Bitable.AppURL, stateValue.Bitable.Tables.Tasks); url != "" {
		return "配置表：可从下方外部入口打开"
	}
	return fmt.Sprintf("配置表：%s", strings.TrimSpace(stateValue.Bitable.AppToken))
}

func cronRunsSummaryLine(stateValue *cronStateFile, configReady bool) string {
	if stateValue == nil || stateValue.Bitable == nil {
		return ""
	}
	if !configReady {
		return ""
	}
	if strings.TrimSpace(stateValue.Bitable.Tables.Runs) == "" {
		return ""
	}
	if url := cronBitableTableURL(stateValue.Bitable.AppURL, stateValue.Bitable.Tables.Runs); url != "" {
		return "运行状态：可从下方外部入口打开"
	}
	return ""
}

func cronExternalLinkSection(stateValue *cronStateFile, configReady bool) (control.CommandCatalogSection, bool) {
	buttons := cronExternalLinkButtons(stateValue, configReady)
	if len(buttons) == 0 {
		return control.CommandCatalogSection{}, false
	}
	return control.CommandCatalogSection{
		Title: "外部入口",
		Entries: []control.CommandCatalogEntry{{
			Buttons: buttons,
		}},
	}, true
}

func cronExternalLinkButtons(stateValue *cronStateFile, configReady bool) []control.CommandCatalogButton {
	if stateValue == nil || stateValue.Bitable == nil || !configReady {
		return nil
	}
	buttons := []control.CommandCatalogButton{}
	if button, ok := cronConfigLinkButton(stateValue, configReady); ok {
		buttons = append(buttons, button)
	}
	if button, ok := cronRunsLinkButton(stateValue, configReady); ok {
		buttons = append(buttons, button)
	}
	return buttons
}

func cronConfigLinkButton(stateValue *cronStateFile, configReady bool) (control.CommandCatalogButton, bool) {
	if stateValue == nil || stateValue.Bitable == nil || !configReady {
		return control.CommandCatalogButton{}, false
	}
	url := cronBitableTableURL(stateValue.Bitable.AppURL, stateValue.Bitable.Tables.Tasks)
	if url == "" {
		return control.CommandCatalogButton{}, false
	}
	return openURLButton("打开 Cron 配置表", url, "", false), true
}

func cronRunsLinkButton(stateValue *cronStateFile, configReady bool) (control.CommandCatalogButton, bool) {
	if stateValue == nil || stateValue.Bitable == nil || !configReady {
		return control.CommandCatalogButton{}, false
	}
	url := cronBitableTableURL(stateValue.Bitable.AppURL, stateValue.Bitable.Tables.Runs)
	if url == "" {
		return control.CommandCatalogButton{}, false
	}
	return openURLButton("打开运行记录", url, "", false), true
}

func cronBitableTableURL(appURL, tableID string) string {
	appURL = strings.TrimSpace(appURL)
	if appURL == "" {
		return ""
	}
	if strings.TrimSpace(tableID) == "" {
		return appURL
	}
	parsed, err := url.Parse(appURL)
	if err != nil || strings.TrimSpace(parsed.Host) == "" {
		return appURL
	}
	query := parsed.Query()
	query.Set("table", strings.TrimSpace(tableID))
	query.Del("view")
	query.Del("record")
	query.Del("field")
	query.Del("search")
	parsed.RawQuery = query.Encode()
	parsed.Fragment = ""
	return parsed.String()
}

func cronPrimaryMenuCommand(stateValue *cronStateFile, ownerView cronOwnerView) string {
	if cronRepairShouldBePrimary(stateValue, ownerView) {
		return "/cron repair"
	}
	if cronCanEdit(stateValue) {
		return "/cron edit"
	}
	if cronCanReload(stateValue, ownerView) {
		return "/cron reload"
	}
	return "/cron status"
}

func cronPrimaryDetailCommand(stateValue *cronStateFile, ownerView cronOwnerView) string {
	if cronRepairShouldBePrimary(stateValue, ownerView) {
		return "/cron repair"
	}
	if cronCanEdit(stateValue) {
		return "/cron edit"
	}
	if cronCanReload(stateValue, ownerView) {
		return "/cron reload"
	}
	return ""
}

func cronPrimaryEditCommand(stateValue *cronStateFile, ownerView cronOwnerView) string {
	if cronRepairShouldBePrimary(stateValue, ownerView) {
		return "/cron repair"
	}
	if cronCanReload(stateValue, ownerView) {
		return "/cron reload"
	}
	return ""
}

func cronPrimaryButtonStyle(primaryCommand, commandText string) string {
	if strings.TrimSpace(primaryCommand) == strings.TrimSpace(commandText) {
		return "primary"
	}
	return ""
}

func cronRepairShouldBePrimary(stateValue *cronStateFile, ownerView cronOwnerView) bool {
	if !cronStateHasBinding(stateValue) {
		return true
	}
	return ownerView.NeedsRepair
}

func cronCanEdit(stateValue *cronStateFile) bool {
	return cronStateHasBinding(stateValue)
}

func cronCanReload(stateValue *cronStateFile, ownerView cronOwnerView) bool {
	if !cronStateHasBinding(stateValue) {
		return false
	}
	switch ownerView.Status {
	case cronOwnerStatusHealthy:
		return true
	default:
		return false
	}
}

func cronOwnerAllowsLoadedJobs(status cronOwnerStatus) bool {
	switch status {
	case cronOwnerStatusHealthy:
		return true
	default:
		return false
	}
}

func cronLoadedJobCountLine(stateValue *cronStateFile, ownerView cronOwnerView) string {
	if stateValue == nil {
		return ""
	}
	if !cronOwnerAllowsLoadedJobs(ownerView.Status) && cronStateHasBinding(stateValue) {
		return "当前任务：待修复后重新加载"
	}
	return fmt.Sprintf("当前已加载任务：%d 条", len(stateValue.Jobs))
}

func cronSortedJobs(jobs []cronJobState) []cronJobState {
	items := append([]cronJobState(nil), jobs...)
	sort.Slice(items, func(i, j int) bool {
		left := items[i].NextRunAt
		right := items[j].NextRunAt
		switch {
		case left.IsZero() && right.IsZero():
			return firstNonEmpty(items[i].Name, items[i].RecordID) < firstNonEmpty(items[j].Name, items[j].RecordID)
		case left.IsZero():
			return false
		case right.IsZero():
			return true
		case !left.Equal(right):
			return left.Before(right)
		default:
			return firstNonEmpty(items[i].Name, items[i].RecordID) < firstNonEmpty(items[j].Name, items[j].RecordID)
		}
	})
	return items
}

func cronLoadedJobEntries(jobs []cronJobState, timeZone string) []control.CommandCatalogEntry {
	items := cronSortedJobs(jobs)
	entries := make([]control.CommandCatalogEntry, 0, len(items))
	for _, job := range items {
		item := cronReloadTaskItemFromJob(job)
		segments := []string{}
		if schedule := cronReloadTaskScheduleText(item); schedule != "" {
			segments = append(segments, schedule)
		}
		if next := cronReloadTaskNextRunText(item, "下次", timeZone); next != "" {
			segments = append(segments, next)
		}
		segments = append(segments, cronJobConcurrencyText(job.MaxConcurrency))
		if source := strings.TrimSpace(cronJobDisplaySource(job)); source != "" {
			segments = append(segments, "来源："+source)
		}
		entry := control.CommandCatalogEntry{
			Title:       firstNonEmpty(strings.TrimSpace(job.Name), strings.TrimSpace(job.RecordID), "unnamed"),
			Description: strings.Join(segments, "｜"),
		}
		if commandText := cronRunCommandText(job.RecordID); commandText != "" {
			entry.Buttons = []control.CommandCatalogButton{
				runCommandButton("立即触发", commandText, "", false),
			}
		}
		entries = append(entries, entry)
	}
	return entries
}

func cronRunCommandText(jobRecordID string) string {
	jobRecordID = strings.TrimSpace(jobRecordID)
	if jobRecordID == "" {
		return ""
	}
	return "/cron run " + jobRecordID
}

func cronNoticeEvent(surfaceID, code, text string) control.UIEvent {
	return control.UIEvent{
		Kind:             control.UIEventNotice,
		SurfaceSessionID: surfaceID,
		Notice: &control.Notice{
			Code:  code,
			Title: "Cron",
			Text:  text,
		},
	}
}

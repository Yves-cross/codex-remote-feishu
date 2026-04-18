package orchestrator

import (
	"fmt"
	"strings"

	"github.com/kxn/codex-remote-feishu/internal/core/agentproto"
	"github.com/kxn/codex-remote-feishu/internal/core/control"
)

const debugErrorNoticeCode = "debug_error"

func NoticeForProblem(problem agentproto.ErrorInfo) control.Notice {
	problem = problem.Normalize()
	title := "链路错误"
	if location := problemLocation(problem); location != "" {
		title += " · " + location
	}

	lines := make([]string, 0, 10)
	if problem.Layer != "" {
		lines = append(lines, "层：`"+problem.Layer+"`")
	}
	if problem.Stage != "" {
		lines = append(lines, "位置：`"+problem.Stage+"`")
	}
	if problem.Operation != "" {
		lines = append(lines, "操作：`"+problem.Operation+"`")
	}
	if problem.CommandID != "" {
		lines = append(lines, "命令：`"+problem.CommandID+"`")
	}
	if problem.ThreadID != "" {
		lines = append(lines, "会话：`"+problem.ThreadID+"`")
	}
	if problem.TurnID != "" {
		lines = append(lines, "Turn：`"+problem.TurnID+"`")
	}
	if problem.Code != "" {
		lines = append(lines, "错误码：`"+problem.Code+"`")
	}
	if problem.Retryable {
		lines = append(lines, "可重试：是")
	}

	if problem.Message != "" {
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, "摘要："+problem.Message)
	}
	if details := problemDetails(problem); details != "" {
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, "调试信息：\n```text\n"+details+"\n```")
	}
	if len(lines) == 0 {
		lines = append(lines, "发生了未分类的链路错误。")
	}

	return control.Notice{
		Code:     debugErrorNoticeCode,
		Title:    title,
		Text:     strings.TrimSpace(strings.Join(lines, "\n")),
		ThemeKey: "error",
		Sections: problemNoticeSections(problem),
	}
}

func problemNoticeSections(problem agentproto.ErrorInfo) []control.FeishuCardTextSection {
	sections := make([]control.FeishuCardTextSection, 0, 3)
	infoLines := make([]string, 0, 8)
	if problem.Layer != "" {
		infoLines = append(infoLines, "层："+problem.Layer)
	}
	if problem.Stage != "" {
		infoLines = append(infoLines, "位置："+problem.Stage)
	}
	if problem.Operation != "" {
		infoLines = append(infoLines, "操作："+problem.Operation)
	}
	if problem.CommandID != "" {
		infoLines = append(infoLines, "命令："+problem.CommandID)
	}
	if problem.ThreadID != "" {
		infoLines = append(infoLines, "会话："+problem.ThreadID)
	}
	if problem.TurnID != "" {
		infoLines = append(infoLines, "Turn："+problem.TurnID)
	}
	if problem.Code != "" {
		infoLines = append(infoLines, "错误码："+problem.Code)
	}
	if problem.Retryable {
		infoLines = append(infoLines, "可重试：是")
	}
	if len(infoLines) != 0 {
		sections = append(sections, control.FeishuCardTextSection{
			Label: "链路信息",
			Lines: infoLines,
		})
	}

	if message := strings.TrimSpace(problem.Message); message != "" {
		sections = append(sections, control.FeishuCardTextSection{
			Label: "摘要",
			Lines: []string{message},
		})
	}
	if details := problemDetails(problem); details != "" {
		sections = append(sections, control.FeishuCardTextSection{
			Label: "调试信息",
			Lines: []string{details},
		})
	}
	if len(sections) == 0 {
		sections = append(sections, control.FeishuCardTextSection{
			Lines: []string{"发生了未分类的链路错误。"},
		})
	}
	return sections
}

func problemLocation(problem agentproto.ErrorInfo) string {
	switch {
	case problem.Layer != "" && problem.Stage != "":
		return problem.Layer + "." + problem.Stage
	case problem.Layer != "":
		return problem.Layer
	case problem.Stage != "":
		return problem.Stage
	default:
		return ""
	}
}

func problemDetails(problem agentproto.ErrorInfo) string {
	details := strings.TrimSpace(problem.Details)
	if details == "" || details == strings.TrimSpace(problem.Message) {
		return ""
	}
	const maxLen = 1500
	if len(details) <= maxLen {
		return details
	}
	return fmt.Sprintf("%s\n...(%d bytes truncated)", details[:maxLen], len(details)-maxLen)
}

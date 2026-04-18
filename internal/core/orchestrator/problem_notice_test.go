package orchestrator

import (
	"strings"
	"testing"

	"github.com/kxn/codex-remote-feishu/internal/core/agentproto"
)

func TestNoticeForProblemBuildsStructuredSections(t *testing.T) {
	notice := NoticeForProblem(agentproto.ErrorInfo{
		Layer:     "wrapper",
		Stage:     "observe_codex_stdout",
		Operation: "codex.stdout",
		Code:      "stdout_parse_failed",
		Message:   "wrapper 无法解析 Codex 子进程输出的 JSON-RPC 帧。",
		Details:   "invalid character 'x' looking for beginning of value",
		Retryable: true,
	})

	if len(notice.Sections) != 3 {
		t.Fatalf("expected structured sections, got %#v", notice)
	}
	if notice.Sections[0].Label != "链路信息" {
		t.Fatalf("expected info section first, got %#v", notice.Sections)
	}
	if got := strings.Join(notice.Sections[0].Lines, "\n"); !strings.Contains(got, "层：wrapper") || !strings.Contains(got, "位置：observe_codex_stdout") || !strings.Contains(got, "错误码：stdout_parse_failed") {
		t.Fatalf("expected problem metadata in structured section, got %q", got)
	}
	if notice.Sections[1].Label != "摘要" || len(notice.Sections[1].Lines) != 1 || notice.Sections[1].Lines[0] != "wrapper 无法解析 Codex 子进程输出的 JSON-RPC 帧。" {
		t.Fatalf("expected summary section, got %#v", notice.Sections[1])
	}
	if notice.Sections[2].Label != "调试信息" || len(notice.Sections[2].Lines) != 1 || !strings.Contains(notice.Sections[2].Lines[0], "invalid character") {
		t.Fatalf("expected details section, got %#v", notice.Sections[2])
	}
	if !strings.Contains(notice.Text, "错误码：`stdout_parse_failed`") || !strings.Contains(notice.Text, "```text") {
		t.Fatalf("expected compatibility text to stay populated, got %#v", notice.Text)
	}
}

package feishu

import (
	"testing"

	"github.com/kxn/codex-remote-feishu/internal/core/control"
)

func TestProjectSelectionPromptKeepsMarkdownMetacharactersInsidePlainTextDetailBlock(t *testing.T) {
	projector := NewProjector()
	ops := projector.Project("chat-1", control.UIEvent{
		Kind: control.UIEventFeishuDirectSelectionPrompt,
		FeishuDirectSelectionPrompt: &control.FeishuDirectSelectionPrompt{
			Kind:  control.SelectionPromptUseThread,
			Title: "最近会话",
			Options: []control.SelectionOption{{
				Index:       1,
				OptionID:    "thread-1",
				Label:       "修复 `登录`",
				ButtonLabel: "修复 `登录`",
				Subtitle:    "/data/dl/#demo\n- 列表项\n[本地链接](docs/demo.md)",
			}},
		},
	})
	if len(ops) != 1 || ops[0].Kind != OperationSendCard {
		t.Fatalf("unexpected ops: %#v", ops)
	}
	if len(ops[0].CardElements) != 3 {
		t.Fatalf("unexpected selection elements: %#v", ops[0].CardElements)
	}
	detail := plainTextContent(ops[0].CardElements[2])
	if !containsAll(detail,
		"/data/dl/#demo",
		"- 列表项",
		"[本地链接](docs/demo.md)",
	) {
		t.Fatalf("expected selection detail to preserve raw dynamic text in plain_text, got %q", detail)
	}
	if markdownContent(ops[0].CardElements[2]) != "" {
		t.Fatalf("expected selection detail to stop using markdown element, got %#v", ops[0].CardElements[2])
	}
}

func TestProjectThreadSelectionChangeKeepsMarkdownMetacharactersInsidePlainTextBlock(t *testing.T) {
	projector := NewProjector()
	ops := projector.Project("chat-1", control.UIEvent{
		Kind: control.UIEventThreadSelectionChange,
		ThreadSelection: &control.ThreadSelectionChanged{
			Title:                "dl · `修复登录`",
			LastAssistantMessage: "# 标题\n- 列表项\n[预览](docs/demo.md)",
		},
	})
	if len(ops) != 1 || ops[0].Kind != OperationSendCard {
		t.Fatalf("unexpected ops: %#v", ops)
	}
	if ops[0].CardBody != "" {
		t.Fatalf("expected thread selection change to keep markdown body empty, got %#v", ops[0])
	}
	if len(ops[0].CardElements) != 1 {
		t.Fatalf("unexpected thread selection change elements: %#v", ops[0].CardElements)
	}
	text := plainTextContent(ops[0].CardElements[0])
	if !containsAll(text,
		"当前输入目标已切换到：dl · `修复登录`",
		"最近回复：",
		"# 标题",
		"- 列表项",
		"[预览](docs/demo.md)",
	) {
		t.Fatalf("expected thread selection change to preserve raw dynamic text in plain_text, got %q", text)
	}
	if markdownContent(ops[0].CardElements[0]) != "" {
		t.Fatalf("expected thread selection change to stop using markdown element, got %#v", ops[0].CardElements[0])
	}
}

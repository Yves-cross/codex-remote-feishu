package feishu

import (
	"testing"

	"github.com/kxn/codex-remote-feishu/internal/core/control"
)

func TestProjectRequestUserInputPromptKeepsMarkdownMetacharactersInsidePlainTextQuestionBlock(t *testing.T) {
	projector := NewProjector()
	ops := projector.Project("chat-1", control.UIEvent{
		Kind: control.UIEventFeishuDirectRequestPrompt,
		FeishuDirectRequestPrompt: &control.FeishuDirectRequestPrompt{
			RequestID:   "req-ui-unsafe",
			RequestType: "request_user_input",
			Questions: []control.RequestPromptQuestion{
				{
					ID:           "notes",
					Header:       "# 标题",
					Question:     "请原样保留：\n- 列表项\n[链接](local.md)\n```go\nfmt.Println(1)\n```",
					Answered:     true,
					DefaultValue: "`rm -rf /`",
					AllowOther:   true,
					Options: []control.RequestPromptQuestionOption{
						{Label: "- 选项A", Description: "[描述](demo.md)"},
					},
				},
			},
		},
	})
	if len(ops) != 1 || ops[0].Kind != OperationSendCard {
		t.Fatalf("unexpected ops: %#v", ops)
	}
	question := plainTextContent(ops[0].CardElements[1])
	if !containsAll(question,
		"问题 1：# 标题",
		"请原样保留：",
		"- 列表项",
		"[链接](local.md)",
		"```go",
		"当前答案：`rm -rf /`",
		"- - 选项A：[描述](demo.md)",
	) {
		t.Fatalf("expected question block to preserve raw dynamic text inside plain_text, got %q", question)
	}
	if markdownContent(ops[0].CardElements[1]) != "" {
		t.Fatalf("expected question block to stop using markdown element, got %#v", ops[0].CardElements[1])
	}
	rendered := renderedV2BodyElements(t, ops[0])
	if plainTextContent(rendered[2]) == "" {
		t.Fatalf("expected rendered V2 card to keep plain_text question block, got %#v", rendered[2])
	}
}

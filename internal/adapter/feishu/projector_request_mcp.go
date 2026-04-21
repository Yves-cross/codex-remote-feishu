package feishu

import (
	"fmt"
	"strings"

	"github.com/kxn/codex-remote-feishu/internal/core/control"
)

func permissionsRequestPromptElements(prompt control.FeishuRequestView, daemonLifecycleID string) []map[string]any {
	options := prompt.Options
	if len(options) == 0 {
		options = []control.RequestPromptOption{
			{OptionID: "accept", Label: "允许本次", Style: "primary"},
			{OptionID: "acceptForSession", Label: "本会话允许", Style: "default"},
			{OptionID: "decline", Label: "拒绝", Style: "default"},
		}
	}
	actions := make([]map[string]any, 0, len(options))
	for _, option := range options {
		button := requestPromptButton(prompt, option, daemonLifecycleID)
		if len(button) == 0 {
			continue
		}
		actions = append(actions, button)
	}
	elements := make([]map[string]any, 0, 2)
	if row := cardButtonGroupElement(actions); len(row) != 0 {
		elements = append(elements, row)
	}
	elements = append(elements, map[string]any{
		"tag":     "markdown",
		"content": "你可以选择仅授权当前这一次，或在当前会话内持续授权。",
	})
	return elements
}

func mcpElicitationPromptElements(prompt control.FeishuRequestView, daemonLifecycleID string) []map[string]any {
	if len(prompt.Questions) == 0 {
		return mcpElicitationChoiceElements(prompt, daemonLifecycleID)
	}
	elements := make([]map[string]any, 0, 9)
	if progress := mcpElicitationProgressMarkdown(prompt); progress != "" {
		elements = append(elements, map[string]any{
			"tag":     "markdown",
			"content": progress,
		})
	}
	elements = appendCurrentRequestQuestionElements(elements, prompt, daemonLifecycleID)
	if hint := mcpElicitationQuestionHint(prompt); hint != "" {
		elements = append(elements, map[string]any{
			"tag":     "markdown",
			"content": hint,
		})
	}
	if requestPromptNeedsForm(prompt) {
		if form := requestPromptFormElement(prompt, daemonLifecycleID); len(form) != 0 {
			elements = append(elements, form)
		}
	}
	if row := requestPromptNavigationActionRow(prompt, daemonLifecycleID); len(row) != 0 {
		elements = append(elements, row)
	}
	if row := requestPromptSubmitActionRow(prompt, daemonLifecycleID); len(row) != 0 {
		elements = append(elements, row)
	}
	if row := mcpElicitationTerminalActionRow(prompt, daemonLifecycleID); len(row) != 0 {
		elements = append(elements, row)
	}
	return elements
}

func mcpElicitationChoiceElements(prompt control.FeishuRequestView, daemonLifecycleID string) []map[string]any {
	options := prompt.Options
	if len(options) == 0 {
		options = []control.RequestPromptOption{
			{OptionID: "accept", Label: "继续", Style: "primary"},
			{OptionID: "decline", Label: "拒绝", Style: "default"},
			{OptionID: "cancel", Label: "取消", Style: "default"},
		}
	}
	actions := make([]map[string]any, 0, len(options))
	for _, option := range options {
		button := requestPromptButton(prompt, option, daemonLifecycleID)
		if len(button) == 0 {
			continue
		}
		actions = append(actions, button)
	}
	elements := make([]map[string]any, 0, 2)
	if row := cardButtonGroupElement(actions); len(row) != 0 {
		elements = append(elements, row)
	}
	elements = append(elements, map[string]any{
		"tag":     "markdown",
		"content": "如果需要先完成外部页面操作，请完成后再点击“继续”；如果不打算继续，可直接拒绝或取消。",
	})
	return elements
}

func mcpElicitationTerminalActionRow(prompt control.FeishuRequestView, daemonLifecycleID string) map[string]any {
	return cardButtonGroupElement([]map[string]any{
		cardCallbackButtonElement("拒绝", "default", stampActionValue(map[string]any{
			cardActionPayloadKeyKind:            cardActionKindRequestRespond,
			cardActionPayloadKeyRequestID:       prompt.RequestID,
			cardActionPayloadKeyRequestType:     strings.TrimSpace(prompt.RequestType),
			cardActionPayloadKeyRequestOptionID: "decline",
			cardActionPayloadKeyRequestRevision: prompt.RequestRevision,
		}, daemonLifecycleID), false, "fill"),
		cardCallbackButtonElement("取消", "default", stampActionValue(map[string]any{
			cardActionPayloadKeyKind:            cardActionKindRequestRespond,
			cardActionPayloadKeyRequestID:       prompt.RequestID,
			cardActionPayloadKeyRequestType:     strings.TrimSpace(prompt.RequestType),
			cardActionPayloadKeyRequestOptionID: "cancel",
			cardActionPayloadKeyRequestRevision: prompt.RequestRevision,
		}, daemonLifecycleID), false, "fill"),
	})
}

func mcpElicitationProgressMarkdown(prompt control.FeishuRequestView) string {
	if len(prompt.Questions) == 0 {
		return ""
	}
	answered := 0
	for _, question := range prompt.Questions {
		if question.Answered {
			answered++
		}
	}
	return fmt.Sprintf("**填写进度** %d/%d · 当前第 %d 题", answered, len(prompt.Questions), normalizedRequestPromptCurrentQuestionIndex(prompt)+1)
}

func mcpElicitationQuestionHint(prompt control.FeishuRequestView) string {
	question, _, ok := requestPromptCurrentQuestion(prompt)
	if !ok {
		return ""
	}
	if question.DirectResponse && requestPromptQuestionNeedsFormInput(question) {
		return "当前题可先点选，也可补充文字或 JSON；可用“上一题 / 下一题”切换，确认无误后点击“提交并继续”。"
	}
	if question.DirectResponse {
		return "当前题可直接点选字段值；可用“上一题 / 下一题”切换，确认无误后点击“提交并继续”。"
	}
	return "填写当前题后先保存本题；可用“上一题 / 下一题”切换，确认无误后点击“提交并继续”。"
}

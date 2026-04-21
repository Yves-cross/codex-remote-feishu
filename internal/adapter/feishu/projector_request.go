package feishu

import (
	"fmt"
	"strings"

	"github.com/kxn/codex-remote-feishu/internal/core/control"
)

const (
	requestUserInputSubmitOptionID                      = "submit"
	requestUserInputConfirmSubmitWithUnansweredOptionID = "confirm_submit_with_unanswered"
	requestUserInputCancelSubmitWithUnansweredOptionID  = "cancel_submit_with_unanswered"
	requestPromptStepPreviousOptionID                   = "step_previous"
	requestPromptStepNextOptionID                       = "step_next"
	requestPromptStepSaveOptionID                       = "step_save"
)

func requestPromptSections(prompt control.FeishuRequestView) []control.FeishuCardTextSection {
	sections := make([]control.FeishuCardTextSection, 0, len(prompt.Sections)+1)
	if threadTitle := strings.TrimSpace(prompt.ThreadTitle); threadTitle != "" {
		sections = append(sections, control.FeishuCardTextSection{
			Lines: []string{"当前会话：" + threadTitle},
		})
	}
	for _, section := range prompt.Sections {
		if normalized := section.Normalized(); normalized.Label != "" || len(normalized.Lines) != 0 {
			sections = append(sections, normalized)
		}
	}
	return sections
}

func requestPromptElements(prompt control.FeishuRequestView, daemonLifecycleID string) []map[string]any {
	elements := appendCardTextSections(nil, requestPromptSections(prompt))
	switch normalizeRequestPromptType(prompt.RequestType) {
	case "request_user_input":
		if len(prompt.Questions) != 0 {
			return append(elements, requestUserInputPromptElements(prompt, daemonLifecycleID)...)
		}
	case "permissions_request_approval":
		return append(elements, permissionsRequestPromptElements(prompt, daemonLifecycleID)...)
	case "mcp_server_elicitation":
		return append(elements, mcpElicitationPromptElements(prompt, daemonLifecycleID)...)
	}
	if normalizeRequestPromptType(prompt.RequestType) == "request_user_input" && len(prompt.Questions) != 0 {
		return append(elements, requestUserInputPromptElements(prompt, daemonLifecycleID)...)
	}
	options := prompt.Options
	if len(options) == 0 {
		options = []control.RequestPromptOption{
			{OptionID: "accept", Label: "允许一次", Style: "primary"},
			{OptionID: "decline", Label: "拒绝", Style: "default"},
			{OptionID: "captureFeedback", Label: "告诉 Codex 怎么改", Style: "default"},
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
	hint := "这个确认只影响当前这一次请求。"
	if requestPromptContainsOption(options, "captureFeedback") {
		hint = "如果想拒绝并补充处理意见，请点击“告诉 Codex 怎么改”后再发送下一条文字。"
	}
	if group := cardButtonGroupElement(actions); len(group) != 0 {
		elements = append(elements, group)
	}
	elements = append(elements, map[string]any{
		"tag":     "markdown",
		"content": hint,
	})
	return elements
}

func requestUserInputPromptElements(prompt control.FeishuRequestView, daemonLifecycleID string) []map[string]any {
	elements := make([]map[string]any, 0, 8)
	if progress := requestPromptProgressMarkdown(prompt); progress != "" {
		elements = append(elements, map[string]any{
			"tag":     "markdown",
			"content": progress,
		})
	}
	if prompt.SubmitWithUnansweredConfirmPending {
		if markdown := requestPromptSubmitConfirmMarkdown(prompt); markdown != "" {
			elements = append(elements, map[string]any{
				"tag":     "markdown",
				"content": markdown,
			})
		}
		if row := requestPromptSubmitConfirmActionRow(prompt, daemonLifecycleID); len(row) != 0 {
			elements = append(elements, row)
		}
		return elements
	}
	elements = appendCurrentRequestQuestionElements(elements, prompt, daemonLifecycleID)
	if hint := requestPromptQuestionHint(prompt); hint != "" {
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
	if extra := requestUserInputExtraActionRow(prompt, daemonLifecycleID); len(extra) != 0 {
		elements = append(elements, extra)
	}
	return elements
}

func appendCurrentRequestQuestionElements(elements []map[string]any, prompt control.FeishuDirectRequestPrompt, daemonLifecycleID string) []map[string]any {
	question, index, ok := requestPromptCurrentQuestion(prompt)
	if !ok {
		return elements
	}
	if section, ok := requestPromptQuestionSection(index, len(prompt.Questions), question); ok {
		elements = appendCardTextSections(elements, []control.FeishuCardTextSection{section})
	}
	if question.DirectResponse && len(question.Options) != 0 {
		actions := make([]map[string]any, 0, len(question.Options))
		for _, option := range question.Options {
			button := requestUserInputOptionButton(prompt, question, option, daemonLifecycleID)
			if len(button) == 0 {
				continue
			}
			actions = append(actions, button)
		}
		if group := cardButtonGroupElement(actions); len(group) != 0 {
			elements = append(elements, group)
		}
	}
	return elements
}

func requestUserInputExtraActionRow(prompt control.FeishuRequestView, daemonLifecycleID string) map[string]any {
	if prompt.SubmitWithUnansweredConfirmPending || len(prompt.Options) == 0 {
		return nil
	}
	actions := make([]map[string]any, 0, len(prompt.Options))
	for _, option := range prompt.Options {
		button := requestPromptButton(prompt, option, daemonLifecycleID)
		if len(button) == 0 {
			continue
		}
		actions = append(actions, button)
	}
	return cardButtonGroupElement(actions)
}

func requestPromptButton(prompt control.FeishuRequestView, option control.RequestPromptOption, daemonLifecycleID string) map[string]any {
	label := strings.TrimSpace(option.Label)
	if label == "" {
		return nil
	}
	buttonType := strings.TrimSpace(option.Style)
	if buttonType == "" {
		buttonType = "default"
	}
	return cardCallbackButtonElement(label, buttonType, stampActionValue(map[string]any{
		cardActionPayloadKeyKind:            cardActionKindRequestRespond,
		cardActionPayloadKeyRequestID:       prompt.RequestID,
		cardActionPayloadKeyRequestType:     strings.TrimSpace(prompt.RequestType),
		cardActionPayloadKeyRequestOptionID: strings.TrimSpace(option.OptionID),
		cardActionPayloadKeyRequestRevision: prompt.RequestRevision,
	}, daemonLifecycleID), false, "")
}

func requestUserInputOptionButton(prompt control.FeishuRequestView, question control.RequestPromptQuestion, option control.RequestPromptQuestionOption, daemonLifecycleID string) map[string]any {
	label := strings.TrimSpace(option.Label)
	if label == "" {
		return nil
	}
	buttonType := "primary"
	selectedAnswer := strings.TrimSpace(question.DefaultValue)
	if selectedAnswer != "" && !strings.EqualFold(selectedAnswer, label) {
		buttonType = "default"
	}
	return cardCallbackButtonElement(label, buttonType, stampActionValue(map[string]any{
		cardActionPayloadKeyKind:            cardActionKindRequestRespond,
		cardActionPayloadKeyRequestID:       prompt.RequestID,
		cardActionPayloadKeyRequestType:     strings.TrimSpace(prompt.RequestType),
		cardActionPayloadKeyRequestRevision: prompt.RequestRevision,
		cardActionPayloadKeyRequestAnswers: map[string]any{
			strings.TrimSpace(question.ID): []any{label},
		},
	}, daemonLifecycleID), false, "fill")
}

func stampActionValue(value map[string]any, daemonLifecycleID string) map[string]any {
	return actionPayloadWithLifecycle(value, daemonLifecycleID)
}

func requestPromptContainsOption(options []control.RequestPromptOption, optionID string) bool {
	for _, option := range options {
		if strings.TrimSpace(option.OptionID) == optionID {
			return true
		}
	}
	return false
}

func requestPromptQuestionSection(index, total int, question control.RequestPromptQuestion) (control.FeishuCardTextSection, bool) {
	lines := make([]string, 0, 10)
	title := firstNonEmpty(strings.TrimSpace(question.Header), strings.TrimSpace(question.Question))
	if title != "" {
		lines = append(lines, "标题："+title)
	}
	if question.Answered {
		lines = append(lines, "状态：已回答")
	} else {
		lines = append(lines, "状态：待回答")
	}
	if question.Header != "" && strings.TrimSpace(question.Question) != "" && strings.TrimSpace(question.Question) != strings.TrimSpace(question.Header) {
		lines = append(lines, "")
		lines = append(lines, "说明：")
		lines = append(lines, strings.TrimSpace(question.Question))
	}
	if value := strings.TrimSpace(question.DefaultValue); value != "" {
		lines = append(lines, "当前答案："+value)
	}
	if len(question.Options) != 0 {
		lines = append(lines, "")
		lines = append(lines, "可选项：")
		for _, option := range question.Options {
			line := "- " + strings.TrimSpace(option.Label)
			if description := strings.TrimSpace(option.Description); description != "" {
				line += "：" + description
			}
			lines = append(lines, line)
		}
	}
	if question.AllowOther {
		lines = append(lines, "")
		lines = append(lines, "也可以直接填写其他答案。")
	}
	if question.Secret {
		lines = append(lines, "")
		lines = append(lines, "该答案按私密输入处理，不会在飞书卡片正文中回显。")
	}
	section := control.FeishuCardTextSection{
		Label: requestPromptQuestionLabel(index, total),
		Lines: lines,
	}.Normalized()
	if section.Label == "" && len(section.Lines) == 0 {
		return control.FeishuCardTextSection{}, false
	}
	return section, true
}

func requestPromptProgressMarkdown(prompt control.FeishuDirectRequestPrompt) string {
	if len(prompt.Questions) == 0 {
		return ""
	}
	answered := 0
	for _, question := range prompt.Questions {
		if question.Answered {
			answered++
		}
	}
	return fmt.Sprintf("**回答进度** %d/%d · 当前第 %d 题", answered, len(prompt.Questions), normalizedRequestPromptCurrentQuestionIndex(prompt)+1)
}

func requestPromptQuestionLabel(index, total int) string {
	if total <= 0 {
		return fmt.Sprintf("问题 %d", index+1)
	}
	return fmt.Sprintf("问题 %d/%d", index+1, total)
}

func normalizedRequestPromptCurrentQuestionIndex(prompt control.FeishuDirectRequestPrompt) int {
	if len(prompt.Questions) == 0 {
		return 0
	}
	if prompt.CurrentQuestionIndex < 0 {
		return 0
	}
	if prompt.CurrentQuestionIndex >= len(prompt.Questions) {
		return len(prompt.Questions) - 1
	}
	return prompt.CurrentQuestionIndex
}

func requestPromptCurrentQuestion(prompt control.FeishuDirectRequestPrompt) (control.RequestPromptQuestion, int, bool) {
	if len(prompt.Questions) == 0 {
		return control.RequestPromptQuestion{}, 0, false
	}
	index := normalizedRequestPromptCurrentQuestionIndex(prompt)
	return prompt.Questions[index], index, true
}

func requestPromptNeedsForm(prompt control.FeishuDirectRequestPrompt) bool {
	question, _, ok := requestPromptCurrentQuestion(prompt)
	if !ok {
		return false
	}
	return requestPromptQuestionNeedsFormInput(question)
}

func requestPromptQuestionNeedsFormInput(question control.RequestPromptQuestion) bool {
	return len(question.Options) == 0 || question.AllowOther || !question.DirectResponse
}

func requestPromptFormElement(prompt control.FeishuDirectRequestPrompt, daemonLifecycleID string) map[string]any {
	question, _, ok := requestPromptCurrentQuestion(prompt)
	if !ok || !requestPromptQuestionNeedsFormInput(question) {
		return nil
	}
	name := strings.TrimSpace(question.ID)
	if name == "" {
		return nil
	}
	elements := make([]map[string]any, 0, 2)
	input := map[string]any{
		"tag":  "input",
		"name": name,
	}
	label := firstNonEmpty(strings.TrimSpace(question.Header), strings.TrimSpace(question.Question), name)
	input["label"] = map[string]any{
		"tag":     "plain_text",
		"content": label,
	}
	input["label_position"] = "left"
	if placeholder := strings.TrimSpace(question.Placeholder); placeholder != "" {
		input["placeholder"] = map[string]any{
			"tag":     "plain_text",
			"content": placeholder,
		}
	}
	if value := strings.TrimSpace(question.DefaultValue); value != "" {
		input["default_value"] = value
	}
	elements = append(elements, input)
	elements = append(elements, cardFormSubmitButtonElement(requestPromptStepSaveLabel(prompt), stampActionValue(map[string]any{
		cardActionPayloadKeyKind:            cardActionKindSubmitRequestForm,
		cardActionPayloadKeyRequestID:       prompt.RequestID,
		cardActionPayloadKeyRequestType:     strings.TrimSpace(prompt.RequestType),
		cardActionPayloadKeyRequestOptionID: requestPromptStepSaveOptionID,
		cardActionPayloadKeyRequestRevision: prompt.RequestRevision,
	}, daemonLifecycleID)))
	return map[string]any{
		"tag":      "form",
		"name":     "request_form_" + strings.TrimSpace(prompt.RequestID),
		"elements": elements,
	}
}

func requestPromptSubmitActionRow(prompt control.FeishuDirectRequestPrompt, daemonLifecycleID string) map[string]any {
	if len(prompt.Questions) == 0 || prompt.SubmitWithUnansweredConfirmPending {
		return nil
	}
	return cardButtonGroupElement([]map[string]any{
		cardCallbackButtonElement(requestPromptSubmitLabel(prompt), "primary", stampActionValue(map[string]any{
			cardActionPayloadKeyKind:            cardActionKindRequestRespond,
			cardActionPayloadKeyRequestID:       prompt.RequestID,
			cardActionPayloadKeyRequestType:     strings.TrimSpace(prompt.RequestType),
			cardActionPayloadKeyRequestOptionID: requestUserInputSubmitOptionID,
			cardActionPayloadKeyRequestRevision: prompt.RequestRevision,
		}, daemonLifecycleID), false, "fill"),
	})
}

func requestPromptNavigationActionRow(prompt control.FeishuDirectRequestPrompt, daemonLifecycleID string) map[string]any {
	if len(prompt.Questions) <= 1 || prompt.SubmitWithUnansweredConfirmPending {
		return nil
	}
	currentIndex := normalizedRequestPromptCurrentQuestionIndex(prompt)
	return cardButtonGroupElement([]map[string]any{
		cardCallbackButtonElement("上一题", "default", stampActionValue(map[string]any{
			cardActionPayloadKeyKind:            cardActionKindRequestRespond,
			cardActionPayloadKeyRequestID:       prompt.RequestID,
			cardActionPayloadKeyRequestType:     strings.TrimSpace(prompt.RequestType),
			cardActionPayloadKeyRequestOptionID: requestPromptStepPreviousOptionID,
			cardActionPayloadKeyRequestRevision: prompt.RequestRevision,
		}, daemonLifecycleID), currentIndex == 0, "fill"),
		cardCallbackButtonElement("下一题", "default", stampActionValue(map[string]any{
			cardActionPayloadKeyKind:            cardActionKindRequestRespond,
			cardActionPayloadKeyRequestID:       prompt.RequestID,
			cardActionPayloadKeyRequestType:     strings.TrimSpace(prompt.RequestType),
			cardActionPayloadKeyRequestOptionID: requestPromptStepNextOptionID,
			cardActionPayloadKeyRequestRevision: prompt.RequestRevision,
		}, daemonLifecycleID), currentIndex >= len(prompt.Questions)-1, "fill"),
	})
}

func requestPromptSubmitLabel(prompt control.FeishuDirectRequestPrompt) string {
	if normalizeRequestPromptType(prompt.RequestType) == "mcp_server_elicitation" {
		return "提交并继续"
	}
	return "提交答案"
}

func requestPromptStepSaveLabel(prompt control.FeishuDirectRequestPrompt) string {
	if normalizeRequestPromptType(prompt.RequestType) == "mcp_server_elicitation" {
		return "保存本题"
	}
	return "保存本题"
}

func requestPromptSubmitConfirmActionRow(prompt control.FeishuDirectRequestPrompt, daemonLifecycleID string) map[string]any {
	return cardButtonGroupElement([]map[string]any{
		cardCallbackButtonElement("继续补答", "default", stampActionValue(map[string]any{
			cardActionPayloadKeyKind:            cardActionKindRequestRespond,
			cardActionPayloadKeyRequestID:       prompt.RequestID,
			cardActionPayloadKeyRequestType:     strings.TrimSpace(prompt.RequestType),
			cardActionPayloadKeyRequestOptionID: requestUserInputCancelSubmitWithUnansweredOptionID,
			cardActionPayloadKeyRequestRevision: prompt.RequestRevision,
		}, daemonLifecycleID), false, "fill"),
		cardCallbackButtonElement("确认提交已有答案", "primary", stampActionValue(map[string]any{
			cardActionPayloadKeyKind:            cardActionKindRequestRespond,
			cardActionPayloadKeyRequestID:       prompt.RequestID,
			cardActionPayloadKeyRequestType:     strings.TrimSpace(prompt.RequestType),
			cardActionPayloadKeyRequestOptionID: requestUserInputConfirmSubmitWithUnansweredOptionID,
			cardActionPayloadKeyRequestRevision: prompt.RequestRevision,
		}, daemonLifecycleID), false, "fill"),
	})
}

func requestPromptSubmitConfirmMarkdown(prompt control.FeishuDirectRequestPrompt) string {
	missing := len(prompt.SubmitWithUnansweredMissingLabels)
	switch {
	case missing <= 0:
		return "仍有未答题。是否提交已有答案？"
	case missing == 1:
		return fmt.Sprintf("仍有 1 个问题未回答：%s。是否提交已有答案？", prompt.SubmitWithUnansweredMissingLabels[0])
	default:
		return fmt.Sprintf("仍有 %d 个问题未回答。是否提交已有答案？", missing)
	}
}

func requestPromptQuestionHint(prompt control.FeishuDirectRequestPrompt) string {
	if prompt.SubmitWithUnansweredConfirmPending {
		return "你可以继续补答，也可以确认提交已有答案（未回答的问题将按留空提交）。"
	}
	question, _, ok := requestPromptCurrentQuestion(prompt)
	if !ok {
		return ""
	}
	if question.DirectResponse && requestPromptQuestionNeedsFormInput(question) {
		return "当前题可直接点选，也可填写其他答案；可用“上一题 / 下一题”切换，准备结束时点击“提交答案”。"
	}
	if question.DirectResponse {
		return "当前题可直接点选答案；可用“上一题 / 下一题”切换，准备结束时点击“提交答案”。"
	}
	return "填写当前题后先保存本题；可用“上一题 / 下一题”切换，准备结束时点击“提交答案”。"
}

func normalizeRequestPromptType(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch {
	case normalized == "", normalized == "approval":
		return "approval"
	case strings.HasPrefix(normalized, "approval"):
		return "approval"
	default:
		return normalized
	}
}

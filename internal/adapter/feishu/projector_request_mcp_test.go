package feishu

import (
	"strings"
	"testing"

	"github.com/kxn/codex-remote-feishu/internal/core/control"
)

func TestProjectPermissionsRequestPromptAsCard(t *testing.T) {
	projector := NewProjector()
	ops := projector.Project("chat-1", requestPromptEvent(control.FeishuRequestView{
		RequestID:   "req-perm-1",
		RequestType: "permissions_request_approval",
		Title:       "需要授予权限",
		Sections: []control.FeishuCardTextSection{
			{Lines: []string{"本地 Codex 正在等待授予附加权限。"}},
			{Label: "申请权限", Lines: []string{"- Read docs (`docs.read`)"}},
		},
		Options: []control.RequestPromptOption{
			{OptionID: "accept", Label: "允许本次", Style: "primary"},
			{OptionID: "acceptForSession", Label: "本会话允许", Style: "default"},
			{OptionID: "decline", Label: "拒绝", Style: "default"},
		},
	}))

	if len(ops) != 1 || ops[0].Kind != OperationSendCard {
		t.Fatalf("unexpected ops: %#v", ops)
	}
	if ops[0].CardBody != "" {
		t.Fatalf("expected permissions prompt body to stay empty, got %#v", ops[0])
	}
	if got := plainTextContent(ops[0].CardElements[0]); !strings.Contains(got, "本地 Codex 正在等待授予附加权限。") {
		t.Fatalf("unexpected permissions intro section: %#v", ops[0].CardElements[0])
	}
	if got := plainTextContent(ops[0].CardElements[2]); !strings.Contains(got, "Read docs (`docs.read`)") {
		t.Fatalf("expected permission names to stay in plain_text, got %#v", ops[0].CardElements[2])
	}
	row := cardElementButtons(t, ops[0].CardElements[3])
	if len(row) != 3 {
		t.Fatalf("expected three permission action buttons, got %#v", ops[0].CardElements[3])
	}
	acceptValue := cardButtonPayload(t, row[0])
	sessionValue := cardButtonPayload(t, row[1])
	declineValue := cardButtonPayload(t, row[2])
	if acceptValue["request_option_id"] != "accept" || sessionValue["request_option_id"] != "acceptForSession" || declineValue["request_option_id"] != "decline" {
		t.Fatalf("unexpected permission request payloads: %#v %#v %#v", acceptValue, sessionValue, declineValue)
	}
	if got := markdownContent(ops[0].CardElements[4]); !strings.Contains(got, "当前会话内持续授权") {
		t.Fatalf("unexpected permission hint: %#v", ops[0].CardElements[4])
	}
}

func TestProjectMCPElicitationFormPromptAsCard(t *testing.T) {
	projector := NewProjector()
	ops := projector.Project("chat-1", requestPromptEvent(control.FeishuRequestView{
		RequestID:       "req-mcp-form-1",
		RequestType:     "mcp_server_elicitation",
		RequestRevision: 5,
		Title:           "需要处理 MCP 请求",
		Sections: []control.FeishuCardTextSection{{
			Lines: []string{"请补充返回内容", "MCP 服务：docs", "授权页面：https://example.com/approve?next=`token`"},
		}},
		Questions: []control.RequestPromptQuestion{
			{
				ID:             "mode",
				Header:         "模式",
				Question:       "选择执行模式（必填）",
				DirectResponse: true,
				Options: []control.RequestPromptQuestionOption{
					{Label: "auto"},
					{Label: "manual"},
				},
			},
			{
				ID:          "token",
				Header:      "Token",
				Question:    "填写 OAuth token（必填）",
				AllowOther:  true,
				Placeholder: "请填写 token",
			},
		},
		Options: []control.RequestPromptOption{
			{OptionID: "decline", Label: "拒绝", Style: "default"},
			{OptionID: "cancel", Label: "取消", Style: "default"},
		},
	}))

	if len(ops) != 1 || ops[0].Kind != OperationSendCard {
		t.Fatalf("unexpected ops: %#v", ops)
	}
	if ops[0].CardBody != "" {
		t.Fatalf("expected mcp prompt body to stay empty, got %#v", ops[0])
	}
	if got := plainTextContent(ops[0].CardElements[0]); !containsAll(got, "请补充返回内容", "MCP 服务：docs", "https://example.com/approve?next=`token`") {
		t.Fatalf("expected mcp intro section to stay plain_text, got %#v", ops[0].CardElements[0])
	}
	if got := markdownContent(ops[0].CardElements[1]); !strings.Contains(got, "填写进度") || !strings.Contains(got, "0/2") {
		t.Fatalf("expected mcp form progress markdown, got %#v", ops[0].CardElements[1])
	}
	if got := markdownContent(ops[0].CardElements[2]); !strings.Contains(got, "问题 1") {
		t.Fatalf("expected first mcp question heading, got %#v", ops[0].CardElements[2])
	}
	if got := plainTextContent(ops[0].CardElements[3]); !containsAll(got, "标题：模式", "说明：", "选择执行模式（必填）", "可选项：", "- auto") {
		t.Fatalf("expected first mcp question body to stay plain_text, got %#v", ops[0].CardElements[3])
	}
	optionRow := cardElementButtons(t, ops[0].CardElements[4])
	if len(optionRow) != 2 {
		t.Fatalf("expected direct response buttons for first field, got %#v", ops[0].CardElements[4])
	}
	optionValue := cardButtonPayload(t, optionRow[0])
	requestAnswers, _ := optionValue["request_answers"].(map[string]any)
	modeAnswers, _ := requestAnswers["mode"].([]any)
	if optionValue["kind"] != "request_respond" || len(modeAnswers) != 1 || modeAnswers[0] != "auto" {
		t.Fatalf("unexpected direct response payload: %#v", optionValue)
	}
	navRow := cardElementButtons(t, ops[0].CardElements[6])
	if len(navRow) != 2 {
		t.Fatalf("expected current-step navigation row, got %#v", ops[0].CardElements[6])
	}
	submitValue := cardButtonPayload(t, ops[0].CardElements[7])
	if submitValue["request_option_id"] != "submit" || submitValue["request_revision"] != 5 {
		t.Fatalf("unexpected mcp form submit payload: %#v", submitValue)
	}
	terminalRow := cardElementButtons(t, ops[0].CardElements[8])
	if len(terminalRow) != 2 {
		t.Fatalf("expected decline/cancel row, got %#v", ops[0].CardElements[8])
	}
	if got := cardButtonLabel(t, terminalRow[0]); got != "拒绝" {
		t.Fatalf("unexpected terminal action row: %#v", ops[0].CardElements[8])
	}
}

func TestProjectMCPElicitationFormPromptRendersCurrentFormFieldAsSingleStepForm(t *testing.T) {
	projector := NewProjector()
	ops := projector.Project("chat-1", requestPromptEvent(control.FeishuRequestView{
		RequestID:            "req-mcp-form-2",
		RequestType:          "mcp_server_elicitation",
		RequestRevision:      6,
		CurrentQuestionIndex: 1,
		Questions: []control.RequestPromptQuestion{
			{
				ID:             "mode",
				Header:         "模式",
				Question:       "选择执行模式（必填）",
				Answered:       true,
				DefaultValue:   "auto",
				DirectResponse: true,
				Options: []control.RequestPromptQuestionOption{
					{Label: "auto"},
					{Label: "manual"},
				},
			},
			{
				ID:          "token",
				Header:      "Token",
				Question:    "填写 OAuth token（必填）",
				AllowOther:  true,
				Placeholder: "请填写 token",
			},
		},
		Options: []control.RequestPromptOption{
			{OptionID: "decline", Label: "拒绝", Style: "default"},
			{OptionID: "cancel", Label: "取消", Style: "default"},
		},
	}))
	if len(ops) != 1 || ops[0].Kind != OperationSendCard {
		t.Fatalf("unexpected ops: %#v", ops)
	}
	if got := markdownContent(ops[0].CardElements[0]); !strings.Contains(got, "填写进度") || !strings.Contains(got, "1/2") || !strings.Contains(got, "当前第 2 题") {
		t.Fatalf("expected step-aware mcp progress, got %#v", ops[0].CardElements[0])
	}
	form := ops[0].CardElements[4]
	if form["tag"] != "form" {
		t.Fatalf("expected current-step mcp form, got %#v", form)
	}
	formElements, _ := form["elements"].([]map[string]any)
	if len(formElements) != 2 {
		t.Fatalf("expected one input and one save button, got %#v", form)
	}
	if label := cardButtonLabel(t, formElements[1]); label != "保存本题" {
		t.Fatalf("unexpected step-save label: %#v", formElements[1])
	}
	saveValue := cardButtonPayload(t, formElements[1])
	if saveValue["kind"] != "submit_request_form" || saveValue["request_option_id"] != "step_save" || saveValue["request_revision"] != 6 {
		t.Fatalf("unexpected step-save payload: %#v", saveValue)
	}
}

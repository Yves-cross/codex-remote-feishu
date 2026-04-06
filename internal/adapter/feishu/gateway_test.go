package feishu

import (
	"testing"

	"github.com/kxn/codex-remote-feishu/internal/core/control"
	larkcallback "github.com/larksuite/oapi-sdk-go/v3/event/dispatcher/callback"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

func TestMenuActionKindKnownValues(t *testing.T) {
	tests := map[string]control.ActionKind{
		"list":           control.ActionListInstances,
		"status":         control.ActionStatus,
		"stop":           control.ActionStop,
		"newinstance":    control.ActionNewInstance,
		"new_instance":   control.ActionNewInstance,
		"killinstance":   control.ActionKillInstance,
		"kill_instance":  control.ActionKillInstance,
		"threads":        control.ActionShowThreads,
		"sessions":       control.ActionShowThreads,
		"use":            control.ActionShowThreads,
		"show_threads":   control.ActionShowThreads,
		"show_sessions":  control.ActionShowThreads,
		"useall":         control.ActionShowAllThreads,
		"threads_all":    control.ActionShowAllThreads,
		"accessfull":     control.ActionAccessCommand,
		"access_full":    control.ActionAccessCommand,
		"accessconfirm":  control.ActionAccessCommand,
		"access_confirm": control.ActionAccessCommand,
	}
	for key, want := range tests {
		got, ok := menuActionKind(key)
		if !ok || got != want {
			t.Fatalf("event key %q => (%q, %v), want (%q, true)", key, got, ok, want)
		}
	}
}

func TestMenuActionReasoningPresets(t *testing.T) {
	tests := map[string]string{
		"reason_low":    "/reasoning low",
		"reasonlow":     "/reasoning low",
		"reason_medium": "/reasoning medium",
		"reasonmedium":  "/reasoning medium",
		"reason_high":   "/reasoning high",
		"reasonhigh":    "/reasoning high",
		"reason_xhigh":  "/reasoning xhigh",
		"reasonxhigh":   "/reasoning xhigh",
	}
	for key, wantText := range tests {
		got, ok := menuAction(key)
		if !ok {
			t.Fatalf("expected menu action for %q", key)
		}
		if got.Kind != control.ActionReasoningCommand || got.Text != wantText {
			t.Fatalf("event key %q => %#v, want reasoning command %q", key, got, wantText)
		}
	}
}

func TestMenuActionDynamicModelPreset(t *testing.T) {
	tests := map[string]string{
		"model_gpt-5.4":       "/model gpt-5.4",
		"model_gpt-5.4-mini":  "/model gpt-5.4-mini",
		"model-gpt-5.4":       "/model gpt-5.4",
		" model_gpt-5.4 \n\t": "/model gpt-5.4",
	}
	for key, wantText := range tests {
		got, ok := menuAction(key)
		if !ok {
			t.Fatalf("expected dynamic model action for %q", key)
		}
		if got.Kind != control.ActionModelCommand || got.Text != wantText {
			t.Fatalf("event key %q => %#v, want model command %q", key, got, wantText)
		}
	}
}

func TestMenuActionAccessPresets(t *testing.T) {
	tests := map[string]string{
		"accessfull":     "/access full",
		"access_full":    "/access full",
		"accessFull":     "/access full",
		"accessconfirm":  "/access confirm",
		"access_confirm": "/access confirm",
		"accessConfirm":  "/access confirm",
	}
	for key, wantText := range tests {
		got, ok := menuAction(key)
		if !ok {
			t.Fatalf("expected menu action for %q", key)
		}
		if got.Kind != control.ActionAccessCommand || got.Text != wantText {
			t.Fatalf("event key %q => %#v, want access command %q", key, got, wantText)
		}
	}
}

func TestNormalizeMenuEventKey(t *testing.T) {
	tests := map[string]string{
		"access_full":      "accessfull",
		"access-full":      "accessfull",
		" accessFull \n":   "accessfull",
		"show_all_threads": "showallthreads",
		"approval_confirm": "approvalconfirm",
		"reason_high":      "reasonhigh",
		"reason_xhigh":     "reasonxhigh",
	}
	for input, want := range tests {
		if got := normalizeMenuEventKey(input); got != want {
			t.Fatalf("normalizeMenuEventKey(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestMenuActionKindUnknownValueIsIgnored(t *testing.T) {
	got, ok := menuActionKind("unexpected")
	if ok || got != "" {
		t.Fatalf("unexpected menu action result: (%q, %v)", got, ok)
	}
}

func TestResolveReceiveTarget(t *testing.T) {
	tests := []struct {
		name        string
		chatID      string
		actorUserID string
		wantID      string
		wantType    string
	}{
		{name: "chat id wins", chatID: "oc_1", actorUserID: "ou_1", wantID: "oc_1", wantType: "chat_id"},
		{name: "open id fallback", actorUserID: "ou_1", wantID: "ou_1", wantType: "open_id"},
		{name: "union id fallback", actorUserID: "on_1", wantID: "on_1", wantType: "union_id"},
		{name: "user id fallback", actorUserID: "user_1", wantID: "user_1", wantType: "user_id"},
	}
	for _, tt := range tests {
		gotID, gotType := ResolveReceiveTarget(tt.chatID, tt.actorUserID)
		if gotID != tt.wantID || gotType != tt.wantType {
			t.Fatalf("%s: got (%q, %q), want (%q, %q)", tt.name, gotID, gotType, tt.wantID, tt.wantType)
		}
	}
}

func TestSurfaceIDForInboundUsesUserScopeForP2P(t *testing.T) {
	got := surfaceIDForInbound("app-1", "oc_xxx", "p2p", "user-1")
	if got != "feishu:app-1:user:user-1" {
		t.Fatalf("unexpected p2p surface id: %q", got)
	}
}

func TestSurfaceIDForInboundUsesChatScopeForGroup(t *testing.T) {
	got := surfaceIDForInbound("app-1", "oc_xxx", "group", "user-1")
	if got != "feishu:app-1:chat:oc_xxx" {
		t.Fatalf("unexpected group surface id: %q", got)
	}
}

func TestParseSurfaceRefSupportsLegacyAndGatewayAwareFormats(t *testing.T) {
	newRef, ok := ParseSurfaceRef("feishu:app-1:chat:oc_1")
	if !ok {
		t.Fatal("expected gateway-aware surface id to parse")
	}
	if newRef.GatewayID != "app-1" || newRef.ScopeKind != ScopeKindChat || newRef.ScopeID != "oc_1" {
		t.Fatalf("unexpected new surface ref: %#v", newRef)
	}

	legacyRef, ok := ParseSurfaceRef("feishu:user:user-1")
	if !ok {
		t.Fatal("expected legacy surface id to parse")
	}
	if legacyRef.GatewayID != LegacyDefaultGatewayID || legacyRef.ScopeKind != ScopeKindUser || legacyRef.ScopeID != "user-1" {
		t.Fatalf("unexpected legacy surface ref: %#v", legacyRef)
	}
}

func TestParseTextActionRecognizesModelAndReasoningCommands(t *testing.T) {
	tests := map[string]control.ActionKind{
		"/model":          control.ActionModelCommand,
		"/model gpt-5.4":  control.ActionModelCommand,
		"/reasoning high": control.ActionReasoningCommand,
		"/effort medium":  control.ActionReasoningCommand,
		"/access":         control.ActionAccessCommand,
		"/access full":    control.ActionAccessCommand,
		"/approval":       control.ActionAccessCommand,
	}
	for input, want := range tests {
		action, handled := parseTextAction(input)
		if !handled {
			t.Fatalf("expected %q to be handled", input)
		}
		if action.Kind != want {
			t.Fatalf("input %q => kind %q, want %q", input, action.Kind, want)
		}
		if action.Text != input {
			t.Fatalf("input %q => text %q, want raw command", input, action.Text)
		}
	}
}

func TestParseTextActionRecognizesSessionCommands(t *testing.T) {
	tests := map[string]control.ActionKind{
		"/threads":      control.ActionShowThreads,
		"/use":          control.ActionShowThreads,
		"/sessions":     control.ActionShowThreads,
		"/useall":       control.ActionShowAllThreads,
		"/sessionsall":  control.ActionShowAllThreads,
		"/newinstance":  control.ActionNewInstance,
		"/killinstance": control.ActionKillInstance,
	}
	for input, want := range tests {
		action, handled := parseTextAction(input)
		if !handled {
			t.Fatalf("expected %q to be handled", input)
		}
		if action.Kind != want {
			t.Fatalf("input %q => kind %q, want %q", input, action.Kind, want)
		}
	}
}

func TestParseMessageEventCommandPreservesGatewayID(t *testing.T) {
	gateway := NewLiveGateway(LiveGatewayConfig{GatewayID: "app-2"})
	event := &larkim.P2MessageReceiveV1{
		Event: &larkim.P2MessageReceiveV1Data{
			Sender: &larkim.EventSender{
				SenderId: &larkim.UserId{OpenId: stringRef("ou_user")},
			},
			Message: &larkim.EventMessage{
				MessageId:   stringRef("om-msg-1"),
				ChatId:      stringRef("oc_chat"),
				ChatType:    stringRef("group"),
				MessageType: stringRef("text"),
				Content:     stringRef(`{"text":" /list "}`),
			},
		},
	}

	action, ok, err := gateway.parseMessageEvent(t.Context(), event)
	if err != nil {
		t.Fatalf("parseMessageEvent returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected command message to be handled")
	}
	if action.Kind != control.ActionListInstances {
		t.Fatalf("unexpected action kind: %#v", action)
	}
	if action.GatewayID != "app-2" {
		t.Fatalf("expected gateway id to be preserved, got %#v", action)
	}
	if action.SurfaceSessionID != "feishu:app-2:chat:oc_chat" {
		t.Fatalf("unexpected surface routing: %#v", action)
	}
	if action.ChatID != "oc_chat" || action.ActorUserID != "ou_user" || action.MessageID != "om-msg-1" {
		t.Fatalf("unexpected command routing payload: %#v", action)
	}
}

func TestCardTemplateUsesSemanticColors(t *testing.T) {
	tests := map[string]string{
		cardThemeInfo:     "grey",
		cardThemeSuccess:  "green",
		cardThemeApproval: "green",
		cardThemeFinal:    "blue",
		cardThemeError:    "red",
		"relay-error":     "red",
		"thread-1":        "grey",
	}
	for input, want := range tests {
		if got := cardTemplate(input, ""); got != want {
			t.Fatalf("cardTemplate(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestParseCardActionTriggerEventBuildsPromptSelectionAction(t *testing.T) {
	gateway := NewLiveGateway(LiveGatewayConfig{GatewayID: "app-1"})
	gateway.recordSurfaceMessage("om-card-1", "feishu:app-1:user:user-1")
	userID := "user-1"
	event := &larkcallback.CardActionTriggerEvent{
		Event: &larkcallback.CardActionTriggerRequest{
			Operator: &larkcallback.Operator{UserID: &userID},
			Action: &larkcallback.CallBackAction{
				Value: map[string]interface{}{
					"kind":      "prompt_select",
					"prompt_id": "prompt-1",
					"option_id": "thread-1",
				},
			},
			Context: &larkcallback.Context{
				OpenChatID:    "oc_1",
				OpenMessageID: "om-card-1",
			},
		},
	}

	action, ok := gateway.parseCardActionTriggerEvent(event)
	if !ok {
		t.Fatal("expected card callback to be parsed")
	}
	if action.Kind != control.ActionSelectPrompt {
		t.Fatalf("unexpected action kind: %#v", action)
	}
	if action.GatewayID != "app-1" {
		t.Fatalf("unexpected gateway id: %#v", action)
	}
	if action.SurfaceSessionID != "feishu:app-1:user:user-1" || action.ChatID != "oc_1" || action.ActorUserID != "user-1" {
		t.Fatalf("unexpected action routing: %#v", action)
	}
	if action.PromptID != "prompt-1" || action.OptionID != "thread-1" {
		t.Fatalf("unexpected prompt selection payload: %#v", action)
	}
}

func TestParseCardActionTriggerEventBuildsDirectUseThreadAction(t *testing.T) {
	gateway := NewLiveGateway(LiveGatewayConfig{GatewayID: "app-1"})
	gateway.recordSurfaceMessage("om-card-3", "feishu:app-1:user:user-1")
	userID := "user-1"
	event := &larkcallback.CardActionTriggerEvent{
		Event: &larkcallback.CardActionTriggerRequest{
			Operator: &larkcallback.Operator{UserID: &userID},
			Action: &larkcallback.CallBackAction{
				Value: map[string]interface{}{
					"kind":      "use_thread",
					"thread_id": "thread-1",
				},
			},
			Context: &larkcallback.Context{
				OpenChatID:    "oc_1",
				OpenMessageID: "om-card-3",
			},
		},
	}

	action, ok := gateway.parseCardActionTriggerEvent(event)
	if !ok {
		t.Fatal("expected card callback to be parsed")
	}
	if action.Kind != control.ActionUseThread {
		t.Fatalf("unexpected action kind: %#v", action)
	}
	if action.ThreadID != "thread-1" {
		t.Fatalf("unexpected direct thread payload: %#v", action)
	}
	if action.SurfaceSessionID != "feishu:app-1:user:user-1" || action.ChatID != "oc_1" || action.ActorUserID != "user-1" {
		t.Fatalf("unexpected action routing: %#v", action)
	}
}

func TestParseCardActionTriggerEventBuildsDirectAttachInstanceAction(t *testing.T) {
	gateway := NewLiveGateway(LiveGatewayConfig{GatewayID: "app-1"})
	gateway.recordSurfaceMessage("om-card-4", "feishu:app-1:user:user-1")
	userID := "user-1"
	event := &larkcallback.CardActionTriggerEvent{
		Event: &larkcallback.CardActionTriggerRequest{
			Operator: &larkcallback.Operator{UserID: &userID},
			Action: &larkcallback.CallBackAction{
				Value: map[string]interface{}{
					"kind":        "attach_instance",
					"instance_id": "inst-1",
				},
			},
			Context: &larkcallback.Context{
				OpenChatID:    "oc_1",
				OpenMessageID: "om-card-4",
			},
		},
	}

	action, ok := gateway.parseCardActionTriggerEvent(event)
	if !ok {
		t.Fatal("expected card callback to be parsed")
	}
	if action.Kind != control.ActionAttachInstance || action.InstanceID != "inst-1" {
		t.Fatalf("unexpected attach action: %#v", action)
	}
	if action.SurfaceSessionID != "feishu:app-1:user:user-1" || action.ChatID != "oc_1" || action.ActorUserID != "user-1" {
		t.Fatalf("unexpected action routing: %#v", action)
	}
}

func TestParseCardActionTriggerEventBuildsResumeHeadlessAction(t *testing.T) {
	gateway := NewLiveGateway(LiveGatewayConfig{GatewayID: "app-1"})
	gateway.recordSurfaceMessage("om-card-5", "feishu:app-1:user:user-1")
	userID := "user-1"
	event := &larkcallback.CardActionTriggerEvent{
		Event: &larkcallback.CardActionTriggerRequest{
			Operator: &larkcallback.Operator{UserID: &userID},
			Action: &larkcallback.CallBackAction{
				Value: map[string]interface{}{
					"kind":      "resume_headless_thread",
					"thread_id": "thread-1",
				},
			},
			Context: &larkcallback.Context{
				OpenChatID:    "oc_1",
				OpenMessageID: "om-card-5",
			},
		},
	}

	action, ok := gateway.parseCardActionTriggerEvent(event)
	if !ok {
		t.Fatal("expected card callback to be parsed")
	}
	if action.Kind != control.ActionResumeHeadless || action.ThreadID != "thread-1" {
		t.Fatalf("unexpected headless resume action: %#v", action)
	}
	if action.SurfaceSessionID != "feishu:app-1:user:user-1" || action.ChatID != "oc_1" || action.ActorUserID != "user-1" {
		t.Fatalf("unexpected action routing: %#v", action)
	}
}

func TestParseCardActionTriggerEventBuildsKickActions(t *testing.T) {
	tests := []struct {
		name         string
		value        map[string]interface{}
		wantKind     control.ActionKind
		wantThreadID string
	}{
		{
			name: "confirm",
			value: map[string]interface{}{
				"kind":      "kick_thread_confirm",
				"thread_id": "thread-1",
			},
			wantKind:     control.ActionConfirmKickThread,
			wantThreadID: "thread-1",
		},
		{
			name: "cancel",
			value: map[string]interface{}{
				"kind":      "kick_thread_cancel",
				"thread_id": "thread-1",
			},
			wantKind:     control.ActionCancelKickThread,
			wantThreadID: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gateway := NewLiveGateway(LiveGatewayConfig{GatewayID: "app-1"})
			gateway.recordSurfaceMessage("om-card-6", "feishu:app-1:user:user-1")
			userID := "user-1"
			event := &larkcallback.CardActionTriggerEvent{
				Event: &larkcallback.CardActionTriggerRequest{
					Operator: &larkcallback.Operator{UserID: &userID},
					Action:   &larkcallback.CallBackAction{Value: tt.value},
					Context: &larkcallback.Context{
						OpenChatID:    "oc_1",
						OpenMessageID: "om-card-6",
					},
				},
			}

			action, ok := gateway.parseCardActionTriggerEvent(event)
			if !ok {
				t.Fatal("expected card callback to be parsed")
			}
			if action.Kind != tt.wantKind || action.ThreadID != tt.wantThreadID {
				t.Fatalf("unexpected kick action: %#v", action)
			}
			if action.SurfaceSessionID != "feishu:app-1:user:user-1" || action.ChatID != "oc_1" || action.ActorUserID != "user-1" {
				t.Fatalf("unexpected action routing: %#v", action)
			}
		})
	}
}

func TestParseCardActionTriggerEventBuildsRequestRespondAction(t *testing.T) {
	gateway := NewLiveGateway(LiveGatewayConfig{GatewayID: "app-1"})
	gateway.recordSurfaceMessage("om-card-2", "feishu:app-1:user:user-1")
	userID := "user-1"
	event := &larkcallback.CardActionTriggerEvent{
		Event: &larkcallback.CardActionTriggerRequest{
			Operator: &larkcallback.Operator{UserID: &userID},
			Action: &larkcallback.CallBackAction{
				Value: map[string]interface{}{
					"kind":              "request_respond",
					"request_id":        "req-1",
					"request_type":      "approval",
					"request_option_id": "acceptForSession",
				},
			},
			Context: &larkcallback.Context{
				OpenChatID:    "oc_1",
				OpenMessageID: "om-card-2",
			},
		},
	}

	action, ok := gateway.parseCardActionTriggerEvent(event)
	if !ok {
		t.Fatal("expected card callback to be parsed")
	}
	if action.Kind != control.ActionRespondRequest {
		t.Fatalf("unexpected action kind: %#v", action)
	}
	if action.RequestID != "req-1" || action.RequestType != "approval" || action.RequestOptionID != "acceptForSession" {
		t.Fatalf("unexpected request respond payload: %#v", action)
	}
}

func TestParseCardActionTriggerEventFallsBackToApprovedBool(t *testing.T) {
	gateway := NewLiveGateway(LiveGatewayConfig{GatewayID: "app-1"})
	gateway.recordSurfaceMessage("om-card-3", "feishu:app-1:user:user-1")
	userID := "user-1"
	event := &larkcallback.CardActionTriggerEvent{
		Event: &larkcallback.CardActionTriggerRequest{
			Operator: &larkcallback.Operator{UserID: &userID},
			Action: &larkcallback.CallBackAction{
				Value: map[string]interface{}{
					"kind":         "request_respond",
					"request_id":   "req-legacy",
					"request_type": "approval",
					"approved":     false,
				},
			},
			Context: &larkcallback.Context{
				OpenChatID:    "oc_1",
				OpenMessageID: "om-card-3",
			},
		},
	}

	action, ok := gateway.parseCardActionTriggerEvent(event)
	if !ok {
		t.Fatal("expected legacy card callback to be parsed")
	}
	if action.RequestOptionID != "decline" || action.Approved {
		t.Fatalf("unexpected legacy request respond payload: %#v", action)
	}
}

func TestParseMessageRecalledEventBuildsRecallAction(t *testing.T) {
	gateway := NewLiveGateway(LiveGatewayConfig{GatewayID: "app-1"})
	gateway.recordSurfaceMessage("om-msg-1", "feishu:app-1:user:user-1")
	event := &larkim.P2MessageRecalledV1{
		Event: &larkim.P2MessageRecalledV1Data{
			MessageId: stringRef("om-msg-1"),
			ChatId:    stringRef("oc_1"),
		},
	}

	action, ok := gateway.parseMessageRecalledEvent(event)
	if !ok {
		t.Fatal("expected recalled event to be parsed")
	}
	if action.Kind != control.ActionMessageRecalled {
		t.Fatalf("unexpected action kind: %#v", action)
	}
	if action.GatewayID != "app-1" {
		t.Fatalf("unexpected gateway id: %#v", action)
	}
	if action.SurfaceSessionID != "feishu:app-1:user:user-1" || action.TargetMessageID != "om-msg-1" || action.ChatID != "oc_1" {
		t.Fatalf("unexpected recalled action payload: %#v", action)
	}
}

func TestParseMessageRecalledEventIgnoresUnknownMessage(t *testing.T) {
	gateway := NewLiveGateway(LiveGatewayConfig{GatewayID: "app-1"})
	event := &larkim.P2MessageRecalledV1{
		Event: &larkim.P2MessageRecalledV1Data{
			MessageId: stringRef("om-missing"),
		},
	}

	if action, ok := gateway.parseMessageRecalledEvent(event); ok || action.Kind != "" {
		t.Fatalf("expected unknown recalled message to be ignored, got %#v", action)
	}
}

func stringRef(value string) *string {
	return &value
}

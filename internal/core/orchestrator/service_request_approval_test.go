package orchestrator

import (
	"testing"
	"time"

	"github.com/kxn/codex-remote-feishu/internal/core/agentproto"
	"github.com/kxn/codex-remote-feishu/internal/core/control"
	"github.com/kxn/codex-remote-feishu/internal/core/state"
)

func TestApprovalCommandRequestPromptAddsCancelOption(t *testing.T) {
	now := time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC)
	svc := newServiceForTest(&now)
	svc.UpsertInstance(&state.InstanceRecord{
		InstanceID:              "inst-1",
		DisplayName:             "droid",
		WorkspaceRoot:           "/data/dl/droid",
		WorkspaceKey:            "/data/dl/droid",
		ShortName:               "droid",
		Online:                  true,
		ObservedFocusedThreadID: "thread-1",
		Threads: map[string]*state.ThreadRecord{
			"thread-1": {ThreadID: "thread-1", Name: "修复登录流程", CWD: "/data/dl/droid", Loaded: true},
		},
	})
	svc.ApplySurfaceAction(control.Action{Kind: control.ActionAttachInstance, SurfaceSessionID: "surface-1", ChatID: "chat-1", ActorUserID: "user-1", InstanceID: "inst-1"})
	svc.ApplyAgentEvent("inst-1", agentproto.Event{
		Kind:      agentproto.EventTurnStarted,
		ThreadID:  "thread-1",
		TurnID:    "turn-1",
		Initiator: agentproto.Initiator{Kind: agentproto.InitiatorLocalUI},
	})

	events := svc.ApplyAgentEvent("inst-1", agentproto.Event{
		Kind:      agentproto.EventRequestStarted,
		ThreadID:  "thread-1",
		TurnID:    "turn-1",
		RequestID: "req-cmd-1",
		Metadata: map[string]any{
			"requestType": "approval",
			"requestKind": "approval_command",
			"title":       "需要确认执行命令",
			"body":        "本地 Codex 想执行 `npm install`。",
		},
	})
	if len(events) != 1 || events[0].FeishuDirectRequestPrompt == nil {
		t.Fatalf("expected one request prompt event, got %#v", events)
	}
	prompt := events[0].FeishuDirectRequestPrompt
	if len(prompt.Options) != 5 {
		t.Fatalf("expected command approval prompt to expose cancel + feedback, got %#v", prompt.Options)
	}
	if prompt.Options[1].OptionID != "acceptForSession" || prompt.Options[3].OptionID != "cancel" || prompt.Options[4].OptionID != "captureFeedback" {
		t.Fatalf("unexpected command approval options: %#v", prompt.Options)
	}
}

func TestRespondRequestCancelDispatchesDecision(t *testing.T) {
	now := time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC)
	svc := newServiceForTest(&now)
	svc.UpsertInstance(&state.InstanceRecord{
		InstanceID:              "inst-1",
		DisplayName:             "droid",
		WorkspaceRoot:           "/data/dl/droid",
		WorkspaceKey:            "/data/dl/droid",
		ShortName:               "droid",
		Online:                  true,
		ObservedFocusedThreadID: "thread-1",
		Threads: map[string]*state.ThreadRecord{
			"thread-1": {ThreadID: "thread-1", Name: "修复登录流程", CWD: "/data/dl/droid", Loaded: true},
		},
	})
	svc.ApplySurfaceAction(control.Action{Kind: control.ActionAttachInstance, SurfaceSessionID: "surface-1", ChatID: "chat-1", ActorUserID: "user-1", InstanceID: "inst-1"})
	svc.ApplyAgentEvent("inst-1", agentproto.Event{
		Kind:      agentproto.EventTurnStarted,
		ThreadID:  "thread-1",
		TurnID:    "turn-1",
		Initiator: agentproto.Initiator{Kind: agentproto.InitiatorLocalUI},
	})
	svc.ApplyAgentEvent("inst-1", agentproto.Event{
		Kind:      agentproto.EventRequestStarted,
		ThreadID:  "thread-1",
		TurnID:    "turn-1",
		RequestID: "req-cmd-1",
		Metadata: map[string]any{
			"requestType": "approval",
			"requestKind": "approval_command",
			"options": []map[string]any{
				{"id": "accept", "label": "允许一次"},
				{"id": "cancel", "label": "取消"},
			},
		},
	})

	events := svc.ApplySurfaceAction(control.Action{
		Kind:             control.ActionRespondRequest,
		SurfaceSessionID: "surface-1",
		MessageID:        "om-card-1",
		RequestID:        "req-cmd-1",
		RequestOptionID:  "cancel",
	})
	if len(events) != 1 || events[0].Command == nil {
		t.Fatalf("expected one agent command event, got %#v", events)
	}
	if events[0].Command.Request.Response["decision"] != "cancel" {
		t.Fatalf("unexpected request cancel payload: %#v", events[0].Command.Request.Response)
	}
}

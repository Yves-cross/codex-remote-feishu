package daemon

import (
	"context"
	"testing"
	"time"

	"github.com/kxn/codex-remote-feishu/internal/adapter/feishu"
	"github.com/kxn/codex-remote-feishu/internal/core/agentproto"
	"github.com/kxn/codex-remote-feishu/internal/core/control"
	"github.com/kxn/codex-remote-feishu/internal/core/state"
)

func TestHandleGatewayActionReplacesMenuCardForListHandoffInNormalMode(t *testing.T) {
	gateway := &recordingGateway{}
	app := New(":0", ":0", gateway, agentproto.ServerIdentity{
		PID:       42,
		StartedAt: time.Date(2026, 4, 10, 10, 0, 0, 0, time.UTC),
	})
	app.service.MaterializeSurface("surface-1", "app-1", "chat-1", "user-1")
	app.service.UpsertInstance(&state.InstanceRecord{
		InstanceID:    "inst-1",
		DisplayName:   "proj1",
		WorkspaceRoot: "/data/dl/proj1",
		WorkspaceKey:  "/data/dl/proj1",
		ShortName:     "proj1",
		Online:        true,
		Threads: map[string]*state.ThreadRecord{
			"thread-1": {ThreadID: "thread-1", Name: "会话1", CWD: "/data/dl/proj1", LastUsedAt: time.Date(2026, 4, 10, 10, 2, 0, 0, time.UTC)},
		},
	})

	result := app.HandleGatewayAction(context.Background(), control.Action{
		Kind:             control.ActionListInstances,
		GatewayID:        "app-1",
		SurfaceSessionID: "surface-1",
		ChatID:           "chat-1",
		ActorUserID:      "user-1",
		Inbound: &control.ActionInboundMeta{
			CardDaemonLifecycleID: app.daemonLifecycleID,
		},
	})

	if result == nil || result.ReplaceCurrentCard == nil {
		t.Fatalf("expected inline replacement result, got %#v", result)
	}
	if len(gateway.operations) != 0 {
		t.Fatalf("expected no appended gateway operations, got %#v", gateway.operations)
	}
	if result.ReplaceCurrentCard.CardTitle != "选择工作区与会话" {
		t.Fatalf("unexpected replacement card title: %#v", result.ReplaceCurrentCard)
	}
}

func TestHandleGatewayActionReplacesMenuCardForListHandoffInVSCodeMode(t *testing.T) {
	gateway := &recordingGateway{}
	app := New(":0", ":0", gateway, agentproto.ServerIdentity{
		PID:       42,
		StartedAt: time.Date(2026, 4, 10, 10, 0, 0, 0, time.UTC),
	})
	app.service.MaterializeSurface("surface-1", "app-1", "chat-1", "user-1")
	app.service.ApplySurfaceAction(control.Action{
		Kind:             control.ActionModeCommand,
		SurfaceSessionID: "surface-1",
		ChatID:           "chat-1",
		ActorUserID:      "user-1",
		Text:             "/mode vscode",
	})
	app.service.UpsertInstance(&state.InstanceRecord{
		InstanceID:    "inst-vscode-1",
		DisplayName:   "droid",
		WorkspaceRoot: "/data/dl/droid",
		WorkspaceKey:  "/data/dl/droid",
		ShortName:     "droid",
		Source:        "vscode",
		Online:        true,
	})

	result := app.HandleGatewayAction(context.Background(), control.Action{
		Kind:             control.ActionListInstances,
		GatewayID:        "app-1",
		SurfaceSessionID: "surface-1",
		ChatID:           "chat-1",
		ActorUserID:      "user-1",
		Inbound: &control.ActionInboundMeta{
			CardDaemonLifecycleID: app.daemonLifecycleID,
		},
	})

	if result == nil || result.ReplaceCurrentCard == nil {
		t.Fatalf("expected inline replacement result, got %#v", result)
	}
	if len(gateway.operations) != 0 {
		t.Fatalf("expected no appended gateway operations, got %#v", gateway.operations)
	}
	if result.ReplaceCurrentCard.CardTitle != "在线 VS Code 实例" {
		t.Fatalf("unexpected replacement card title: %#v", result.ReplaceCurrentCard)
	}
}

func TestHandleGatewayActionReplacesMenuCardForSendFileHandoff(t *testing.T) {
	gateway := &recordingGateway{}
	app := New(":0", ":0", gateway, agentproto.ServerIdentity{
		PID:       42,
		StartedAt: time.Date(2026, 4, 10, 10, 0, 0, 0, time.UTC),
	})
	workspaceRoot := t.TempDir()
	app.service.MaterializeSurface("surface-1", "app-1", "chat-1", "user-1")
	app.service.UpsertInstance(&state.InstanceRecord{
		InstanceID:    "inst-1",
		DisplayName:   "headless",
		WorkspaceRoot: workspaceRoot,
		WorkspaceKey:  workspaceRoot,
		Source:        "headless",
		Managed:       true,
		Online:        true,
		Threads:       map[string]*state.ThreadRecord{},
	})
	app.service.ApplySurfaceAction(control.Action{
		Kind:             control.ActionAttachInstance,
		SurfaceSessionID: "surface-1",
		ChatID:           "chat-1",
		ActorUserID:      "user-1",
		InstanceID:       "inst-1",
	})

	result := app.HandleGatewayAction(context.Background(), control.Action{
		Kind:             control.ActionSendFile,
		GatewayID:        "app-1",
		SurfaceSessionID: "surface-1",
		ChatID:           "chat-1",
		ActorUserID:      "user-1",
		Inbound: &control.ActionInboundMeta{
			CardDaemonLifecycleID: app.daemonLifecycleID,
		},
	})

	if result == nil || result.ReplaceCurrentCard == nil {
		t.Fatalf("expected inline replacement result, got %#v", result)
	}
	if len(gateway.operations) != 0 {
		t.Fatalf("expected no appended gateway operations, got %#v", gateway.operations)
	}
	if result.ReplaceCurrentCard.CardTitle != "选择要发送的文件" {
		t.Fatalf("unexpected replacement card title: %#v", result.ReplaceCurrentCard)
	}
}

func TestHandleGatewayActionUpdatesMenuCardForCompactOwnerFlow(t *testing.T) {
	gateway := newLifecycleGateway()
	app := New(":0", ":0", gateway, agentproto.ServerIdentity{
		PID:       42,
		StartedAt: time.Date(2026, 4, 19, 9, 0, 0, 0, time.UTC),
	})
	app.sendAgentCommand = func(_ string, command agentproto.Command) error {
		if command.Kind != agentproto.CommandThreadCompactStart {
			t.Fatalf("unexpected command dispatched during compact menu handoff: %#v", command)
		}
		return nil
	}
	app.service.MaterializeSurface("surface-1", "app-1", "chat-1", "user-1")
	app.service.UpsertInstance(&state.InstanceRecord{
		InstanceID:    "inst-1",
		DisplayName:   "proj1",
		WorkspaceRoot: "/data/dl/proj1",
		WorkspaceKey:  "/data/dl/proj1",
		ShortName:     "proj1",
		Online:        true,
		Threads: map[string]*state.ThreadRecord{
			"thread-1": {ThreadID: "thread-1", Name: "会话1", CWD: "/data/dl/proj1", Loaded: true},
		},
	})
	app.service.ApplySurfaceAction(control.Action{
		Kind:             control.ActionAttachInstance,
		SurfaceSessionID: "surface-1",
		ChatID:           "chat-1",
		ActorUserID:      "user-1",
		InstanceID:       "inst-1",
	})
	app.service.ApplySurfaceAction(control.Action{
		Kind:             control.ActionUseThread,
		SurfaceSessionID: "surface-1",
		ThreadID:         "thread-1",
	})

	result := app.HandleGatewayAction(context.Background(), control.Action{
		Kind:             control.ActionCompact,
		GatewayID:        "app-1",
		SurfaceSessionID: "surface-1",
		ChatID:           "chat-1",
		ActorUserID:      "user-1",
		MessageID:        "om-menu-compact-1",
		Inbound: &control.ActionInboundMeta{
			CardDaemonLifecycleID: app.daemonLifecycleID,
		},
	})

	if result != nil {
		t.Fatalf("expected compact menu handoff to stream through gateway updates, got %#v", result)
	}
	ops := gateway.snapshotOperations()
	if len(ops) != 1 {
		t.Fatalf("expected one in-place compact card update, got %#v", ops)
	}
	if ops[0].Kind != feishu.OperationUpdateCard || ops[0].MessageID != "om-menu-compact-1" || ops[0].CardTitle != "正在压缩上下文" {
		t.Fatalf("unexpected compact owner-card update: %#v", ops[0])
	}
}

func TestHandleGatewayActionReplacesMenuCardWhenSendFileUnavailable(t *testing.T) {
	gateway := &recordingGateway{}
	app := New(":0", ":0", gateway, agentproto.ServerIdentity{
		PID:       42,
		StartedAt: time.Date(2026, 4, 19, 9, 5, 0, 0, time.UTC),
	})
	app.service.MaterializeSurface("surface-1", "app-1", "chat-1", "user-1")
	app.service.ApplySurfaceAction(control.Action{
		Kind:             control.ActionModeCommand,
		SurfaceSessionID: "surface-1",
		ChatID:           "chat-1",
		ActorUserID:      "user-1",
		Text:             "/mode vscode",
	})

	result := app.HandleGatewayAction(context.Background(), control.Action{
		Kind:             control.ActionSendFile,
		GatewayID:        "app-1",
		SurfaceSessionID: "surface-1",
		ChatID:           "chat-1",
		ActorUserID:      "user-1",
		MessageID:        "om-menu-sendfile-1",
		Inbound: &control.ActionInboundMeta{
			CardDaemonLifecycleID: app.daemonLifecycleID,
		},
	})

	if result == nil || result.ReplaceCurrentCard == nil {
		t.Fatalf("expected inline replacement result, got %#v", result)
	}
	if result.ReplaceCurrentCard.CardTitle != "当前不能发送文件" {
		t.Fatalf("unexpected unavailable replacement title: %#v", result.ReplaceCurrentCard)
	}
	if len(gateway.operations) != 0 {
		t.Fatalf("expected no appended gateway operations, got %#v", gateway.operations)
	}
}

package daemon

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/kxn/codex-remote-feishu/internal/core/agentproto"
	"github.com/kxn/codex-remote-feishu/internal/core/control"
	"github.com/kxn/codex-remote-feishu/internal/core/state"
	relayruntime "github.com/kxn/codex-remote-feishu/internal/runtime"
)

func TestDaemonPersistsHeadlessRestoreHintAcrossRestart(t *testing.T) {
	t.Parallel()

	stateDir := t.TempDir()
	app := newRestoreHintTestApp(stateDir)
	seedHeadlessInstance(app, "inst-headless-1", "thread-1")

	app.HandleAction(context.Background(), control.Action{
		Kind:             control.ActionAttachInstance,
		GatewayID:        "app-1",
		SurfaceSessionID: "surface-1",
		ChatID:           "chat-1",
		ActorUserID:      "user-1",
		InstanceID:       "inst-headless-1",
	})

	hint := app.HeadlessRestoreHint("surface-1")
	if hint == nil {
		t.Fatal("expected restore hint after headless attach")
	}
	if hint.GatewayID != "app-1" || hint.ChatID != "chat-1" || hint.ActorUserID != "user-1" {
		t.Fatalf("unexpected restore hint routing: %#v", hint)
	}
	if hint.ThreadID != "thread-1" || hint.ThreadTitle == "" || hint.ThreadCWD != "/data/dl/droid" {
		t.Fatalf("unexpected restore hint payload: %#v", hint)
	}

	restarted := newRestoreHintTestApp(stateDir)
	reloaded := restarted.HeadlessRestoreHint("surface-1")
	if reloaded == nil {
		t.Fatal("expected restore hint after restart")
	}
	if !sameHeadlessRestoreHintContent(*hint, *reloaded) {
		t.Fatalf("unexpected restore hint after restart: want=%#v got=%#v", hint, reloaded)
	}
}

func TestDaemonDerivesHeadlessRestoreHintFromSurfaceResumeState(t *testing.T) {
	t.Parallel()

	stateDir := t.TempDir()
	putSurfaceResumeStateForTest(t, stateDir, SurfaceResumeEntry{
		SurfaceSessionID:   "surface-1",
		GatewayID:          "app-1",
		ChatID:             "chat-1",
		ActorUserID:        "user-1",
		ProductMode:        "normal",
		ResumeThreadID:     "thread-1",
		ResumeThreadTitle:  "修复登录流程",
		ResumeThreadCWD:    "/data/dl/droid",
		ResumeWorkspaceKey: "/data/dl/droid",
		ResumeRouteMode:    "pinned",
		ResumeHeadless:     true,
	})

	app := newRestoreHintTestApp(stateDir)
	hint := app.HeadlessRestoreHint("surface-1")
	if hint == nil {
		t.Fatal("expected derived headless restore hint from surface resume state")
	}
	if hint.ThreadID != "thread-1" || hint.ThreadTitle != "修复登录流程" || hint.ThreadCWD != "/data/dl/droid" {
		t.Fatalf("unexpected derived restore hint: %#v", hint)
	}
	if len(app.surfaceResumeRuntime.headlessRestore) != 1 {
		t.Fatalf("expected in-memory headless restore state derived from surface resume state, got %#v", app.surfaceResumeRuntime.headlessRestore)
	}
}

func TestDaemonClearsHeadlessRestoreHintOnDetach(t *testing.T) {
	t.Parallel()

	stateDir := t.TempDir()
	app := newRestoreHintTestApp(stateDir)
	seedHeadlessInstance(app, "inst-headless-1", "thread-1")

	app.HandleAction(context.Background(), control.Action{
		Kind:             control.ActionAttachInstance,
		GatewayID:        "app-1",
		SurfaceSessionID: "surface-1",
		ChatID:           "chat-1",
		ActorUserID:      "user-1",
		InstanceID:       "inst-headless-1",
	})
	app.HandleAction(context.Background(), control.Action{
		Kind:             control.ActionDetach,
		SurfaceSessionID: "surface-1",
	})

	if hint := app.HeadlessRestoreHint("surface-1"); hint != nil {
		t.Fatalf("expected restore hint to clear on detach, got %#v", hint)
	}
	restarted := newRestoreHintTestApp(stateDir)
	if hint := restarted.HeadlessRestoreHint("surface-1"); hint != nil {
		t.Fatalf("expected restore hint to stay cleared after restart, got %#v", hint)
	}
}

func TestDaemonClearsHeadlessRestoreHintWhenSwitchingToVSCode(t *testing.T) {
	t.Parallel()

	stateDir := t.TempDir()
	app := newRestoreHintTestApp(stateDir)
	seedHeadlessInstance(app, "inst-headless-1", "thread-1")
	app.service.UpsertInstance(&state.InstanceRecord{
		InstanceID:    "inst-vscode-1",
		DisplayName:   "droid",
		WorkspaceRoot: "/data/dl/droid",
		WorkspaceKey:  "/data/dl/droid",
		ShortName:     "droid",
		Source:        "vscode",
		Online:        true,
		Threads: map[string]*state.ThreadRecord{
			"thread-2": {ThreadID: "thread-2", Name: "VS Code 会话", CWD: "/data/dl/droid", Loaded: true},
		},
	})

	app.HandleAction(context.Background(), control.Action{
		Kind:             control.ActionAttachInstance,
		GatewayID:        "app-1",
		SurfaceSessionID: "surface-1",
		ChatID:           "chat-1",
		ActorUserID:      "user-1",
		InstanceID:       "inst-headless-1",
	})
	if hint := app.HeadlessRestoreHint("surface-1"); hint == nil {
		t.Fatal("expected initial restore hint after headless attach")
	}

	app.HandleAction(context.Background(), control.Action{
		Kind:             control.ActionUseThread,
		SurfaceSessionID: "surface-1",
		ThreadID:         "thread-2",
	})

	if hint := app.HeadlessRestoreHint("surface-1"); hint != nil {
		t.Fatalf("expected restore hint to clear after switching to vscode, got %#v", hint)
	}
}

func TestDaemonModeSwitchToVSCodeClearsHeadlessRestoreState(t *testing.T) {
	t.Parallel()

	stateDir := t.TempDir()
	app := newRestoreHintTestApp(stateDir)
	seedHeadlessInstance(app, "inst-headless-1", "thread-1")

	app.HandleAction(context.Background(), control.Action{
		Kind:             control.ActionAttachInstance,
		GatewayID:        "app-1",
		SurfaceSessionID: "surface-1",
		ChatID:           "chat-1",
		ActorUserID:      "user-1",
		InstanceID:       "inst-headless-1",
	})
	if hint := app.HeadlessRestoreHint("surface-1"); hint == nil {
		t.Fatal("expected restore hint after headless attach")
	}
	if len(app.surfaceResumeRuntime.headlessRestore) != 1 {
		t.Fatalf("expected one in-memory restore entry before mode switch, got %#v", app.surfaceResumeRuntime.headlessRestore)
	}

	app.HandleAction(context.Background(), control.Action{
		Kind:             control.ActionModeCommand,
		GatewayID:        "app-1",
		SurfaceSessionID: "surface-1",
		ChatID:           "chat-1",
		ActorUserID:      "user-1",
		Text:             "/mode vscode",
	})

	if hint := app.HeadlessRestoreHint("surface-1"); hint != nil {
		t.Fatalf("expected restore hint to clear after /mode vscode, got %#v", hint)
	}
	if len(app.surfaceResumeRuntime.headlessRestore) != 0 {
		t.Fatalf("expected in-memory restore state to clear after /mode vscode, got %#v", app.surfaceResumeRuntime.headlessRestore)
	}
	snapshot := app.service.SurfaceSnapshot("surface-1")
	if snapshot == nil || snapshot.ProductMode != "vscode" || snapshot.Attachment.InstanceID != "" || snapshot.PendingHeadless.InstanceID != "" {
		t.Fatalf("expected detached vscode snapshot after mode switch, got %#v", snapshot)
	}

	restarted := newRestoreHintTestApp(stateDir)
	if hint := restarted.HeadlessRestoreHint("surface-1"); hint != nil {
		t.Fatalf("expected restore hint to stay cleared after restart, got %#v", hint)
	}
}

func TestDaemonModeSwitchToVSCodeStaysDetachedAfterNormalAutoRestore(t *testing.T) {
	t.Parallel()

	stateDir := t.TempDir()
	putRestoreHintForTest(t, stateDir, HeadlessRestoreHint{
		SurfaceSessionID: "surface-1",
		GatewayID:        "app-1",
		ChatID:           "chat-1",
		ActorUserID:      "user-1",
		ThreadID:         "thread-1",
		ThreadTitle:      "修复登录流程",
		ThreadCWD:        "/data/dl/droid",
	})

	app := newRestoreHintTestApp(stateDir)
	app.startHeadless = func(opts relayruntime.HeadlessLaunchOptions) (int, error) {
		return 4321, nil
	}

	base := time.Date(2026, 4, 9, 14, 0, 0, 0, time.UTC)
	app.onTick(context.Background(), base)
	pending := app.service.SurfaceSnapshot("surface-1").PendingHeadless
	if pending.InstanceID == "" {
		t.Fatalf("expected pending headless after auto-restore tick, got %#v", pending)
	}

	app.onHello(context.Background(), agentproto.Hello{
		Instance: agentproto.InstanceHello{
			InstanceID:    pending.InstanceID,
			DisplayName:   "headless",
			WorkspaceRoot: "/data/dl/droid",
			WorkspaceKey:  "/data/dl/droid",
			ShortName:     "headless",
			Source:        "headless",
			Managed:       true,
			PID:           4321,
		},
	})

	snapshot := app.service.SurfaceSnapshot("surface-1")
	if snapshot == nil || snapshot.ProductMode != "normal" || snapshot.Attachment.InstanceID != pending.InstanceID || snapshot.Attachment.SelectedThreadID != "thread-1" {
		t.Fatalf("expected normal auto-restore to attach headless thread, got %#v", snapshot)
	}

	app.HandleAction(context.Background(), control.Action{
		Kind:             control.ActionModeCommand,
		GatewayID:        "app-1",
		SurfaceSessionID: "surface-1",
		ChatID:           "chat-1",
		ActorUserID:      "user-1",
		Text:             "/mode vscode",
	})
	app.onTick(context.Background(), base.Add(time.Second))

	snapshot = app.service.SurfaceSnapshot("surface-1")
	if snapshot == nil || snapshot.ProductMode != "vscode" || snapshot.Attachment.InstanceID != "" || snapshot.PendingHeadless.InstanceID != "" {
		t.Fatalf("expected vscode mode switch to stay detached after prior auto-restore, got %#v", snapshot)
	}
	if hint := app.HeadlessRestoreHint("surface-1"); hint != nil {
		t.Fatalf("expected restore hint to clear after /mode vscode from auto-restored state, got %#v", hint)
	}
	if len(app.surfaceResumeRuntime.headlessRestore) != 0 {
		t.Fatalf("expected in-memory restore state to clear after /mode vscode from auto-restored state, got %#v", app.surfaceResumeRuntime.headlessRestore)
	}
}

func TestDaemonKeepsHeadlessRestoreHintOnDisconnect(t *testing.T) {
	t.Parallel()

	stateDir := t.TempDir()
	app := newRestoreHintTestApp(stateDir)
	seedHeadlessInstance(app, "inst-headless-1", "thread-1")

	app.HandleAction(context.Background(), control.Action{
		Kind:             control.ActionAttachInstance,
		GatewayID:        "app-1",
		SurfaceSessionID: "surface-1",
		ChatID:           "chat-1",
		ActorUserID:      "user-1",
		InstanceID:       "inst-headless-1",
	})
	before := app.HeadlessRestoreHint("surface-1")
	if before == nil {
		t.Fatal("expected restore hint before disconnect")
	}

	app.onDisconnect(context.Background(), "inst-headless-1")

	after := app.HeadlessRestoreHint("surface-1")
	if after == nil {
		t.Fatal("expected restore hint to survive disconnect")
	}
	if !sameHeadlessRestoreHintContent(*before, *after) {
		t.Fatalf("unexpected restore hint after disconnect: want=%#v got=%#v", before, after)
	}
}

func TestDaemonMaterializesLatentSurfaceFromRestoreHintOnRestart(t *testing.T) {
	t.Parallel()

	stateDir := t.TempDir()
	app := newRestoreHintTestApp(stateDir)
	seedHeadlessInstance(app, "inst-headless-1", "thread-1")

	app.HandleAction(context.Background(), control.Action{
		Kind:             control.ActionAttachInstance,
		GatewayID:        "app-1",
		SurfaceSessionID: "surface-1",
		ChatID:           "chat-1",
		ActorUserID:      "user-1",
		InstanceID:       "inst-headless-1",
	})

	restarted := newRestoreHintTestApp(stateDir)
	snapshot := restarted.service.SurfaceSnapshot("surface-1")
	if snapshot == nil {
		t.Fatal("expected latent surface to materialize from restore hint")
	}
	if snapshot.Attachment.InstanceID != "" || snapshot.PendingHeadless.InstanceID != "" {
		t.Fatalf("expected materialized restore surface to stay detached, got %#v", snapshot)
	}
	if restarted.service.SurfaceGatewayID("surface-1") != "app-1" || restarted.service.SurfaceChatID("surface-1") != "chat-1" || restarted.service.SurfaceActorUserID("surface-1") != "user-1" {
		t.Fatalf("unexpected materialized surface routing: gateway=%q chat=%q actor=%q", restarted.service.SurfaceGatewayID("surface-1"), restarted.service.SurfaceChatID("surface-1"), restarted.service.SurfaceActorUserID("surface-1"))
	}
	if len(restarted.surfaceResumeRuntime.headlessRestore) != 1 {
		t.Fatalf("expected one recovery state entry after restart, got %#v", restarted.surfaceResumeRuntime.headlessRestore)
	}
}

func TestDaemonAutoRestoreReconnectsWithRecoveryNoticeOnly(t *testing.T) {
	t.Parallel()

	stateDir := t.TempDir()
	gateway := &recordingGateway{}
	app := New(":0", ":0", gateway, agentproto.ServerIdentity{})
	app.SetHeadlessRuntime(HeadlessRuntimeConfig{
		IdleTTL:    time.Hour,
		KillGrace:  time.Second,
		Paths:      relayruntime.Paths{StateDir: stateDir},
		BinaryPath: "codex",
	})
	app.sendAgentCommand = func(string, agentproto.Command) error { return nil }
	seedHeadlessInstance(app, "inst-headless-1", "thread-1")
	app.service.Instance("inst-headless-1").Threads["thread-1"].UndeliveredReplay = &state.ThreadReplayRecord{
		Kind:           state.ThreadReplayNotice,
		NoticeCode:     "problem_saved",
		NoticeTitle:    "问题提示",
		NoticeText:     "等待 headless 接手的 notice",
		NoticeThemeKey: "warning",
	}

	app.HandleAction(context.Background(), control.Action{
		Kind:             control.ActionAttachInstance,
		GatewayID:        "app-1",
		SurfaceSessionID: "surface-1",
		ChatID:           "chat-1",
		ActorUserID:      "user-1",
		InstanceID:       "inst-headless-1",
	})
	gateway.operations = nil
	if hint := app.HeadlessRestoreHint("surface-1"); hint == nil || hint.ThreadCWD != "/data/dl/droid" {
		t.Fatalf("expected persisted restore hint with cwd before disconnect, got %#v", hint)
	}
	if len(app.surfaceResumeRuntime.headlessRestore) != 1 {
		t.Fatalf("expected one in-memory restore state before disconnect, got %#v", app.surfaceResumeRuntime.headlessRestore)
	}

	app.onDisconnect(context.Background(), "inst-headless-1")
	gateway.operations = nil
	snapshot := app.service.SurfaceSnapshot("surface-1")
	if snapshot == nil || snapshot.Attachment.InstanceID != "" {
		t.Fatalf("expected surface to detach after disconnect while keeping restore state, got %#v", snapshot)
	}

	app.startHeadless = func(opts relayruntime.HeadlessLaunchOptions) (int, error) {
		return 4321, nil
	}
	base := time.Date(2026, 4, 8, 3, 20, 0, 0, time.UTC)
	app.onTick(context.Background(), base)
	if len(gateway.operations) != 0 {
		t.Fatalf("expected auto-restore headless start to stay silent, got %#v", gateway.operations)
	}
	pending := app.service.SurfaceSnapshot("surface-1").PendingHeadless
	if pending.InstanceID == "" {
		t.Fatalf("expected auto-restore pending headless after tick, got %#v", pending)
	}

	app.onHello(context.Background(), agentproto.Hello{
		Instance: agentproto.InstanceHello{
			InstanceID:    pending.InstanceID,
			DisplayName:   "headless",
			WorkspaceRoot: "/data/dl/droid",
			WorkspaceKey:  "/data/dl/droid",
			ShortName:     "headless",
			Source:        "headless",
			Managed:       true,
			PID:           4321,
		},
	})

	snapshot = app.service.SurfaceSnapshot("surface-1")
	if snapshot == nil || snapshot.Attachment.InstanceID != pending.InstanceID || snapshot.Attachment.SelectedThreadID != "thread-1" || snapshot.PendingHeadless.InstanceID != "" {
		t.Fatalf("expected recovery hello to attach restored thread, got %#v", snapshot)
	}
	if len(gateway.operations) != 1 {
		t.Fatalf("expected exactly one recovery notice, got %#v", gateway.operations)
	}
	if !strings.Contains(gateway.operations[0].CardTitle, "恢复") || !strings.Contains(gateway.operations[0].CardBody, "重连成功，已恢复到之前会话") {
		t.Fatalf("expected recovery success notice, got %#v", gateway.operations[0])
	}
	if strings.Contains(gateway.operations[0].CardBody, "等待 headless 接手的 notice") || strings.Contains(gateway.operations[0].CardBody, "当前输入目标已切换到") {
		t.Fatalf("expected no stale replay or selection card on recovery, got %#v", gateway.operations[0])
	}
	if replay := app.service.Instance("inst-headless-1").Threads["thread-1"].UndeliveredReplay; replay != nil {
		t.Fatalf("expected stale replay to be drained after recovery, got %#v", replay)
	}
	if replay := app.service.Instance(pending.InstanceID).Threads["thread-1"].UndeliveredReplay; replay != nil {
		t.Fatalf("expected restored headless thread replay to stay empty, got %#v", replay)
	}
}

func TestDaemonAutoRestoreWaitsForFirstRefreshBeforeMissingNotice(t *testing.T) {
	t.Parallel()

	stateDir := t.TempDir()
	putRestoreHintForTest(t, stateDir, HeadlessRestoreHint{
		SurfaceSessionID: "surface-1",
		GatewayID:        "app-1",
		ChatID:           "chat-1",
		ActorUserID:      "user-1",
		ThreadID:         "thread-missing",
		ThreadTitle:      "旧会话",
	})
	gateway := &recordingGateway{}
	app := New(":0", ":0", gateway, agentproto.ServerIdentity{})
	app.SetHeadlessRuntime(HeadlessRuntimeConfig{
		IdleTTL:    time.Hour,
		KillGrace:  time.Second,
		Paths:      relayruntime.Paths{StateDir: stateDir},
		BinaryPath: "codex",
	})
	app.sendAgentCommand = func(string, agentproto.Command) error { return nil }

	base := time.Date(2026, 4, 8, 3, 30, 0, 0, time.UTC)
	app.onTick(context.Background(), base)
	if len(gateway.operations) != 0 {
		t.Fatalf("expected no recovery notice before first refresh round, got %#v", gateway.operations)
	}

	app.onHello(context.Background(), agentproto.Hello{
		Instance: agentproto.InstanceHello{
			InstanceID:    "inst-vscode-1",
			DisplayName:   "droid",
			WorkspaceRoot: "/data/dl/droid",
			WorkspaceKey:  "/data/dl/droid",
			ShortName:     "droid",
			Source:        "vscode",
		},
	})
	if len(gateway.operations) != 0 {
		t.Fatalf("expected no recovery notice while first refresh is still pending, got %#v", gateway.operations)
	}

	app.onTick(context.Background(), base.Add(2*time.Second))
	if len(gateway.operations) != 0 {
		t.Fatalf("expected no retry notice before first refresh snapshot, got %#v", gateway.operations)
	}

	app.onEvents(context.Background(), "inst-vscode-1", []agentproto.Event{{
		Kind:    agentproto.EventThreadsSnapshot,
		Threads: []agentproto.ThreadSnapshotRecord{{ThreadID: "thread-other", Name: "其他会话", CWD: "/data/dl/droid", Loaded: true}},
	}})
	if len(gateway.operations) != 1 {
		t.Fatalf("expected single missing-thread notice after first refresh settles, got %#v", gateway.operations)
	}
	if !strings.Contains(gateway.operations[0].CardBody, "暂时无法找到之前会话") {
		t.Fatalf("expected missing-thread recovery notice, got %#v", gateway.operations[0])
	}

	app.onTick(context.Background(), base.Add(5*time.Second))
	if len(gateway.operations) != 1 {
		t.Fatalf("expected backoff to suppress repeated recovery notice, got %#v", gateway.operations)
	}
}

func TestDaemonAutoRestoreLaunchFailureUsesBackoff(t *testing.T) {
	t.Parallel()

	stateDir := t.TempDir()
	putRestoreHintForTest(t, stateDir, HeadlessRestoreHint{
		SurfaceSessionID: "surface-1",
		GatewayID:        "app-1",
		ChatID:           "chat-1",
		ActorUserID:      "user-1",
		ThreadID:         "thread-1",
		ThreadTitle:      "修复登录流程",
		ThreadCWD:        "/data/dl/droid",
	})
	gateway := &recordingGateway{}
	app := New(":0", ":0", gateway, agentproto.ServerIdentity{})
	app.SetHeadlessRuntime(HeadlessRuntimeConfig{
		IdleTTL:    time.Hour,
		KillGrace:  time.Second,
		Paths:      relayruntime.Paths{StateDir: stateDir},
		BinaryPath: "codex",
	})
	app.startHeadless = func(relayruntime.HeadlessLaunchOptions) (int, error) {
		return 0, errors.New("spawn failed")
	}

	base := time.Date(2026, 4, 8, 3, 40, 0, 0, time.UTC)
	app.onTick(context.Background(), base)
	if len(gateway.operations) != 1 {
		t.Fatalf("expected one launch failure notice, got %#v", gateway.operations)
	}
	if !strings.Contains(gateway.operations[0].CardBody, "暂时无法恢复") {
		t.Fatalf("expected recovery launch failure notice, got %#v", gateway.operations[0])
	}

	app.onTick(context.Background(), base.Add(5*time.Second))
	if len(gateway.operations) != 1 {
		t.Fatalf("expected launch failure backoff to suppress retry noise, got %#v", gateway.operations)
	}
}

func newRestoreHintTestApp(stateDir string) *App {
	app := New(":0", ":0", &recordingGateway{}, agentproto.ServerIdentity{})
	app.SetHeadlessRuntime(HeadlessRuntimeConfig{
		IdleTTL:    time.Hour,
		KillGrace:  time.Second,
		Paths:      relayruntime.Paths{StateDir: stateDir},
		BinaryPath: "codex",
	})
	return app
}

func seedHeadlessInstance(app *App, instanceID, threadID string) {
	app.service.UpsertInstance(&state.InstanceRecord{
		InstanceID:              instanceID,
		DisplayName:             "headless",
		WorkspaceRoot:           "/data/dl/droid",
		WorkspaceKey:            "/data/dl/droid",
		ShortName:               "headless",
		Source:                  "headless",
		Managed:                 true,
		Online:                  true,
		ObservedFocusedThreadID: threadID,
		Threads: map[string]*state.ThreadRecord{
			threadID: {ThreadID: threadID, Name: "修复登录流程", CWD: "/data/dl/droid", Loaded: true},
		},
	})
}

func putRestoreHintForTest(t *testing.T, stateDir string, hint HeadlessRestoreHint) {
	t.Helper()
	hint, ok := normalizeHeadlessRestoreHint(hint)
	if !ok {
		t.Fatalf("normalize restore hint: %#v", hint)
	}
	store, err := loadSurfaceResumeStore(surfaceResumeStatePath(stateDir))
	if err != nil {
		t.Fatalf("load surface resume store: %v", err)
	}

	entry, exists := store.Get(hint.SurfaceSessionID)
	if !exists {
		entry = SurfaceResumeEntry{
			SurfaceSessionID: hint.SurfaceSessionID,
			GatewayID:        hint.GatewayID,
			ChatID:           hint.ChatID,
			ActorUserID:      hint.ActorUserID,
			ProductMode:      string(state.ProductModeNormal),
			UpdatedAt:        hint.UpdatedAt,
		}
	}
	entry.GatewayID = firstNonEmpty(entry.GatewayID, hint.GatewayID)
	entry.ChatID = firstNonEmpty(entry.ChatID, hint.ChatID)
	entry.ActorUserID = firstNonEmpty(entry.ActorUserID, hint.ActorUserID)
	if entry.ProductMode == "" {
		entry.ProductMode = string(state.ProductModeNormal)
	}
	if entry.ResumeThreadID == "" {
		entry.ResumeThreadID = hint.ThreadID
	}
	if entry.ResumeThreadTitle == "" {
		entry.ResumeThreadTitle = firstNonEmpty(hint.ThreadTitle, hint.ThreadID)
	}
	if entry.ResumeThreadCWD == "" {
		entry.ResumeThreadCWD = state.NormalizeWorkspaceKey(hint.ThreadCWD)
	}
	if entry.ResumeWorkspaceKey == "" {
		entry.ResumeWorkspaceKey = state.ResolveWorkspaceKey(entry.ResumeThreadCWD, hint.ThreadCWD)
	}
	if entry.ResumeRouteMode == "" {
		entry.ResumeRouteMode = string(state.RouteModePinned)
	}
	entry.ResumeHeadless = true
	if entry.UpdatedAt.IsZero() {
		entry.UpdatedAt = hint.UpdatedAt
	}
	if err := store.Put(entry); err != nil {
		t.Fatalf("put surface resume entry from restore hint: %v", err)
	}
}

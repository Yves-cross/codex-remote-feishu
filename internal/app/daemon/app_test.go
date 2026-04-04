package daemon

import (
	"context"
	"testing"
	"time"

	"github.com/kxn/codex-remote-feishu/internal/adapter/feishu"
	"github.com/kxn/codex-remote-feishu/internal/core/agentproto"
	"github.com/kxn/codex-remote-feishu/internal/core/control"
)

type recordingGateway struct {
	operations []feishu.Operation
}

func (g *recordingGateway) Start(context.Context, feishu.ActionHandler) error { return nil }

func (g *recordingGateway) Apply(_ context.Context, operations []feishu.Operation) error {
	g.operations = append(g.operations, operations...)
	return nil
}

type ctxCheckingGateway struct {
	ctxErr     error
	operations []feishu.Operation
}

func (g *ctxCheckingGateway) Start(context.Context, feishu.ActionHandler) error { return nil }

func (g *ctxCheckingGateway) Apply(ctx context.Context, operations []feishu.Operation) error {
	g.ctxErr = ctx.Err()
	g.operations = append(g.operations, operations...)
	return nil
}

func TestDaemonProjectsListAttachAndAssistantOutput(t *testing.T) {
	gateway := &recordingGateway{}
	app := New(":0", ":0", gateway)

	app.onHello(context.Background(), agentproto.Hello{
		Instance: agentproto.InstanceHello{
			InstanceID:    "inst-1",
			DisplayName:   "droid",
			WorkspaceRoot: "/data/dl/droid",
			WorkspaceKey:  "/data/dl/droid",
			ShortName:     "droid",
		},
	})
	app.onEvents(context.Background(), "inst-1", []agentproto.Event{{
		Kind:    agentproto.EventThreadsSnapshot,
		Threads: []agentproto.ThreadSnapshotRecord{{ThreadID: "thread-1", Name: "修复登录流程", CWD: "/data/dl/droid", Loaded: true}},
	}})
	app.onEvents(context.Background(), "inst-1", []agentproto.Event{{
		Kind:        agentproto.EventThreadFocused,
		ThreadID:    "thread-1",
		CWD:         "/data/dl/droid",
		FocusSource: "local_ui",
	}})

	app.HandleAction(context.Background(), control.Action{
		Kind:             control.ActionListInstances,
		SurfaceSessionID: "feishu:chat:1",
		ChatID:           "chat-1",
		ActorUserID:      "user-1",
	})
	app.HandleAction(context.Background(), control.Action{
		Kind:             control.ActionTextMessage,
		SurfaceSessionID: "feishu:chat:1",
		ChatID:           "chat-1",
		ActorUserID:      "user-1",
		MessageID:        "msg-select",
		Text:             "1",
	})
	app.HandleAction(context.Background(), control.Action{
		Kind:             control.ActionTextMessage,
		SurfaceSessionID: "feishu:chat:1",
		ChatID:           "chat-1",
		ActorUserID:      "user-1",
		MessageID:        "msg-1",
		Text:             "你好",
	})

	app.onEvents(context.Background(), "inst-1", []agentproto.Event{{
		Kind:      agentproto.EventTurnStarted,
		ThreadID:  "thread-1",
		TurnID:    "turn-1",
		Initiator: agentproto.Initiator{Kind: agentproto.InitiatorRemoteSurface, SurfaceSessionID: "feishu:chat:1"},
	}})
	app.onEvents(context.Background(), "inst-1", []agentproto.Event{{
		Kind:     agentproto.EventItemCompleted,
		ThreadID: "thread-1",
		TurnID:   "turn-1",
		ItemID:   "item-1",
		Metadata: map[string]any{"text": "已收到：\n\n```text\nREADME.md\n```"},
	}})
	app.onEvents(context.Background(), "inst-1", []agentproto.Event{{
		Kind:      agentproto.EventTurnCompleted,
		ThreadID:  "thread-1",
		TurnID:    "turn-1",
		Status:    "completed",
		Initiator: agentproto.Initiator{Kind: agentproto.InitiatorRemoteSurface, SurfaceSessionID: "feishu:chat:1"},
	}})

	var hasListCard bool
	var hasTyping bool
	var hasFinalReplyCard bool
	for _, operation := range gateway.operations {
		switch {
		case operation.Kind == feishu.OperationSendCard && operation.CardTitle == "在线实例":
			hasListCard = true
		case operation.Kind == feishu.OperationAddReaction && operation.MessageID == "msg-1":
			hasTyping = true
		case operation.Kind == feishu.OperationSendCard && operation.CardTitle == "最终回复 · droid · 修复登录流程":
			hasFinalReplyCard = operation.CardBody == "已收到：\n\n```text\nREADME.md\n```"
		}
	}
	if !hasListCard {
		t.Fatalf("expected online instance card, got %#v", gateway.operations)
	}
	if !hasTyping {
		t.Fatalf("expected typing reaction, got %#v", gateway.operations)
	}
	if !hasFinalReplyCard {
		t.Fatalf("expected final assistant reply card, got %#v", gateway.operations)
	}
}

func TestDaemonDecouplesGatewayApplyFromCanceledParentContext(t *testing.T) {
	gateway := &ctxCheckingGateway{}
	app := New(":0", ":0", gateway)

	app.onHello(context.Background(), agentproto.Hello{
		Instance: agentproto.InstanceHello{
			InstanceID:    "inst-1",
			DisplayName:   "droid",
			WorkspaceRoot: "/data/dl/droid",
			WorkspaceKey:  "/data/dl/droid",
			ShortName:     "droid",
		},
	})
	app.onEvents(context.Background(), "inst-1", []agentproto.Event{{
		Kind:    agentproto.EventThreadsSnapshot,
		Threads: []agentproto.ThreadSnapshotRecord{{ThreadID: "thread-1", Name: "修复登录流程", CWD: "/data/dl/droid", Loaded: true}},
	}})
	app.onEvents(context.Background(), "inst-1", []agentproto.Event{{
		Kind:        agentproto.EventThreadFocused,
		ThreadID:    "thread-1",
		CWD:         "/data/dl/droid",
		FocusSource: "local_ui",
	}})

	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	app.HandleAction(cancelledCtx, control.Action{
		Kind:             control.ActionListInstances,
		SurfaceSessionID: "feishu:chat:1",
		ChatID:           "chat-1",
		ActorUserID:      "user-1",
	})

	if gateway.ctxErr != nil {
		t.Fatalf("expected gateway apply context to be decoupled from canceled parent, got %v", gateway.ctxErr)
	}
	if len(gateway.operations) == 0 {
		t.Fatalf("expected gateway operations, got %#v", gateway.operations)
	}
}

func TestDaemonNotifiesAttachedSurfaceWhenInstanceDisconnects(t *testing.T) {
	gateway := &recordingGateway{}
	app := New(":0", ":0", gateway)

	app.onHello(context.Background(), agentproto.Hello{
		Instance: agentproto.InstanceHello{
			InstanceID:    "inst-1",
			DisplayName:   "droid",
			WorkspaceRoot: "/data/dl/droid",
			WorkspaceKey:  "/data/dl/droid",
			ShortName:     "droid",
		},
	})

	app.HandleAction(context.Background(), control.Action{
		Kind:             control.ActionListInstances,
		SurfaceSessionID: "feishu:chat:1",
		ChatID:           "chat-1",
		ActorUserID:      "user-1",
	})
	app.HandleAction(context.Background(), control.Action{
		Kind:             control.ActionTextMessage,
		SurfaceSessionID: "feishu:chat:1",
		ChatID:           "chat-1",
		ActorUserID:      "user-1",
		MessageID:        "msg-select",
		Text:             "1",
	})

	before := len(gateway.operations)
	app.onDisconnect(context.Background(), "inst-1")

	var hasOfflineNotice bool
	for _, operation := range gateway.operations[before:] {
		switch {
		case operation.Kind == feishu.OperationSendCard && operation.CardTitle == "系统提示" && operation.CardBody == "当前接管实例已离线：droid":
			hasOfflineNotice = true
		}
	}
	if !hasOfflineNotice {
		t.Fatalf("expected offline notice, got %#v", gateway.operations[before:])
	}
}

func TestDaemonTickResumesQueuedRemoteInputAfterLocalTurnCompletes(t *testing.T) {
	gateway := &recordingGateway{}
	app := New(":0", ":0", gateway)

	app.onHello(context.Background(), agentproto.Hello{
		Instance: agentproto.InstanceHello{
			InstanceID:    "inst-1",
			DisplayName:   "droid",
			WorkspaceRoot: "/data/dl/droid",
			WorkspaceKey:  "/data/dl/droid",
			ShortName:     "droid",
		},
	})
	app.onEvents(context.Background(), "inst-1", []agentproto.Event{{
		Kind:    agentproto.EventThreadsSnapshot,
		Threads: []agentproto.ThreadSnapshotRecord{{ThreadID: "thread-1", Name: "修复登录流程", CWD: "/data/dl/droid", Loaded: true}},
	}})
	app.HandleAction(context.Background(), control.Action{
		Kind:             control.ActionAttachInstance,
		SurfaceSessionID: "feishu:chat:1",
		ChatID:           "chat-1",
		ActorUserID:      "user-1",
		InstanceID:       "inst-1",
	})

	app.onEvents(context.Background(), "inst-1", []agentproto.Event{{
		Kind:     agentproto.EventLocalInteractionObserved,
		ThreadID: "thread-1",
		CWD:      "/data/dl/droid",
		Action:   "turn_start",
	}})
	app.HandleAction(context.Background(), control.Action{
		Kind:             control.ActionTextMessage,
		SurfaceSessionID: "feishu:chat:1",
		ChatID:           "chat-1",
		ActorUserID:      "user-1",
		MessageID:        "msg-queued",
		Text:             "列一下目录",
	})
	app.onEvents(context.Background(), "inst-1", []agentproto.Event{{
		Kind:      agentproto.EventTurnCompleted,
		ThreadID:  "thread-1",
		TurnID:    "turn-1",
		Status:    "completed",
		Initiator: agentproto.Initiator{Kind: agentproto.InitiatorLocalUI},
	}})
	app.onTick(context.Background(), time.Now().Add(2*time.Second))

	var hasTyping bool
	for _, operation := range gateway.operations {
		if operation.Kind == feishu.OperationAddReaction && operation.MessageID == "msg-queued" {
			hasTyping = true
		}
	}
	if !hasTyping {
		t.Fatalf("expected queued message to resume dispatch after tick, got %#v", gateway.operations)
	}
}

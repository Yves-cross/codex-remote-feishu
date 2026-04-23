package daemon

import (
	"context"
	"testing"

	"github.com/kxn/codex-remote-feishu/internal/conversationtrace"
	"github.com/kxn/codex-remote-feishu/internal/core/agentproto"
	"github.com/kxn/codex-remote-feishu/internal/core/control"
	"github.com/kxn/codex-remote-feishu/internal/core/eventcontract"
	"github.com/kxn/codex-remote-feishu/internal/core/render"
	"github.com/kxn/codex-remote-feishu/internal/core/state"
)

type traceCollector struct {
	entries []conversationtrace.Entry
}

func (c *traceCollector) Log(entry conversationtrace.Entry) {
	c.entries = append(c.entries, entry)
}

func (c *traceCollector) Close() error { return nil }

func findTraceEntry(entries []conversationtrace.Entry, event conversationtrace.EventKind) *conversationtrace.Entry {
	for i := range entries {
		if entries[i].Event == event {
			return &entries[i]
		}
	}
	return nil
}

func TestConversationTraceRecordsUserMessage(t *testing.T) {
	app := New(":0", ":0", &recordingGateway{}, agentproto.ServerIdentity{})
	trace := &traceCollector{}
	app.SetConversationTrace(trace)

	app.HandleAction(context.Background(), control.Action{
		Kind:             control.ActionTextMessage,
		SurfaceSessionID: "surface-1",
		ChatID:           "chat-1",
		ActorUserID:      "user-1",
		MessageID:        "msg-1",
		Text:             "你好",
	})

	entry := findTraceEntry(trace.entries, conversationtrace.EventUserMessage)
	if entry == nil {
		t.Fatalf("expected user_message trace entry, got %#v", trace.entries)
	}
	if entry.ChatID != "chat-1" || entry.MessageID != "msg-1" || entry.Text != "你好" {
		t.Fatalf("unexpected user_message trace payload: %#v", entry)
	}
}

func TestConversationTraceRecordsSteerMessage(t *testing.T) {
	app := New(":0", ":0", &recordingGateway{}, agentproto.ServerIdentity{})
	trace := &traceCollector{}
	app.SetConversationTrace(trace)

	var commands []agentproto.Command
	app.sendAgentCommand = func(_ string, command agentproto.Command) error {
		commands = append(commands, command)
		return nil
	}

	app.service.UpsertInstance(&state.InstanceRecord{
		InstanceID:              "inst-1",
		DisplayName:             "droid",
		WorkspaceRoot:           "/data/dl/droid",
		WorkspaceKey:            "/data/dl/droid",
		ShortName:               "droid",
		Online:                  true,
		ObservedFocusedThreadID: "thread-1",
		Threads: map[string]*state.ThreadRecord{
			"thread-1": {ThreadID: "thread-1", Name: "修复登录流程", CWD: "/data/dl/droid"},
		},
	})
	app.HandleAction(context.Background(), control.Action{
		Kind:             control.ActionAttachInstance,
		SurfaceSessionID: "feishu:app-1:chat:1",
		ChatID:           "chat-1",
		ActorUserID:      "user-1",
		InstanceID:       "inst-1",
	})
	app.HandleAction(context.Background(), control.Action{
		Kind:             control.ActionUseThread,
		SurfaceSessionID: "feishu:app-1:chat:1",
		ChatID:           "chat-1",
		ActorUserID:      "user-1",
		ThreadID:         "thread-1",
	})
	app.HandleAction(context.Background(), control.Action{
		Kind:             control.ActionTextMessage,
		SurfaceSessionID: "feishu:app-1:chat:1",
		ChatID:           "chat-1",
		ActorUserID:      "user-1",
		MessageID:        "msg-active",
		Text:             "先开始",
	})
	app.onEvents(context.Background(), "inst-1", []agentproto.Event{{
		Kind:      agentproto.EventTurnStarted,
		ThreadID:  "thread-1",
		TurnID:    "turn-1",
		Initiator: agentproto.Initiator{Kind: agentproto.InitiatorUnknown},
	}})
	app.HandleAction(context.Background(), control.Action{
		Kind:             control.ActionTextMessage,
		SurfaceSessionID: "feishu:app-1:chat:1",
		ChatID:           "chat-1",
		ActorUserID:      "user-1",
		MessageID:        "msg-queued",
		Text:             "补充信息",
	})
	app.HandleAction(context.Background(), control.Action{
		Kind:             control.ActionReactionCreated,
		SurfaceSessionID: "feishu:app-1:chat:1",
		ChatID:           "chat-1",
		ActorUserID:      "user-1",
		TargetMessageID:  "msg-queued",
		ReactionType:     "ThumbsUp",
	})

	if len(commands) < 2 {
		t.Fatalf("expected steer command to be dispatched, got %#v", commands)
	}
	entry := findTraceEntry(trace.entries, conversationtrace.EventSteerMessage)
	if entry == nil {
		t.Fatalf("expected steer_message trace entry, got %#v", trace.entries)
	}
	if entry.MessageID != "msg-queued" || entry.ThreadID != "thread-1" || entry.TurnID != "turn-1" || entry.Text != "补充信息" {
		t.Fatalf("unexpected steer_message trace payload: %#v", entry)
	}
}

func TestConversationTraceRecordsAssistantTextForFinalAndNonFinal(t *testing.T) {
	app := New(":0", ":0", &recordingGateway{}, agentproto.ServerIdentity{})
	trace := &traceCollector{}
	app.SetConversationTrace(trace)

	app.service.UpsertInstance(&state.InstanceRecord{
		InstanceID:    "inst-1",
		DisplayName:   "droid",
		WorkspaceRoot: "/data/dl/droid",
		WorkspaceKey:  "/data/dl/droid",
		ShortName:     "droid",
		Online:        true,
		Threads:       map[string]*state.ThreadRecord{},
	})
	app.HandleAction(context.Background(), control.Action{
		Kind:             control.ActionAttachInstance,
		SurfaceSessionID: "feishu:app-1:chat:1",
		ChatID:           "chat-1",
		ActorUserID:      "user-1",
		InstanceID:       "inst-1",
	})

	if err := app.deliverUIEventWithContext(context.Background(), eventcontract.Event{
		Kind:             eventcontract.EventBlockCommitted,
		SurfaceSessionID: "feishu:app-1:chat:1",
		SourceMessageID:  "msg-1",
		Block: &render.Block{
			Kind:     render.BlockAssistantMarkdown,
			ThreadID: "thread-1",
			TurnID:   "turn-1",
			Text:     "先确认一下现状。",
			Final:    false,
		},
	}); err != nil {
		t.Fatalf("deliver non-final block: %v", err)
	}
	if err := app.deliverUIEventWithContext(context.Background(), eventcontract.Event{
		Kind:             eventcontract.EventBlockCommitted,
		SurfaceSessionID: "feishu:app-1:chat:1",
		SourceMessageID:  "msg-1",
		Block: &render.Block{
			Kind:     render.BlockAssistantMarkdown,
			ThreadID: "thread-1",
			TurnID:   "turn-1",
			Text:     "已经处理完成。",
			Final:    true,
		},
	}); err != nil {
		t.Fatalf("deliver final block: %v", err)
	}

	var nonFinal, final *conversationtrace.Entry
	for i := range trace.entries {
		if trace.entries[i].Event != conversationtrace.EventAssistantText {
			continue
		}
		if trace.entries[i].Final {
			final = &trace.entries[i]
		} else {
			nonFinal = &trace.entries[i]
		}
	}
	if nonFinal == nil || final == nil {
		t.Fatalf("expected both final and non-final assistant_text traces, got %#v", trace.entries)
	}
	if nonFinal.Text != "先确认一下现状。" || final.Text != "已经处理完成。" {
		t.Fatalf("unexpected assistant_text entries: non-final=%#v final=%#v", nonFinal, final)
	}
}

func TestConversationTraceRecordsTurnLifecycleTimeline(t *testing.T) {
	app := New(":0", ":0", &recordingGateway{}, agentproto.ServerIdentity{})
	trace := &traceCollector{}
	app.SetConversationTrace(trace)

	app.sendAgentCommand = func(_ string, _ agentproto.Command) error { return nil }
	app.service.UpsertInstance(&state.InstanceRecord{
		InstanceID:              "inst-1",
		DisplayName:             "droid",
		WorkspaceRoot:           "/data/dl/droid",
		WorkspaceKey:            "/data/dl/droid",
		ShortName:               "droid",
		Online:                  true,
		ObservedFocusedThreadID: "thread-1",
		Threads: map[string]*state.ThreadRecord{
			"thread-1": {ThreadID: "thread-1", Name: "修复登录流程", CWD: "/data/dl/droid"},
		},
	})
	app.HandleAction(context.Background(), control.Action{
		Kind:             control.ActionAttachInstance,
		SurfaceSessionID: "feishu:app-1:chat:1",
		ChatID:           "chat-1",
		ActorUserID:      "user-1",
		InstanceID:       "inst-1",
	})
	app.HandleAction(context.Background(), control.Action{
		Kind:             control.ActionUseThread,
		SurfaceSessionID: "feishu:app-1:chat:1",
		ChatID:           "chat-1",
		ActorUserID:      "user-1",
		ThreadID:         "thread-1",
	})
	app.HandleAction(context.Background(), control.Action{
		Kind:             control.ActionTextMessage,
		SurfaceSessionID: "feishu:app-1:chat:1",
		ChatID:           "chat-1",
		ActorUserID:      "user-1",
		MessageID:        "msg-1",
		Text:             "处理一下",
	})

	app.onEvents(context.Background(), "inst-1", []agentproto.Event{{
		Kind:      agentproto.EventTurnStarted,
		ThreadID:  "thread-1",
		TurnID:    "turn-1",
		Initiator: agentproto.Initiator{Kind: agentproto.InitiatorUnknown},
	}})
	app.onEvents(context.Background(), "inst-1", []agentproto.Event{{
		Kind:      agentproto.EventTurnCompleted,
		ThreadID:  "thread-1",
		TurnID:    "turn-1",
		Status:    "completed",
		Initiator: agentproto.Initiator{Kind: agentproto.InitiatorUnknown},
	}})

	start := findTraceEntry(trace.entries, conversationtrace.EventTurnStarted)
	done := findTraceEntry(trace.entries, conversationtrace.EventTurnCompleted)
	if start == nil || done == nil {
		t.Fatalf("expected turn lifecycle entries, got %#v", trace.entries)
	}
	if start.MessageID != "msg-1" || done.MessageID != "msg-1" {
		t.Fatalf("expected turn lifecycle to keep source message id, got started=%#v completed=%#v", start, done)
	}
	if done.Status != "completed" {
		t.Fatalf("expected completed status, got %#v", done)
	}
}

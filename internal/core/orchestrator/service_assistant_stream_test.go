package orchestrator

import (
	"strings"
	"testing"
	"time"

	"github.com/kxn/codex-remote-feishu/internal/core/agentproto"
	"github.com/kxn/codex-remote-feishu/internal/core/control"
	"github.com/kxn/codex-remote-feishu/internal/core/state"
)

func TestCommentaryAssistantDeltaReusesSingleStreamUntilFinal(t *testing.T) {
	now := time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC)
	svc := newServiceForTest(&now)
	svc.UpsertInstance(&state.InstanceRecord{
		InstanceID:              "inst-1",
		DisplayName:             "droid",
		WorkspaceRoot:           "/data/dl/droid",
		WorkspaceKey:            "/data/dl/droid",
		ShortName:               "droid",
		Online:                  true,
		ObservedFocusedThreadID: "thread-2",
		Threads: map[string]*state.ThreadRecord{
			"thread-2": {ThreadID: "thread-2", Name: "修复登录流程", CWD: "/data/dl/droid"},
		},
	})
	svc.ApplySurfaceAction(control.Action{Kind: control.ActionAttachInstance, SurfaceSessionID: "surface-1", ChatID: "chat-1", ActorUserID: "user-1", InstanceID: "inst-1"})

	started := svc.ApplyAgentEvent("inst-1", agentproto.Event{
		Kind:     agentproto.EventItemStarted,
		ThreadID: "thread-2",
		TurnID:   "turn-1",
		ItemID:   "item-1",
		ItemKind: "agent_message",
		Metadata: map[string]any{"phase": "commentary"},
	})
	if len(started) != 1 || started[0].AssistantStream == nil || started[0].AssistantStream.Text != "" || !started[0].AssistantStream.Loading {
		t.Fatalf("expected commentary start to emit loading stream, got %#v", started)
	}
	svc.RecordAssistantStreamMessage("surface-1", "thread-2", "turn-1", "item-1", "om-stream-1", "card-stream-1")

	now = now.Add(assistantStreamLoadingInterval)
	tick := svc.Tick(now)
	if len(tick) != 0 {
		t.Fatalf("expected empty loading tick not to patch native stream content, got %#v", tick)
	}

	first := svc.ApplyAgentEvent("inst-1", agentproto.Event{
		Kind:     agentproto.EventItemDelta,
		ThreadID: "thread-2",
		TurnID:   "turn-1",
		ItemID:   "item-1",
		ItemKind: "agent_message",
		Delta:    "继续执行刚才被中断的验证和安装。",
	})
	if len(first) != 1 || first[0].AssistantStream == nil || first[0].AssistantStream.MessageID != "om-stream-1" || first[0].AssistantStream.StreamCardID != "card-stream-1" || first[0].AssistantStream.Text != "继续执行刚才被中断的验证和安装。" || !first[0].AssistantStream.Loading || first[0].AssistantStream.Done {
		t.Fatalf("expected commentary delta to start assistant stream, got %#v", first)
	}
	now = now.Add(assistantStreamLoadingInterval)
	waitingForNextPartial := svc.Tick(now)
	if len(waitingForNextPartial) != 1 || waitingForNextPartial[0].AssistantStream == nil || waitingForNextPartial[0].AssistantStream.Text != "继续执行刚才被中断的验证和安装。" || !waitingForNextPartial[0].AssistantStream.Loading {
		t.Fatalf("expected loading marker to remain while waiting for next partial, got %#v", waitingForNextPartial)
	}

	completed := svc.ApplyAgentEvent("inst-1", agentproto.Event{
		Kind:     agentproto.EventItemCompleted,
		ThreadID: "thread-2",
		TurnID:   "turn-1",
		ItemID:   "item-1",
		ItemKind: "agent_message",
	})
	if len(completed) != 0 {
		t.Fatalf("expected commentary completion to keep stream open silently, got %#v", completed)
	}
	if active := svc.root.Surfaces["surface-1"].ActiveAssistantStream; active != nil {
		if active.MessageID != "om-stream-1" || active.StreamCardID != "card-stream-1" || active.CompletedText != "继续执行刚才被中断的验证和安装。" {
			t.Fatalf("expected active stream to keep completed commentary, got %#v", active)
		}
	} else {
		t.Fatalf("expected active stream to remain open")
	}

	next := svc.ApplyAgentEvent("inst-1", agentproto.Event{
		Kind:     agentproto.EventItemStarted,
		ThreadID: "thread-2",
		TurnID:   "turn-1",
		ItemID:   "cmd-1",
		ItemKind: "command_execution",
		Metadata: map[string]any{"command": "go test ./..."},
	})
	for _, event := range next {
		if event.Block != nil && strings.Contains(event.Block.Text, "继续执行刚才") {
			t.Fatalf("expected streamed commentary not to flush again as text block, got %#v", next)
		}
	}

	svc.ApplyAgentEvent("inst-1", agentproto.Event{
		Kind:     agentproto.EventItemStarted,
		ThreadID: "thread-2",
		TurnID:   "turn-1",
		ItemID:   "item-2",
		ItemKind: "agent_message",
		Metadata: map[string]any{"phase": "final_answer"},
	})
	now = now.Add(assistantStreamMaxInterval)
	finalDelta := svc.ApplyAgentEvent("inst-1", agentproto.Event{
		Kind:     agentproto.EventItemDelta,
		ThreadID: "thread-2",
		TurnID:   "turn-1",
		ItemID:   "item-2",
		ItemKind: "agent_message",
		Delta:    "最终答复。",
	})
	wantText := "继续执行刚才被中断的验证和安装。\n\n最终答复。"
	if len(finalDelta) != 1 || finalDelta[0].AssistantStream == nil {
		t.Fatalf("expected final delta to update existing stream, got %#v", finalDelta)
	}
	if stream := finalDelta[0].AssistantStream; stream.MessageID != "om-stream-1" || stream.StreamCardID != "card-stream-1" || stream.Text != wantText || !stream.Loading || stream.Done {
		t.Fatalf("expected final delta to reuse stream card, got %#v", stream)
	}
	svc.ApplyAgentEvent("inst-1", agentproto.Event{
		Kind:     agentproto.EventItemCompleted,
		ThreadID: "thread-2",
		TurnID:   "turn-1",
		ItemID:   "item-2",
		ItemKind: "agent_message",
	})
	finished := svc.ApplyAgentEvent("inst-1", agentproto.Event{
		Kind:      agentproto.EventTurnCompleted,
		ThreadID:  "thread-2",
		TurnID:    "turn-1",
		Status:    "completed",
		Initiator: agentproto.Initiator{Kind: agentproto.InitiatorRemoteSurface, SurfaceSessionID: "surface-1"},
	})
	if len(finished) != 1 || finished[0].Block == nil || !finished[0].Block.Final {
		t.Fatalf("expected final block to close single stream card, got %#v", finished)
	}
	if finished[0].Block.MessageID != "om-stream-1" || finished[0].Block.StreamCardID != "card-stream-1" || finished[0].Block.Text != wantText {
		t.Fatalf("expected final block to close same stream card with merged text, got %#v", finished[0].Block)
	}
}

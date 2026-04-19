package daemon

import (
	"testing"
	"time"

	"github.com/kxn/codex-remote-feishu/internal/adapter/feishu"
	"github.com/kxn/codex-remote-feishu/internal/core/agentproto"
	"github.com/kxn/codex-remote-feishu/internal/core/control"
	"github.com/kxn/codex-remote-feishu/internal/core/state"
)

func TestRecordUIEventDeliveryTracksNewestExecProgressContinuationCard(t *testing.T) {
	app := New(":0", ":0", &recordingGateway{}, agentproto.ServerIdentity{
		PID:       42,
		StartedAt: time.Date(2026, 4, 19, 10, 0, 0, 0, time.UTC),
	})
	app.service.MaterializeSurface("surface-1", "app-1", "chat-1", "user-1")
	surfaces := app.service.Surfaces()
	if len(surfaces) != 1 {
		t.Fatalf("expected one surface, got %d", len(surfaces))
	}
	surface := surfaces[0]
	surface.ActiveExecProgress = &state.ExecCommandProgressRecord{
		ThreadID:      "thread-1",
		TurnID:        "turn-1",
		ItemID:        "cmd-3",
		MessageID:     "om-progress-1",
		CardStartSeq:  1,
		Status:        "running",
		LastEmittedAt: time.Date(2026, 4, 19, 10, 1, 0, 0, time.UTC),
	}

	app.recordUIEventDelivery(control.UIEvent{
		SurfaceSessionID: "surface-1",
		ExecCommandProgress: &control.ExecCommandProgress{
			ThreadID: "thread-1",
			TurnID:   "turn-1",
			ItemID:   "cmd-3",
		},
	}, []feishu.Operation{
		{Kind: feishu.OperationUpdateCard, MessageID: "om-progress-1", ProgressCardStartSeq: 1},
		{Kind: feishu.OperationSendCard, MessageID: "om-progress-2", ProgressCardStartSeq: 40},
		{Kind: feishu.OperationSendCard, MessageID: "om-progress-3", ProgressCardStartSeq: 73},
	})

	if surface.ActiveExecProgress.MessageID != "om-progress-3" || surface.ActiveExecProgress.CardStartSeq != 73 {
		t.Fatalf("expected last continuation card to become active patch target, got %#v", surface.ActiveExecProgress)
	}
}

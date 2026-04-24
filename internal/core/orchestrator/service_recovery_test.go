package orchestrator

import (
	"strings"
	"testing"
	"time"

	"github.com/kxn/codex-remote-feishu/internal/core/agentproto"
	"github.com/kxn/codex-remote-feishu/internal/core/control"
	"github.com/kxn/codex-remote-feishu/internal/core/state"
)

func TestRecoveryCommandUpdatesSnapshotWithoutAttach(t *testing.T) {
	now := time.Date(2026, 4, 9, 10, 5, 0, 0, time.UTC)
	svc := newServiceForTest(&now)

	enabled := svc.ApplySurfaceAction(control.Action{
		Kind:             control.ActionRecoveryCommand,
		SurfaceSessionID: "surface-1",
		ChatID:           "chat-1",
		ActorUserID:      "user-1",
		Text:             "/recovery on",
	})
	if len(enabled) != 1 || enabled[0].Notice == nil || enabled[0].Notice.Code != "recovery_enabled" {
		t.Fatalf("expected enable notice, got %#v", enabled)
	}

	snapshot := svc.SurfaceSnapshot("surface-1")
	if snapshot == nil {
		t.Fatal("expected snapshot after enable")
	}
	if !snapshot.Recovery.Enabled {
		t.Fatalf("expected recovery enabled in snapshot, got %#v", snapshot.Recovery)
	}

	disabled := svc.ApplySurfaceAction(control.Action{
		Kind:             control.ActionRecoveryCommand,
		SurfaceSessionID: "surface-1",
		Text:             "/recovery off",
	})
	if len(disabled) != 1 || disabled[0].Notice == nil || disabled[0].Notice.Code != "recovery_disabled" {
		t.Fatalf("expected disable notice, got %#v", disabled)
	}
	if snapshot := svc.SurfaceSnapshot("surface-1"); snapshot == nil || snapshot.Recovery.Enabled {
		t.Fatalf("expected recovery disabled in snapshot, got %#v", snapshot)
	}
}

func TestSurfaceSnapshotIncludesRecoverySummary(t *testing.T) {
	now := time.Date(2026, 4, 9, 11, 35, 0, 0, time.UTC)
	svc := newServiceForTest(&now)
	svc.root.Surfaces["surface-1"] = &state.SurfaceConsoleRecord{
		SurfaceSessionID: "surface-1",
		DispatchMode:     state.DispatchModeNormal,
		QueueItems:       map[string]*state.QueueItemRecord{},
		StagedImages:     map[string]*state.StagedImageRecord{},
		PendingRequests:  map[string]*state.RequestPromptRecord{},
		Recovery: state.RecoveryRuntimeRecord{
			Enabled: true,
			Episode: &state.PendingRecoveryEpisodeRecord{
				EpisodeID:                  "recovery-1",
				State:                      state.RecoveryEpisodeScheduled,
				AttemptCount:               3,
				ConsecutiveDryFailureCount: 2,
				PendingDueAt:               now.Add(5 * time.Second),
				TriggerKind:                state.RecoveryTriggerKindUpstreamRetryableFailure,
			},
		},
	}

	snapshot := svc.SurfaceSnapshot("surface-1")
	if snapshot == nil {
		t.Fatal("expected snapshot")
	}
	if !snapshot.Recovery.Enabled ||
		snapshot.Recovery.State != string(state.RecoveryEpisodeScheduled) ||
		!snapshot.Recovery.PendingDueAt.Equal(now.Add(5*time.Second)) ||
		snapshot.Recovery.AttemptCount != 3 ||
		snapshot.Recovery.ConsecutiveDryFailureCount != 2 ||
		snapshot.Recovery.TriggerKind != string(state.RecoveryTriggerKindUpstreamRetryableFailure) {
		t.Fatalf("unexpected recovery snapshot: %#v", snapshot.Recovery)
	}
}

func TestRecoveryDispatchesRetryableFailureImmediately(t *testing.T) {
	now := time.Date(2026, 4, 9, 12, 5, 0, 0, time.UTC)
	svc := newServiceForTest(&now)
	surface := setupAutoContinueSurface(t, svc)
	surface.Recovery.Enabled = true

	startRemoteTurnForAutoContinueTest(t, svc, "msg-1", "继续处理", "turn-1")
	events := completeRemoteTurnWithFinalText(t, svc, "turn-1", "interrupted", "upstream stream closed", "", &agentproto.ErrorInfo{
		Code:      "responseStreamDisconnected",
		Layer:     "codex",
		Stage:     "runtime_error",
		Message:   "upstream stream closed",
		ThreadID:  "thread-1",
		TurnID:    "turn-1",
		Retryable: true,
	})

	if surface.AutoContinue.PendingReason != "" {
		t.Fatalf("expected retryable failure to stay out of autowhip runtime, got %#v", surface.AutoContinue)
	}
	episode := surface.Recovery.Episode
	if episode == nil {
		t.Fatal("expected recovery episode")
	}
	if episode.State != state.RecoveryEpisodeRunning || episode.AttemptCount != 1 || episode.ConsecutiveDryFailureCount != 1 {
		t.Fatalf("expected immediate first recovery attempt, got %#v", episode)
	}
	if surface.ActiveQueueItemID == "" {
		t.Fatalf("expected immediate recovery dispatch to occupy active queue")
	}
	active := surface.QueueItems[surface.ActiveQueueItemID]
	if active == nil || active.SourceKind != state.QueueItemSourceRecovery || active.RecoveryEpisodeID != episode.EpisodeID {
		t.Fatalf("expected recovery queue item to dispatch, got %#v", active)
	}
	var sawTurnFailedNotice bool
	var sawRecoveryCard bool
	var sawRecoveryPrompt bool
	for _, event := range events {
		if event.Notice != nil && event.Notice.Code == "turn_failed" {
			sawTurnFailedNotice = true
		}
		if event.PageView != nil && strings.TrimSpace(event.PageView.TrackingKey) == episode.EpisodeID {
			sawRecoveryCard = true
		}
		if event.Command != nil && event.Command.Kind == agentproto.CommandPromptSend && len(event.Command.Prompt.Inputs) == 1 && event.Command.Prompt.Inputs[0].Text == recoveryContinuePromptText {
			sawRecoveryPrompt = true
		}
	}
	if sawTurnFailedNotice {
		t.Fatalf("expected recovery path to suppress direct turn_failed notice, got %#v", events)
	}
	if !sawRecoveryCard || !sawRecoveryPrompt {
		t.Fatalf("expected recovery card plus prompt dispatch, got %#v", events)
	}
}

func TestRecoveryDoesNotScheduleAfterUserStopEvenWithRetryableProblem(t *testing.T) {
	now := time.Date(2026, 4, 9, 12, 15, 0, 0, time.UTC)
	svc := newServiceForTest(&now)
	surface := setupAutoContinueSurface(t, svc)
	surface.Recovery.Enabled = true
	startRemoteTurnForAutoContinueTest(t, svc, "msg-1", "继续处理", "turn-1")

	stopEvents := svc.ApplySurfaceAction(control.Action{
		Kind:             control.ActionStop,
		SurfaceSessionID: "surface-1",
		ChatID:           "chat-1",
		ActorUserID:      "user-1",
	})
	if len(stopEvents) == 0 {
		t.Fatal("expected stop events")
	}

	events := svc.ApplyAgentEvent("inst-1", agentproto.Event{
		Kind:         agentproto.EventTurnCompleted,
		ThreadID:     "thread-1",
		TurnID:       "turn-1",
		Status:       "interrupted",
		ErrorMessage: "stream disconnected before completion",
		Problem: &agentproto.ErrorInfo{
			Code:      "responseStreamDisconnected",
			Layer:     "codex",
			Stage:     "runtime_error",
			Message:   "stream disconnected before completion",
			Retryable: true,
		},
	})
	if episode := surface.Recovery.Episode; episode != nil {
		t.Fatalf("expected /stop to suppress recovery scheduling, got %#v", episode)
	}
	if active := surface.ActiveQueueItemID; active != "" {
		t.Fatalf("expected no recovery dispatch after /stop, got active %q", active)
	}
	for _, event := range events {
		if event.Notice != nil && event.Notice.Code == "turn_failed" {
			t.Fatalf("expected user stop not to emit failure notice, got %#v", events)
		}
		if event.Command != nil && event.Command.Kind == agentproto.CommandPromptSend && len(event.Command.Prompt.Inputs) == 1 && event.Command.Prompt.Inputs[0].Text == recoveryContinuePromptText {
			t.Fatalf("expected user stop not to trigger recovery prompt, got %#v", events)
		}
	}
}

func TestDetachClearsPendingRecoveryEpisode(t *testing.T) {
	now := time.Date(2026, 4, 9, 12, 18, 0, 0, time.UTC)
	svc := newServiceForTest(&now)
	surface := setupAutoContinueSurface(t, svc)
	surface.Recovery.Enabled = true
	surface.Recovery.Episode = &state.PendingRecoveryEpisodeRecord{
		EpisodeID:       "recovery-1",
		InstanceID:      "inst-1",
		ThreadID:        "thread-1",
		FrozenCWD:       "/data/dl/droid",
		FrozenRouteMode: state.RouteModePinned,
		State:           state.RecoveryEpisodeScheduled,
		PendingDueAt:    now.Add(5 * time.Second),
	}

	events := svc.ApplySurfaceAction(control.Action{
		Kind:             control.ActionDetach,
		SurfaceSessionID: "surface-1",
		ChatID:           "chat-1",
		ActorUserID:      "user-1",
	})
	if len(events) == 0 {
		t.Fatal("expected detach events")
	}
	if !surface.Recovery.Enabled || surface.Recovery.Episode != nil {
		t.Fatalf("expected detach to preserve recovery toggle but clear pending episode, got %#v", surface.Recovery)
	}
}

func TestNewThreadReadyClearsPendingRecoveryEpisode(t *testing.T) {
	now := time.Date(2026, 4, 9, 12, 19, 0, 0, time.UTC)
	svc := newServiceForTest(&now)
	surface := setupAutoContinueSurface(t, svc)
	surface.Recovery.Enabled = true
	surface.Recovery.Episode = &state.PendingRecoveryEpisodeRecord{
		EpisodeID:       "recovery-1",
		InstanceID:      "inst-1",
		ThreadID:        "thread-1",
		FrozenCWD:       "/data/dl/droid",
		FrozenRouteMode: state.RouteModePinned,
		State:           state.RecoveryEpisodeScheduled,
		PendingDueAt:    now.Add(5 * time.Second),
	}

	events := svc.ApplySurfaceAction(control.Action{
		Kind:             control.ActionNewThread,
		SurfaceSessionID: "surface-1",
		ChatID:           "chat-1",
		ActorUserID:      "user-1",
	})
	if len(events) == 0 {
		t.Fatal("expected /new events")
	}
	if !surface.Recovery.Enabled || surface.Recovery.Episode != nil {
		t.Fatalf("expected /new to preserve recovery toggle but clear pending episode, got %#v", surface.Recovery)
	}
}

func TestRecoveryPrioritizesQueuedUserInput(t *testing.T) {
	now := time.Date(2026, 4, 9, 12, 20, 0, 0, time.UTC)
	svc := newServiceForTest(&now)
	surface := setupAutoContinueSurface(t, svc)
	surface.Recovery.Enabled = true
	startRemoteTurnForAutoContinueTest(t, svc, "msg-1", "继续处理", "turn-1")

	queued := svc.ApplySurfaceAction(control.Action{
		Kind:             control.ActionTextMessage,
		SurfaceSessionID: "surface-1",
		MessageID:        "msg-2",
		Text:             "后面的补充消息",
	})
	if len(queued) == 0 {
		t.Fatal("expected queued user input events")
	}

	events := svc.ApplyAgentEvent("inst-1", agentproto.Event{
		Kind:         agentproto.EventTurnCompleted,
		ThreadID:     "thread-1",
		TurnID:       "turn-1",
		Status:       "interrupted",
		ErrorMessage: "stream disconnected before completion",
		Problem: &agentproto.ErrorInfo{
			Code:      "responseStreamDisconnected",
			Layer:     "codex",
			Stage:     "runtime_error",
			Message:   "stream disconnected before completion",
			Retryable: true,
		},
	})
	active := surface.QueueItems[surface.ActiveQueueItemID]
	if active == nil || active.SourceKind != state.QueueItemSourceRecovery {
		t.Fatalf("expected recovery to dispatch before queued user input, got active=%#v queued=%#v", active, surface.QueuedQueueItemIDs)
	}
	if len(surface.QueuedQueueItemIDs) != 1 {
		t.Fatalf("expected original queued user input to remain queued, got %#v", surface.QueuedQueueItemIDs)
	}
	queuedItem := surface.QueueItems[surface.QueuedQueueItemIDs[0]]
	if queuedItem == nil || queuedItem.SourceKind != state.QueueItemSourceUser || queuedItem.SourceMessageID != "msg-2" {
		t.Fatalf("expected queued user item to remain intact, got %#v", queuedItem)
	}
	var sawRecoveryPrompt bool
	for _, event := range events {
		if event.Command != nil && event.Command.Kind == agentproto.CommandPromptSend && len(event.Command.Prompt.Inputs) == 1 && event.Command.Prompt.Inputs[0].Text == recoveryContinuePromptText {
			sawRecoveryPrompt = true
		}
	}
	if !sawRecoveryPrompt {
		t.Fatalf("expected recovery dispatch command, got %#v", events)
	}
}

func TestRecoveryOutputResetsDryFailureBackoff(t *testing.T) {
	now := time.Date(2026, 4, 9, 12, 30, 0, 0, time.UTC)
	svc := newServiceForTest(&now)
	surface := setupAutoContinueSurface(t, svc)
	surface.Recovery.Enabled = true
	startRemoteTurnForAutoContinueTest(t, svc, "msg-1", "继续处理", "turn-1")

	first := svc.ApplyAgentEvent("inst-1", agentproto.Event{
		Kind:         agentproto.EventTurnCompleted,
		ThreadID:     "thread-1",
		TurnID:       "turn-1",
		Status:       "interrupted",
		ErrorMessage: "stream disconnected before completion",
		Problem: &agentproto.ErrorInfo{
			Code:      "responseStreamDisconnected",
			Layer:     "codex",
			Stage:     "runtime_error",
			Message:   "stream disconnected before completion",
			Retryable: true,
		},
	})
	if len(first) == 0 {
		t.Fatal("expected first recovery dispatch")
	}
	svc.ApplyAgentEvent("inst-1", agentproto.Event{
		Kind:      agentproto.EventTurnStarted,
		ThreadID:  "thread-1",
		TurnID:    "turn-2",
		Initiator: agentproto.Initiator{Kind: agentproto.InitiatorUnknown},
	})
	if delta := svc.ApplyAgentEvent("inst-1", agentproto.Event{
		Kind:     agentproto.EventItemDelta,
		ThreadID: "thread-1",
		TurnID:   "turn-2",
		ItemID:   "item-2",
		Delta:    "先输出一点内容",
	}); len(delta) != 0 {
		t.Fatalf("expected delta to stay buffered, got %#v", delta)
	}
	second := svc.ApplyAgentEvent("inst-1", agentproto.Event{
		Kind:         agentproto.EventTurnCompleted,
		ThreadID:     "thread-1",
		TurnID:       "turn-2",
		Status:       "interrupted",
		ErrorMessage: "stream disconnected again",
		Problem: &agentproto.ErrorInfo{
			Code:      "responseStreamDisconnected",
			Layer:     "codex",
			Stage:     "runtime_error",
			Message:   "stream disconnected again",
			Retryable: true,
		},
	})
	episode := surface.Recovery.Episode
	if episode == nil {
		t.Fatal("expected recovery episode after second failure")
	}
	if episode.State != state.RecoveryEpisodeRunning || episode.AttemptCount != 2 || episode.ConsecutiveDryFailureCount != 1 {
		t.Fatalf("expected output to reset dry failure backoff before next retry, got %#v", episode)
	}
	var sawRecoveryPrompt bool
	for _, event := range second {
		if event.Command != nil && event.Command.Kind == agentproto.CommandPromptSend && len(event.Command.Prompt.Inputs) == 1 && event.Command.Prompt.Inputs[0].Text == recoveryContinuePromptText {
			sawRecoveryPrompt = true
		}
	}
	if !sawRecoveryPrompt {
		t.Fatalf("expected second recovery attempt to dispatch immediately after outputful failure, got %#v", second)
	}
}

func TestRecoveryStatusCardStopsPatchingAfterTailMoves(t *testing.T) {
	now := time.Date(2026, 4, 9, 12, 35, 0, 0, time.UTC)
	svc := newServiceForTest(&now)
	surface := setupAutoContinueSurface(t, svc)
	surface.Recovery.Enabled = true
	startRemoteTurnForAutoContinueTest(t, svc, "msg-1", "继续处理", "turn-1")

	_ = completeRemoteTurnWithFinalText(t, svc, "turn-1", "interrupted", "upstream stream closed", "", &agentproto.ErrorInfo{
		Code:      "responseStreamDisconnected",
		Layer:     "codex",
		Stage:     "runtime_error",
		Message:   "upstream stream closed",
		ThreadID:  "thread-1",
		TurnID:    "turn-1",
		Retryable: true,
	})
	episode := surface.Recovery.Episode
	if episode == nil {
		t.Fatal("expected recovery episode")
	}

	svc.RecordSurfaceOutboundMessage(surface.SurfaceSessionID, "om-recovery-1", state.SurfaceMessageKindCard, "msg-1")
	svc.RecordPageTrackingMessage(surface.SurfaceSessionID, episode.EpisodeID, "om-recovery-1")
	if got := recoveryStatusMessageID(surface, episode); got != "om-recovery-1" {
		t.Fatalf("expected tail recovery card to remain patchable, got %q", got)
	}

	svc.RecordSurfaceOutboundMessage(surface.SurfaceSessionID, "om-next-1", state.SurfaceMessageKindText, "msg-1")
	event := svc.recoveryStatusCardEvent(surface, episode)
	if event.PageView == nil {
		t.Fatalf("expected recovery status page event, got %#v", event)
	}
	if event.PageView.MessageID != "" {
		t.Fatalf("expected recovery status card to stop patching after tail moves, got %#v", event.PageView)
	}
}

func TestStartupFailureDoesNotEnterRecovery(t *testing.T) {
	now := time.Date(2026, 4, 9, 12, 40, 0, 0, time.UTC)
	svc := newServiceForTest(&now)
	surface := setupAutoContinueSurface(t, svc)
	surface.Recovery.Enabled = true

	queued := svc.ApplySurfaceAction(control.Action{
		Kind:             control.ActionTextMessage,
		SurfaceSessionID: "surface-1",
		MessageID:        "msg-1",
		Text:             "继续处理",
	})
	if len(queued) == 0 {
		t.Fatal("expected initial dispatch")
	}

	events := svc.ApplyAgentEvent("inst-1", agentproto.Event{
		Kind:                 agentproto.EventTurnCompleted,
		ThreadID:             "thread-1",
		Status:               "failed",
		ErrorMessage:         "thread resume rejected",
		TurnCompletionOrigin: agentproto.TurnCompletionOriginThreadResumeRejected,
		Problem: &agentproto.ErrorInfo{
			Code:      "thread_resume_rejected",
			Layer:     "codex",
			Stage:     "thread_resume",
			Message:   "thread resume rejected",
			Retryable: true,
		},
	})
	if episode := surface.Recovery.Episode; episode != nil {
		t.Fatalf("expected startup failure to avoid recovery lane, got %#v", episode)
	}
	item := surface.QueueItems["queue-1"]
	if item == nil || item.Status != state.QueueItemFailed {
		t.Fatalf("expected startup failure to fail queue item, got %#v", item)
	}
	var sawTurnFailed bool
	for _, event := range events {
		if event.Notice != nil && event.Notice.Code == "turn_failed" {
			sawTurnFailed = true
		}
		if event.Command != nil && event.Command.Kind == agentproto.CommandPromptSend && len(event.Command.Prompt.Inputs) == 1 && event.Command.Prompt.Inputs[0].Text == recoveryContinuePromptText {
			t.Fatalf("expected startup failure not to dispatch recovery prompt, got %#v", events)
		}
	}
	if !sawTurnFailed {
		t.Fatalf("expected startup failure to emit explicit failure notice, got %#v", events)
	}
}

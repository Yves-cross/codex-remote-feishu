package orchestrator

import (
	"fmt"
	"strings"
	"time"

	"github.com/kxn/codex-remote-feishu/internal/core/agentproto"
	"github.com/kxn/codex-remote-feishu/internal/core/control"
	"github.com/kxn/codex-remote-feishu/internal/core/eventcontract"
	"github.com/kxn/codex-remote-feishu/internal/core/state"
)

const recoveryContinuePromptText = "上一次响应因上游推理中断，请从中断处继续完成当前任务；如果其实已经完成，请直接说明结果。"

func activeRecoveryEpisode(surface *state.SurfaceConsoleRecord) *state.PendingRecoveryEpisodeRecord {
	if surface == nil {
		return nil
	}
	return surface.Recovery.Episode
}

func clearRecoveryRuntime(surface *state.SurfaceConsoleRecord) {
	if surface == nil {
		return
	}
	enabled := surface.Recovery.Enabled
	surface.Recovery = state.RecoveryRuntimeRecord{Enabled: enabled}
}

func (s *Service) nextRecoveryEpisodeToken() string {
	s.nextRecoveryEpisodeID++
	return fmt.Sprintf("recovery-%d", s.nextRecoveryEpisodeID)
}

func recoveryBackoff(consecutiveDryFailures int) (time.Duration, int, bool) {
	delays := []time.Duration{0, 0, 2 * time.Second, 5 * time.Second, 10 * time.Second}
	if consecutiveDryFailures <= 0 || consecutiveDryFailures > len(delays) {
		return 0, len(delays), false
	}
	return delays[consecutiveDryFailures-1], len(delays), true
}

func (s *Service) recoveryDispatchReady(surface *state.SurfaceConsoleRecord) bool {
	if surface == nil || !surface.Recovery.Enabled {
		return false
	}
	if surface.AttachedInstanceID == "" || surface.Abandoning || surface.PendingHeadless != nil {
		return false
	}
	if surface.DispatchMode != state.DispatchModeNormal {
		return false
	}
	if surface.ActiveRequestCapture != nil || activePendingRequest(surface) != nil {
		return false
	}
	if s.progress.surfaceHasPendingCompact(surface) || s.surfaceHasPendingSteer(surface) {
		return false
	}
	return true
}

func recoveryDelayText(delay time.Duration) string {
	return formatAutoContinueDelay(delay)
}

func (s *Service) recoveryStatusCardEvent(surface *state.SurfaceConsoleRecord, episode *state.PendingRecoveryEpisodeRecord) eventcontract.Event {
	if surface == nil || episode == nil {
		return eventcontract.Event{}
	}
	title := "自动恢复"
	theme := "progress"
	lines := []string{}
	sealed := false
	switch episode.State {
	case state.RecoveryEpisodeScheduled:
		title = "等待自动恢复"
		if episode.PendingDueAt.IsZero() || !episode.PendingDueAt.After(time.Time{}) {
			lines = append(lines, fmt.Sprintf("上游推理中断，准备开始第 %d 次自动恢复。", episode.AttemptCount))
		} else {
			delay := episode.PendingDueAt.Sub(s.now())
			if delay < 0 {
				delay = 0
			}
			lines = append(lines, fmt.Sprintf("上游推理中断，计划在 %s 后开始第 %d 次自动恢复。", recoveryDelayText(delay), episode.AttemptCount))
		}
	case state.RecoveryEpisodeRunning:
		title = "正在自动恢复"
		lines = append(lines, fmt.Sprintf("上游推理中断，已开始第 %d 次自动恢复。", episode.AttemptCount))
	case state.RecoveryEpisodeCompleted:
		title = "自动恢复完成"
		theme = "success"
		sealed = true
		lines = append(lines, "当前自动恢复已完成。")
	case state.RecoveryEpisodeCancelled:
		title = "已停止自动恢复"
		theme = "info"
		sealed = true
		lines = append(lines, "当前自动恢复已停止。")
	case state.RecoveryEpisodeFailed:
		title = "自动恢复失败"
		theme = "error"
		sealed = true
		lines = append(lines, fmt.Sprintf("自动恢复已连续失败 %d 次，已停止继续重试。", episode.AttemptCount))
	default:
		lines = append(lines, "自动恢复状态已更新。")
	}
	if episode.LastProblem != nil && strings.TrimSpace(episode.LastProblem.Message) != "" {
		lines = append(lines, episode.LastProblem.Message)
	}
	view := control.NormalizeFeishuPageView(control.FeishuPageView{
		Title:       title,
		MessageID:   recoveryStatusMessageID(surface, episode),
		TrackingKey: strings.TrimSpace(episode.EpisodeID),
		ThemeKey:    theme,
		Patchable:   true,
		BodySections: []control.FeishuCardTextSection{{
			Lines: lines,
		}},
		Interactive: false,
		Sealed:      sealed,
	})
	return eventcontract.NewEventFromPayload(
		eventcontract.PagePayload{View: view},
		eventcontract.EventMeta{
			Target: eventcontract.TargetRef{
				GatewayID:        strings.TrimSpace(surface.GatewayID),
				SurfaceSessionID: strings.TrimSpace(surface.SurfaceSessionID),
			},
			SourceMessageID:      strings.TrimSpace(episode.RootReplyToMessageID),
			SourceMessagePreview: strings.TrimSpace(episode.RootReplyToMessagePreview),
			MessageDelivery:      patchTailReplyThreadMessageDelivery(),
		},
	)
}

func recoveryStatusMessageID(surface *state.SurfaceConsoleRecord, episode *state.PendingRecoveryEpisodeRecord) string {
	if !recoveryEpisodeCanPatchTail(surface, episode) {
		return ""
	}
	return strings.TrimSpace(episode.NoticeMessageID)
}

func (s *Service) maybeScheduleRecoveryAfterOutcome(outcome *remoteTurnOutcome) []eventcontract.Event {
	if outcome == nil || outcome.Surface == nil || outcome.Item == nil || outcome.Cause != terminalCauseUpstreamRetryableFailure {
		return nil
	}
	surface := outcome.Surface
	if !surface.Recovery.Enabled {
		return nil
	}
	episode := activeRecoveryEpisode(surface)
	continuing := false
	if episode != nil && strings.TrimSpace(outcome.Binding.RecoveryEpisodeID) != "" && strings.TrimSpace(episode.EpisodeID) == strings.TrimSpace(outcome.Binding.RecoveryEpisodeID) {
		continuing = true
	}
	if !continuing || episode == nil {
		episode = &state.PendingRecoveryEpisodeRecord{
			EpisodeID:                 s.nextRecoveryEpisodeToken(),
			InstanceID:                outcome.InstanceID,
			ThreadID:                  strings.TrimSpace(firstNonEmpty(outcome.ThreadID, outcome.Item.FrozenThreadID)),
			FrozenCWD:                 strings.TrimSpace(firstNonEmpty(outcome.Binding.ThreadCWD, outcome.Item.FrozenCWD)),
			FrozenRouteMode:           outcome.Item.RouteModeAtEnqueue,
			FrozenOverride:            outcome.Item.FrozenOverride,
			FrozenPlanMode:            outcome.Item.FrozenPlanMode,
			RootReplyToMessageID:      strings.TrimSpace(firstNonEmpty(outcome.Binding.ReplyToMessageID, outcome.Item.ReplyToMessageID, outcome.Item.SourceMessageID)),
			RootReplyToMessagePreview: strings.TrimSpace(firstNonEmpty(outcome.Binding.ReplyToMessagePreview, outcome.Item.ReplyToMessagePreview, outcome.Item.SourceMessagePreview)),
			TriggerKind:               state.RecoveryTriggerKindUpstreamRetryableFailure,
		}
		surface.Recovery.Episode = episode
	}
	episode.LastTurnID = strings.TrimSpace(outcome.TurnID)
	episode.LastProblem = cloneProblem(outcome.Problem)
	episode.ThreadID = strings.TrimSpace(firstNonEmpty(outcome.ThreadID, episode.ThreadID))
	if strings.TrimSpace(outcome.Binding.ThreadCWD) != "" {
		episode.FrozenCWD = strings.TrimSpace(outcome.Binding.ThreadCWD)
	}
	if strings.TrimSpace(outcome.Binding.ReplyToMessageID) != "" {
		episode.RootReplyToMessageID = strings.TrimSpace(outcome.Binding.ReplyToMessageID)
		episode.RootReplyToMessagePreview = strings.TrimSpace(outcome.Binding.ReplyToMessagePreview)
	}
	dryFailures := 1
	if continuing && !outcome.AnyOutputSeen {
		dryFailures = episode.ConsecutiveDryFailureCount + 1
	}
	episode.ConsecutiveDryFailureCount = dryFailures
	delay, _, ok := recoveryBackoff(dryFailures)
	if !ok {
		if outcome.AnyOutputSeen {
			episode.NoticeMessageID = ""
			episode.NoticeAppendSeq = 0
		}
		episode.State = state.RecoveryEpisodeFailed
		episode.PendingDueAt = time.Time{}
		return []eventcontract.Event{s.recoveryFailureEvent(surface, episode)}
	}
	if outcome.AnyOutputSeen {
		episode.NoticeMessageID = ""
		episode.NoticeAppendSeq = 0
	}
	episode.AttemptCount++
	episode.CurrentAttemptOutputSeen = false
	episode.PendingDueAt = s.now().Add(delay)
	episode.State = state.RecoveryEpisodeScheduled
	return []eventcontract.Event{s.recoveryStatusCardEvent(surface, surface.Recovery.Episode)}
}

func (s *Service) recoveryFailureEvent(surface *state.SurfaceConsoleRecord, episode *state.PendingRecoveryEpisodeRecord) eventcontract.Event {
	return s.recoveryStatusCardEvent(surface, episode)
}

func (s *Service) maybeDispatchPendingRecovery(surface *state.SurfaceConsoleRecord, now time.Time) []eventcontract.Event {
	episode := activeRecoveryEpisode(surface)
	if episode == nil || !surface.Recovery.Enabled || episode.State != state.RecoveryEpisodeScheduled {
		return nil
	}
	if strings.TrimSpace(surface.AttachedInstanceID) == "" || strings.TrimSpace(surface.AttachedInstanceID) != strings.TrimSpace(episode.InstanceID) {
		clearRecoveryRuntime(surface)
		return nil
	}
	switch episode.FrozenRouteMode {
	case state.RouteModeNewThreadReady:
		if surface.RouteMode != state.RouteModeNewThreadReady || strings.TrimSpace(surface.PreparedThreadCWD) != strings.TrimSpace(episode.FrozenCWD) {
			clearRecoveryRuntime(surface)
			return nil
		}
	default:
		if strings.TrimSpace(episode.ThreadID) != "" && strings.TrimSpace(surface.SelectedThreadID) != strings.TrimSpace(episode.ThreadID) {
			clearRecoveryRuntime(surface)
			return nil
		}
	}
	if !episode.PendingDueAt.IsZero() && now.Before(episode.PendingDueAt) {
		return nil
	}
	if !s.recoveryDispatchReady(surface) {
		return nil
	}
	inst := s.root.Instances[surface.AttachedInstanceID]
	if inst == nil || !inst.Online || inst.ActiveTurnID != "" || s.turns.pendingRemote[inst.InstanceID] != nil || surface.ActiveQueueItemID != "" {
		return nil
	}
	return s.dispatchRecoveryEpisode(surface, inst, episode)
}

func (s *Service) dispatchRecoveryEpisode(surface *state.SurfaceConsoleRecord, inst *state.InstanceRecord, episode *state.PendingRecoveryEpisodeRecord) []eventcontract.Event {
	if surface == nil || inst == nil || episode == nil {
		return nil
	}
	s.nextQueueItemID++
	itemID := fmt.Sprintf("queue-%d", s.nextQueueItemID)
	item := &state.QueueItemRecord{
		ID:                    itemID,
		SurfaceSessionID:      surface.SurfaceSessionID,
		SourceKind:            state.QueueItemSourceRecovery,
		RecoveryEpisodeID:     episode.EpisodeID,
		ReplyToMessageID:      episode.RootReplyToMessageID,
		ReplyToMessagePreview: episode.RootReplyToMessagePreview,
		Inputs:                []agentproto.Input{{Type: agentproto.InputText, Text: recoveryContinuePromptText}},
		FrozenThreadID:        episode.ThreadID,
		FrozenCWD:             episode.FrozenCWD,
		FrozenOverride:        episode.FrozenOverride,
		FrozenPlanMode:        episode.FrozenPlanMode,
		RouteModeAtEnqueue:    episode.FrozenRouteMode,
		Status:                state.QueueItemDispatching,
	}
	surface.QueueItems[item.ID] = item
	surface.ActiveQueueItemID = item.ID
	s.turns.pendingRemote[inst.InstanceID] = &remoteTurnBinding{
		InstanceID:            inst.InstanceID,
		SurfaceSessionID:      surface.SurfaceSessionID,
		QueueItemID:           item.ID,
		RecoveryEpisodeID:     episode.EpisodeID,
		AttemptTriggerKind:    string(episode.TriggerKind),
		ReplyToMessageID:      episode.RootReplyToMessageID,
		ReplyToMessagePreview: episode.RootReplyToMessagePreview,
		ThreadID:              item.FrozenThreadID,
		ThreadCWD:             item.FrozenCWD,
		Status:                string(item.Status),
	}
	episode.State = state.RecoveryEpisodeRunning
	episode.PendingDueAt = time.Time{}
	episode.CurrentAttemptOutputSeen = false
	command := &agentproto.Command{
		Kind: agentproto.CommandPromptSend,
		Origin: agentproto.Origin{
			Surface:   surface.SurfaceSessionID,
			UserID:    surface.ActorUserID,
			ChatID:    surface.ChatID,
			MessageID: episode.RootReplyToMessageID,
		},
		Target: agentproto.Target{
			ThreadID:              item.FrozenThreadID,
			CWD:                   item.FrozenCWD,
			CreateThreadIfMissing: item.FrozenThreadID == "",
		},
		Prompt: agentproto.Prompt{
			Inputs: item.Inputs,
		},
		Overrides: agentproto.PromptOverrides{
			Model:           item.FrozenOverride.Model,
			ReasoningEffort: item.FrozenOverride.ReasoningEffort,
			AccessMode:      item.FrozenOverride.AccessMode,
			PlanMode:        string(state.NormalizePlanModeSetting(item.FrozenPlanMode)),
		},
	}
	return []eventcontract.Event{
		s.recoveryStatusCardEvent(surface, episode),
		{
			Kind:             eventcontract.KindAgentCommand,
			SurfaceSessionID: surface.SurfaceSessionID,
			Command:          command,
		},
	}
}

func (s *Service) finishRecoveryEpisode(outcome *remoteTurnOutcome) {
	if outcome == nil || outcome.Surface == nil {
		return
	}
	episode := activeRecoveryEpisode(outcome.Surface)
	if episode == nil {
		return
	}
	if strings.TrimSpace(outcome.Binding.RecoveryEpisodeID) == "" || strings.TrimSpace(episode.EpisodeID) != strings.TrimSpace(outcome.Binding.RecoveryEpisodeID) {
		return
	}
	if outcome.Cause == terminalCauseCompleted {
		surface := outcome.Surface
		surface.Recovery.Episode = nil
	}
}

func (s *Service) cancelRecoveryEpisode(surface *state.SurfaceConsoleRecord) []eventcontract.Event {
	episode := activeRecoveryEpisode(surface)
	if surface == nil || episode == nil {
		return nil
	}
	episode.State = state.RecoveryEpisodeCancelled
	episode.PendingDueAt = time.Time{}
	episode.CurrentAttemptOutputSeen = false
	return []eventcontract.Event{s.recoveryStatusCardEvent(surface, episode)}
}

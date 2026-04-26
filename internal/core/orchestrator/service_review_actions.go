package orchestrator

import (
	"strings"

	"github.com/kxn/codex-remote-feishu/internal/core/agentproto"
	"github.com/kxn/codex-remote-feishu/internal/core/control"
	"github.com/kxn/codex-remote-feishu/internal/core/eventcontract"
	"github.com/kxn/codex-remote-feishu/internal/core/state"
)

const reviewApplyPromptPrefix = "请根据以下审阅意见继续修改：\n\n"

func (s *Service) startReviewFromFinalCard(surface *state.SurfaceConsoleRecord, action control.Action) []eventcontract.Event {
	if surface == nil {
		return nil
	}
	if active := s.activeReviewSession(surface); active != nil {
		return notice(surface, "review_session_active", "当前已经在审阅中；请直接继续提问，或使用“放弃审阅”/“按审阅意见继续修改”退出。")
	}
	if strings.TrimSpace(surface.AttachedInstanceID) == "" {
		return notice(surface, "review_not_attached", s.notAttachedText(surface))
	}
	lifecycleID := ""
	if action.Inbound != nil {
		lifecycleID = action.Inbound.CardDaemonLifecycleID
	}
	record := s.LookupFinalCardByMessageID(surface.SurfaceSessionID, action.MessageID, lifecycleID)
	if record == nil {
		return notice(surface, "review_source_not_found", "当前结果卡片已经不可用，请重新获取最新结果后再进入审阅。")
	}
	if strings.TrimSpace(record.InstanceID) != strings.TrimSpace(surface.AttachedInstanceID) {
		return notice(surface, "review_source_instance_changed", "当前已经切换到其他实例，请重新获取这条结果对应的最新卡片后再进入审阅。")
	}
	parentThreadID := strings.TrimSpace(record.ThreadID)
	if parentThreadID == "" {
		return notice(surface, "review_source_thread_missing", "当前结果缺少可审阅的线程上下文，请重新获取最新结果后再试。")
	}
	inst := s.root.Instances[strings.TrimSpace(surface.AttachedInstanceID)]
	if inst == nil {
		return notice(surface, "review_instance_missing", "当前实例已经不可用，请稍后重试。")
	}
	parentThread := inst.Threads[parentThreadID]
	if parentThread == nil {
		return notice(surface, "review_parent_thread_missing", "当前结果对应的会话已不可用，请重新获取最新结果后再试。")
	}
	surface.ReviewSession = &state.ReviewSessionRecord{
		Phase:           state.ReviewSessionPhasePending,
		ParentThreadID:  parentThreadID,
		ThreadCWD:       firstNonEmpty(strings.TrimSpace(parentThread.CWD), strings.TrimSpace(inst.WorkspaceRoot)),
		SourceMessageID: strings.TrimSpace(action.MessageID),
		StartedAt:       s.now(),
		LastUpdatedAt:   s.now(),
	}
	return []eventcontract.Event{
		{
			Kind:             eventcontract.KindNotice,
			SurfaceSessionID: surface.SurfaceSessionID,
			Notice: &control.Notice{
				Code:     "review_start_requested",
				Title:    "正在进入审阅",
				Text:     "正在为这条结果创建独立审阅会话。",
				ThemeKey: "system",
			},
		},
		{
			Kind:             eventcontract.KindAgentCommand,
			SurfaceSessionID: surface.SurfaceSessionID,
			Command: &agentproto.Command{
				Kind: agentproto.CommandReviewStart,
				Origin: agentproto.Origin{
					Surface:   surface.SurfaceSessionID,
					UserID:    surface.ActorUserID,
					ChatID:    surface.ChatID,
					MessageID: strings.TrimSpace(action.MessageID),
				},
				Target: agentproto.Target{
					ThreadID: parentThreadID,
				},
				Review: agentproto.ReviewRequest{
					Delivery: agentproto.ReviewDeliveryDetached,
					Target: agentproto.ReviewTarget{
						Kind: agentproto.ReviewTargetKindUncommittedChanges,
					},
				},
			},
		},
	}
}

func (s *Service) discardReviewSession(surface *state.SurfaceConsoleRecord) []eventcontract.Event {
	if surface == nil || s.activeReviewSession(surface) == nil {
		return notice(surface, "review_session_inactive", "当前没有进行中的审阅会话。")
	}
	surface.ReviewSession = nil
	return []eventcontract.Event{{
		Kind:             eventcontract.KindNotice,
		SurfaceSessionID: surface.SurfaceSessionID,
		Notice: &control.Notice{
			Code:     "review_discarded",
			Title:    "已放弃审阅",
			Text:     "已退出当前审阅会话。",
			ThemeKey: "system",
		},
	}}
}

func (s *Service) applyReviewSessionResult(surface *state.SurfaceConsoleRecord, action control.Action) []eventcontract.Event {
	if surface == nil {
		return nil
	}
	session := s.activeReviewSession(surface)
	if session == nil {
		return notice(surface, "review_session_inactive", "当前没有进行中的审阅会话。")
	}
	parentThreadID := strings.TrimSpace(session.ParentThreadID)
	reviewText := strings.TrimSpace(session.LastReviewText)
	if parentThreadID == "" {
		return notice(surface, "review_parent_thread_missing", "当前审阅会话缺少原始会话上下文，请重新进入审阅后再试。")
	}
	if reviewText == "" {
		return notice(surface, "review_result_not_ready", "当前审阅结果尚未就绪，请等本轮审阅完成后再继续修改。")
	}
	inst := s.root.Instances[strings.TrimSpace(surface.AttachedInstanceID)]
	cwd := reviewSessionCWD(inst, session)
	if strings.TrimSpace(cwd) == "" {
		return notice(surface, "review_parent_cwd_missing", "当前无法恢复原始会话的工作目录，请重新选择会话后再继续修改。")
	}
	promptText := reviewApplyPromptPrefix + reviewText
	sourceMessageID := firstNonEmpty(strings.TrimSpace(action.MessageID), strings.TrimSpace(session.SourceMessageID))
	surface.ReviewSession = nil
	return []eventcontract.Event{
		{
			Kind:             eventcontract.KindNotice,
			SurfaceSessionID: surface.SurfaceSessionID,
			Notice: &control.Notice{
				Code:     "review_apply_requested",
				Title:    "正在继续修改",
				Text:     "已退出审阅，正在把审阅意见带回原会话继续修改。",
				ThemeKey: "system",
			},
		},
		{
			Kind:             eventcontract.KindAgentCommand,
			SurfaceSessionID: surface.SurfaceSessionID,
			Command: &agentproto.Command{
				Kind: agentproto.CommandPromptSend,
				Origin: agentproto.Origin{
					Surface:   surface.SurfaceSessionID,
					UserID:    surface.ActorUserID,
					ChatID:    surface.ChatID,
					MessageID: sourceMessageID,
				},
				Target: agentproto.Target{
					ExecutionMode:        agentproto.PromptExecutionModeResumeExisting,
					ThreadID:             parentThreadID,
					CWD:                  cwd,
					SurfaceBindingPolicy: agentproto.SurfaceBindingPolicyKeepSurfaceSelection,
				},
				Prompt: agentproto.Prompt{
					Inputs: []agentproto.Input{{
						Type: agentproto.InputText,
						Text: promptText,
					}},
				},
				Overrides: agentproto.PromptOverrides{
					Model:           surface.PromptOverride.Model,
					ReasoningEffort: surface.PromptOverride.ReasoningEffort,
					AccessMode:      surface.PromptOverride.AccessMode,
					PlanMode:        string(state.NormalizePlanModeSetting(surface.PlanMode)),
				},
			},
		},
	}
}

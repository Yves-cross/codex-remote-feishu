package eventcontractcompat

import (
	"strings"

	"github.com/kxn/codex-remote-feishu/internal/core/control"
	"github.com/kxn/codex-remote-feishu/internal/core/eventcontract"
)

func FromLegacyUIEvents(events []control.UIEvent) []eventcontract.Event {
	if len(events) == 0 {
		return nil
	}
	converted := make([]eventcontract.Event, 0, len(events))
	for _, event := range events {
		converted = append(converted, FromLegacyUIEvent(event))
	}
	return converted
}

func ToLegacyUIEvents(events []eventcontract.Event) []control.UIEvent {
	if len(events) == 0 {
		return nil
	}
	converted := make([]control.UIEvent, 0, len(events))
	for _, event := range events {
		converted = append(converted, ToLegacyUIEvent(event))
	}
	return converted
}

func FromLegacyUIEvent(event control.UIEvent) eventcontract.Event {
	return eventcontract.Event{
		Meta: eventcontract.EventMeta{
			Target:               eventcontract.ExplicitTarget(event.GatewayID, event.SurfaceSessionID),
			SourceMessageID:      event.SourceMessageID,
			SourceMessagePreview: event.SourceMessagePreview,
			DaemonLifecycleID:    event.DaemonLifecycleID,
			InlineReplaceMode:    legacyInlineReplaceMode(event),
			Semantics:            legacyDeliverySemantics(event),
		},
		Payload: payloadFromLegacyUIEvent(event),
	}.Normalized()
}

func ToLegacyUIEvent(event eventcontract.Event) control.UIEvent {
	event = event.Normalized()
	legacy := control.UIEvent{
		GatewayID:                event.GatewayID(),
		SurfaceSessionID:         event.SurfaceSessionID(),
		DaemonLifecycleID:        event.Meta.DaemonLifecycleID,
		SourceMessageID:          event.Meta.SourceMessageID,
		SourceMessagePreview:     event.Meta.SourceMessagePreview,
		InlineReplaceCurrentCard: event.Meta.InlineReplaceMode == eventcontract.InlineReplaceCurrentCard,
	}
	switch payload := event.Payload.(type) {
	case eventcontract.SnapshotPayload:
		legacy.Kind = control.UIEventSnapshot
		snapshot := payload.Snapshot
		legacy.Snapshot = &snapshot
	case eventcontract.SelectionPayload:
		legacy.Kind = control.UIEventFeishuSelectionView
		view := payload.View
		legacy.FeishuSelectionView = &view
		legacy.FeishuSelectionContext = cloneSelectionContext(payload.Context)
	case eventcontract.PagePayload:
		legacy.Kind = control.UIEventFeishuPageView
		view := payload.View
		legacy.FeishuPageView = &view
		legacy.FeishuPageContext = clonePageContext(payload.Context)
	case eventcontract.RequestPayload:
		legacy.Kind = control.UIEventFeishuRequestView
		view := payload.View
		legacy.FeishuRequestView = &view
		legacy.FeishuRequestContext = cloneRequestContext(payload.Context)
	case eventcontract.PathPickerPayload:
		legacy.Kind = control.UIEventFeishuPathPicker
		view := payload.View
		legacy.FeishuPathPickerView = &view
		legacy.FeishuPathPickerContext = clonePathPickerContext(payload.Context)
	case eventcontract.TargetPickerPayload:
		legacy.Kind = control.UIEventFeishuTargetPicker
		view := payload.View
		legacy.FeishuTargetPickerView = &view
		legacy.FeishuTargetPickerContext = cloneTargetPickerContext(payload.Context)
	case eventcontract.ThreadHistoryPayload:
		legacy.Kind = control.UIEventFeishuThreadHistory
		view := payload.View
		legacy.FeishuThreadHistoryView = &view
		legacy.FeishuThreadHistoryContext = cloneThreadHistoryContext(payload.Context)
	case eventcontract.PendingInputPayload:
		legacy.Kind = control.UIEventPendingInput
		state := payload.State
		legacy.PendingInput = &state
	case eventcontract.NoticePayload:
		legacy.Kind = control.UIEventNotice
		notice := payload.Notice
		legacy.Notice = &notice
		legacy.ThreadSelection = cloneThreadSelection(payload.ThreadSelection)
	case eventcontract.PlanUpdatePayload:
		legacy.Kind = control.UIEventPlanUpdated
		update := payload.PlanUpdate
		legacy.PlanUpdate = &update
	case eventcontract.BlockCommittedPayload:
		legacy.Kind = control.UIEventBlockCommitted
		block := payload.Block
		legacy.Block = &block
		legacy.FileChangeSummary = cloneFileChangeSummary(payload.FileChangeSummary)
		legacy.TurnDiffSnapshot = cloneTurnDiffSnapshot(payload.TurnDiffSnapshot)
		legacy.FinalTurnSummary = cloneFinalTurnSummary(payload.FinalTurnSummary)
	case eventcontract.TimelineTextPayload:
		legacy.Kind = control.UIEventTimelineText
		timeline := payload.TimelineText
		legacy.TimelineText = &timeline
	case eventcontract.ImageOutputPayload:
		legacy.Kind = control.UIEventImageOutput
		output := payload.ImageOutput
		legacy.ImageOutput = &output
	case eventcontract.ExecCommandProgressPayload:
		legacy.Kind = control.UIEventExecCommandProgress
		progress := payload.Progress
		legacy.ExecCommandProgress = &progress
	case eventcontract.AgentCommandPayload:
		legacy.Kind = control.UIEventAgentCommand
		command := payload.Command
		legacy.Command = &command
	case eventcontract.DaemonCommandPayload:
		legacy.Kind = control.UIEventDaemonCommand
		command := payload.Command
		legacy.DaemonCommand = &command
	}
	return legacy
}

func KindFromLegacyUIEvent(event control.UIEvent) eventcontract.Kind {
	switch {
	case event.Command != nil:
		return eventcontract.KindAgentCommand
	case event.DaemonCommand != nil:
		return eventcontract.KindDaemonCommand
	case event.Block != nil:
		return eventcontract.KindBlockCommitted
	case event.ExecCommandProgress != nil:
		return eventcontract.KindExecCommandProgress
	case event.ImageOutput != nil:
		return eventcontract.KindImageOutput
	case event.TimelineText != nil:
		return eventcontract.KindTimelineText
	case event.PlanUpdate != nil:
		return eventcontract.KindPlanUpdate
	case event.Notice != nil:
		return eventcontract.KindNotice
	case event.PendingInput != nil:
		return eventcontract.KindPendingInput
	case event.FeishuThreadHistoryView != nil:
		return eventcontract.KindThreadHistory
	case event.FeishuTargetPickerView != nil:
		return eventcontract.KindTargetPicker
	case event.FeishuPathPickerView != nil:
		return eventcontract.KindPathPicker
	case event.FeishuRequestView != nil:
		return eventcontract.KindRequest
	case event.FeishuPageView != nil:
		return eventcontract.KindPage
	case event.FeishuSelectionView != nil:
		return eventcontract.KindSelection
	case event.Snapshot != nil:
		return eventcontract.KindSnapshot
	}
	switch event.Kind {
	case control.UIEventSnapshot:
		return eventcontract.KindSnapshot
	case control.UIEventFeishuSelectionView:
		return eventcontract.KindSelection
	case control.UIEventFeishuPageView:
		return eventcontract.KindPage
	case control.UIEventFeishuRequestView:
		return eventcontract.KindRequest
	case control.UIEventFeishuPathPicker:
		return eventcontract.KindPathPicker
	case control.UIEventFeishuTargetPicker:
		return eventcontract.KindTargetPicker
	case control.UIEventFeishuThreadHistory:
		return eventcontract.KindThreadHistory
	case control.UIEventPendingInput:
		return eventcontract.KindPendingInput
	case control.UIEventNotice:
		return eventcontract.KindNotice
	case control.UIEventPlanUpdated:
		return eventcontract.KindPlanUpdate
	case control.UIEventBlockCommitted:
		return eventcontract.KindBlockCommitted
	case control.UIEventTimelineText:
		return eventcontract.KindTimelineText
	case control.UIEventImageOutput:
		return eventcontract.KindImageOutput
	case control.UIEventExecCommandProgress:
		return eventcontract.KindExecCommandProgress
	case control.UIEventAgentCommand:
		return eventcontract.KindAgentCommand
	case control.UIEventDaemonCommand:
		return eventcontract.KindDaemonCommand
	default:
		return eventcontract.KindUnknown
	}
}

func payloadFromLegacyUIEvent(event control.UIEvent) eventcontract.Payload {
	switch KindFromLegacyUIEvent(event) {
	case eventcontract.KindSnapshot:
		if event.Snapshot != nil {
			return eventcontract.SnapshotPayload{Snapshot: *event.Snapshot}
		}
		return eventcontract.SnapshotPayload{}
	case eventcontract.KindSelection:
		if event.FeishuSelectionView != nil {
			return eventcontract.SelectionPayload{
				View:    *event.FeishuSelectionView,
				Context: cloneSelectionContext(event.FeishuSelectionContext),
			}
		}
		return eventcontract.SelectionPayload{Context: cloneSelectionContext(event.FeishuSelectionContext)}
	case eventcontract.KindPage:
		if event.FeishuPageView != nil {
			return eventcontract.PagePayload{
				View:    *event.FeishuPageView,
				Context: clonePageContext(event.FeishuPageContext),
			}
		}
		return eventcontract.PagePayload{Context: clonePageContext(event.FeishuPageContext)}
	case eventcontract.KindRequest:
		if event.FeishuRequestView != nil {
			return eventcontract.RequestPayload{
				View:    *event.FeishuRequestView,
				Context: cloneRequestContext(event.FeishuRequestContext),
			}
		}
		return eventcontract.RequestPayload{Context: cloneRequestContext(event.FeishuRequestContext)}
	case eventcontract.KindPathPicker:
		if event.FeishuPathPickerView != nil {
			return eventcontract.PathPickerPayload{
				View:    *event.FeishuPathPickerView,
				Context: clonePathPickerContext(event.FeishuPathPickerContext),
			}
		}
		return eventcontract.PathPickerPayload{Context: clonePathPickerContext(event.FeishuPathPickerContext)}
	case eventcontract.KindTargetPicker:
		if event.FeishuTargetPickerView != nil {
			return eventcontract.TargetPickerPayload{
				View:    *event.FeishuTargetPickerView,
				Context: cloneTargetPickerContext(event.FeishuTargetPickerContext),
			}
		}
		return eventcontract.TargetPickerPayload{Context: cloneTargetPickerContext(event.FeishuTargetPickerContext)}
	case eventcontract.KindThreadHistory:
		if event.FeishuThreadHistoryView != nil {
			return eventcontract.ThreadHistoryPayload{
				View:    *event.FeishuThreadHistoryView,
				Context: cloneThreadHistoryContext(event.FeishuThreadHistoryContext),
			}
		}
		return eventcontract.ThreadHistoryPayload{Context: cloneThreadHistoryContext(event.FeishuThreadHistoryContext)}
	case eventcontract.KindPendingInput:
		if event.PendingInput != nil {
			return eventcontract.PendingInputPayload{State: *event.PendingInput}
		}
		return eventcontract.PendingInputPayload{}
	case eventcontract.KindNotice:
		payload := eventcontract.NoticePayload{
			ThreadSelection: cloneThreadSelection(event.ThreadSelection),
		}
		if event.Notice != nil {
			payload.Notice = *event.Notice
		}
		return payload
	case eventcontract.KindPlanUpdate:
		if event.PlanUpdate != nil {
			return eventcontract.PlanUpdatePayload{PlanUpdate: *event.PlanUpdate}
		}
		return eventcontract.PlanUpdatePayload{}
	case eventcontract.KindBlockCommitted:
		payload := eventcontract.BlockCommittedPayload{
			FileChangeSummary: cloneFileChangeSummary(event.FileChangeSummary),
			TurnDiffSnapshot:  cloneTurnDiffSnapshot(event.TurnDiffSnapshot),
			FinalTurnSummary:  cloneFinalTurnSummary(event.FinalTurnSummary),
		}
		if event.Block != nil {
			payload.Block = *event.Block
		}
		return payload
	case eventcontract.KindTimelineText:
		if event.TimelineText != nil {
			return eventcontract.TimelineTextPayload{TimelineText: *event.TimelineText}
		}
		return eventcontract.TimelineTextPayload{}
	case eventcontract.KindImageOutput:
		if event.ImageOutput != nil {
			return eventcontract.ImageOutputPayload{ImageOutput: *event.ImageOutput}
		}
		return eventcontract.ImageOutputPayload{}
	case eventcontract.KindExecCommandProgress:
		if event.ExecCommandProgress != nil {
			return eventcontract.ExecCommandProgressPayload{Progress: *event.ExecCommandProgress}
		}
		return eventcontract.ExecCommandProgressPayload{}
	case eventcontract.KindAgentCommand:
		if event.Command != nil {
			return eventcontract.AgentCommandPayload{Command: *event.Command}
		}
		return eventcontract.AgentCommandPayload{}
	case eventcontract.KindDaemonCommand:
		if event.DaemonCommand != nil {
			return eventcontract.DaemonCommandPayload{Command: *event.DaemonCommand}
		}
		return eventcontract.DaemonCommandPayload{}
	default:
		return nil
	}
}

func legacyInlineReplaceMode(event control.UIEvent) eventcontract.InlineReplaceMode {
	if event.InlineReplaceCurrentCard {
		return eventcontract.InlineReplaceCurrentCard
	}
	return eventcontract.InlineReplaceNone
}

func legacyDeliverySemantics(event control.UIEvent) eventcontract.DeliverySemantics {
	kind := KindFromLegacyUIEvent(event)
	semantics := eventcontract.DeliverySemantics{
		VisibilityClass:        legacyVisibilityClass(event, kind),
		HandoffClass:           legacyHandoffClass(event, kind),
		FirstResultDisposition: eventcontract.FirstResultDispositionKeep,
		OwnerCardDisposition:   eventcontract.OwnerCardDispositionKeep,
	}
	switch semantics.HandoffClass {
	case eventcontract.HandoffClassNotice, eventcontract.HandoffClassThreadSelection:
		semantics.FirstResultDisposition = eventcontract.FirstResultDispositionDrop
		semantics.OwnerCardDisposition = eventcontract.OwnerCardDispositionDrop
	}
	return semantics.Normalized()
}

func legacyVisibilityClass(event control.UIEvent, kind eventcontract.Kind) eventcontract.VisibilityClass {
	switch kind {
	case eventcontract.KindPlanUpdate:
		return eventcontract.VisibilityClassPlan
	case eventcontract.KindExecCommandProgress:
		return eventcontract.VisibilityClassProgressText
	case eventcontract.KindBlockCommitted:
		if event.Block != nil && event.Block.Final {
			return eventcontract.VisibilityClassAlwaysVisible
		}
		return eventcontract.VisibilityClassProgressText
	case eventcontract.KindTimelineText, eventcontract.KindRequest, eventcontract.KindImageOutput:
		return eventcontract.VisibilityClassAlwaysVisible
	case eventcontract.KindNotice:
		if event.Notice != nil && noticeIsAlwaysVisible(*event.Notice) {
			return eventcontract.VisibilityClassAlwaysVisible
		}
		return eventcontract.VisibilityClassUINavigation
	case eventcontract.KindSnapshot,
		eventcontract.KindSelection,
		eventcontract.KindPage,
		eventcontract.KindPathPicker,
		eventcontract.KindTargetPicker,
		eventcontract.KindThreadHistory,
		eventcontract.KindPendingInput:
		return eventcontract.VisibilityClassUINavigation
	default:
		return eventcontract.VisibilityClassDefault
	}
}

func legacyHandoffClass(event control.UIEvent, kind eventcontract.Kind) eventcontract.HandoffClass {
	switch kind {
	case eventcontract.KindNotice:
		if event.ThreadSelection != nil {
			return eventcontract.HandoffClassThreadSelection
		}
		return eventcontract.HandoffClassNotice
	case eventcontract.KindSnapshot,
		eventcontract.KindSelection,
		eventcontract.KindPage,
		eventcontract.KindPathPicker,
		eventcontract.KindTargetPicker,
		eventcontract.KindThreadHistory,
		eventcontract.KindPendingInput:
		return eventcontract.HandoffClassNavigation
	case eventcontract.KindExecCommandProgress, eventcontract.KindPlanUpdate:
		return eventcontract.HandoffClassProcessDetail
	case eventcontract.KindBlockCommitted:
		if event.Block != nil && !event.Block.Final {
			return eventcontract.HandoffClassProcessDetail
		}
		return eventcontract.HandoffClassTerminalContent
	case eventcontract.KindTimelineText, eventcontract.KindRequest, eventcontract.KindImageOutput:
		return eventcontract.HandoffClassTerminalContent
	default:
		return eventcontract.HandoffClassDefault
	}
}

func noticeIsAlwaysVisible(notice control.Notice) bool {
	theme := strings.ToLower(strings.TrimSpace(notice.ThemeKey))
	code := strings.ToLower(strings.TrimSpace(notice.Code))
	title := strings.TrimSpace(notice.Title)
	text := strings.TrimSpace(notice.Text)
	switch {
	case theme == "error" || strings.Contains(theme, "error") || strings.Contains(theme, "fail"):
		return true
	case strings.Contains(code, "error"), strings.Contains(code, "failed"), strings.Contains(code, "rejected"), strings.Contains(code, "offline"), strings.Contains(code, "expired"), strings.Contains(code, "invalid"):
		return true
	case strings.Contains(title, "错误"), strings.Contains(title, "失败"), strings.Contains(title, "无法"), strings.Contains(title, "拒绝"), strings.Contains(title, "离线"), strings.Contains(title, "过期"), strings.Contains(title, "失效"):
		return true
	case strings.Contains(text, "链路错误"), strings.Contains(text, "创建失败"), strings.Contains(text, "连接失败"):
		return true
	default:
		return false
	}
}

func cloneSelectionContext(context *control.FeishuUISelectionContext) *control.FeishuUISelectionContext {
	if context == nil {
		return nil
	}
	cloned := *context
	return &cloned
}

func clonePageContext(context *control.FeishuUIPageContext) *control.FeishuUIPageContext {
	if context == nil {
		return nil
	}
	cloned := *context
	return &cloned
}

func cloneRequestContext(context *control.FeishuUIRequestContext) *control.FeishuUIRequestContext {
	if context == nil {
		return nil
	}
	cloned := *context
	return &cloned
}

func clonePathPickerContext(context *control.FeishuUIPathPickerContext) *control.FeishuUIPathPickerContext {
	if context == nil {
		return nil
	}
	cloned := *context
	return &cloned
}

func cloneTargetPickerContext(context *control.FeishuUITargetPickerContext) *control.FeishuUITargetPickerContext {
	if context == nil {
		return nil
	}
	cloned := *context
	return &cloned
}

func cloneThreadHistoryContext(context *control.FeishuUIThreadHistoryContext) *control.FeishuUIThreadHistoryContext {
	if context == nil {
		return nil
	}
	cloned := *context
	return &cloned
}

func cloneThreadSelection(selection *control.ThreadSelectionChanged) *control.ThreadSelectionChanged {
	if selection == nil {
		return nil
	}
	cloned := *selection
	return &cloned
}

func cloneFileChangeSummary(summary *control.FileChangeSummary) *control.FileChangeSummary {
	if summary == nil {
		return nil
	}
	cloned := *summary
	if len(summary.Files) != 0 {
		cloned.Files = append([]control.FileChangeSummaryEntry(nil), summary.Files...)
	}
	return &cloned
}

func cloneTurnDiffSnapshot(snapshot *control.TurnDiffSnapshot) *control.TurnDiffSnapshot {
	if snapshot == nil {
		return nil
	}
	cloned := *snapshot
	return &cloned
}

func cloneFinalTurnSummary(summary *control.FinalTurnSummary) *control.FinalTurnSummary {
	if summary == nil {
		return nil
	}
	cloned := *summary
	return &cloned
}

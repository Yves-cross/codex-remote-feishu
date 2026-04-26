package orchestrator

import (
	"fmt"
	"strings"

	"github.com/kxn/codex-remote-feishu/internal/core/agentproto"
	"github.com/kxn/codex-remote-feishu/internal/core/control"
	"github.com/kxn/codex-remote-feishu/internal/core/eventcontract"
	"github.com/kxn/codex-remote-feishu/internal/core/state"
)

func (s *Service) targetPickerThreadHistoryReadCommand(surface *state.SurfaceConsoleRecord, record *activeTargetPickerRecord) *eventcontract.Event {
	if surface == nil || record == nil {
		return nil
	}
	kind, threadID := parseTargetPickerSessionValue(record.SelectedSessionValue)
	if kind != control.FeishuTargetPickerSessionThread || strings.TrimSpace(threadID) == "" {
		record.HistoryLoadingThreadID = ""
		return nil
	}
	workspaceKey := normalizeWorkspaceClaimKey(record.SelectedWorkspaceKey)
	view := s.mergedThreadView(surface, threadID)
	if view == nil || workspaceKey == "" || mergedThreadWorkspaceClaimKey(view) != workspaceKey {
		record.HistoryLoadingThreadID = ""
		return nil
	}
	if history := s.SurfaceThreadHistory(surface.SurfaceSessionID); history != nil && strings.TrimSpace(history.Thread.ThreadID) == strings.TrimSpace(threadID) {
		record.HistoryLoadingThreadID = ""
		return nil
	}
	instanceID := s.targetPickerHistoryReadInstanceID(surface, view)
	if instanceID == "" {
		record.HistoryLoadingThreadID = ""
		return nil
	}
	record.HistoryLoadingThreadID = strings.TrimSpace(threadID)
	return &eventcontract.Event{
		Kind:             eventcontract.KindDaemonCommand,
		GatewayID:        surface.GatewayID,
		SurfaceSessionID: surface.SurfaceSessionID,
		DaemonCommand: &control.DaemonCommand{
			Kind:             control.DaemonCommandThreadHistoryRead,
			GatewayID:        surface.GatewayID,
			SurfaceSessionID: surface.SurfaceSessionID,
			PickerID:         record.PickerID,
			InstanceID:       instanceID,
			ThreadID:         threadID,
		},
	}
}

func (s *Service) targetPickerHistoryReadInstanceID(surface *state.SurfaceConsoleRecord, view *mergedThreadView) string {
	if surface == nil || view == nil {
		return ""
	}
	target := s.resolveThreadTargetFromView(surface, view)
	if target.Instance != nil && target.Instance.Online {
		return strings.TrimSpace(target.Instance.InstanceID)
	}
	if target.Mode == threadAttachCurrentVisible {
		inst := s.root.Instances[strings.TrimSpace(surface.AttachedInstanceID)]
		if inst != nil && inst.Online {
			return strings.TrimSpace(inst.InstanceID)
		}
	}
	if view.FreeVisibleInst != nil && view.FreeVisibleInst.Online {
		return strings.TrimSpace(view.FreeVisibleInst.InstanceID)
	}
	if view.AnyVisibleInst != nil && view.AnyVisibleInst.Online {
		return strings.TrimSpace(view.AnyVisibleInst.InstanceID)
	}
	return ""
}

func (s *Service) targetPickerSelectedSessionContextSections(surface *state.SurfaceConsoleRecord, workspaceKey, sessionValue string) []control.FeishuCardTextSection {
	workspaceKey = normalizeWorkspaceClaimKey(workspaceKey)
	kind, threadID := parseTargetPickerSessionValue(sessionValue)
	if workspaceKey == "" || kind != control.FeishuTargetPickerSessionThread || strings.TrimSpace(threadID) == "" {
		return nil
	}
	view := s.mergedThreadView(surface, threadID)
	if view == nil || mergedThreadWorkspaceClaimKey(view) != workspaceKey {
		return nil
	}
	lines := s.targetPickerRecentThreadHistoryLines(surface, view)
	if len(lines) == 0 {
		lines = targetPickerRecentLocalThreadHistoryLines(threadID, 5)
	}
	if len(lines) == 0 && s.targetPickerSelectedSessionHistoryLoading(surface, threadID) {
		lines = []string{"正在读取最近 5 轮历史对话..."}
	}
	if len(lines) == 0 {
		lines = targetPickerThreadContextLines(view.Thread)
	}
	if len(lines) == 0 {
		lines = []string{"历史摘要暂不可见，确认切换后会从本地 Codex 继续恢复完整上下文。"}
	}
	return []control.FeishuCardTextSection{{
		Label: "历史对话摘要",
		Lines: lines,
	}}
}

func (s *Service) targetPickerSelectedSessionHistoryLoading(surface *state.SurfaceConsoleRecord, threadID string) bool {
	record := s.activeTargetPicker(surface)
	return record != nil &&
		strings.TrimSpace(record.HistoryLoadingThreadID) != "" &&
		strings.TrimSpace(record.HistoryLoadingThreadID) == strings.TrimSpace(threadID)
}

func (s *Service) HandleSurfaceTargetPickerHistoryLoaded(surfaceID, threadID string) []eventcontract.Event {
	surface := s.root.Surfaces[strings.TrimSpace(surfaceID)]
	flow := s.activeOwnerCardFlow(surface)
	record := s.activeTargetPicker(surface)
	if surface == nil || flow == nil || flow.Kind != ownerCardFlowKindTargetPicker || record == nil {
		return nil
	}
	if !targetPickerSelectedThreadMatches(record, threadID) {
		return nil
	}
	record.HistoryLoadingThreadID = ""
	view, err := s.buildTargetPickerView(surface, record)
	if err != nil {
		return notice(surface, "target_picker_unavailable", err.Error())
	}
	return []eventcontract.Event{s.targetPickerViewEvent(surface, view, false)}
}

func (s *Service) HandleSurfaceTargetPickerHistoryFailure(surfaceID, threadID, code, text string) []eventcontract.Event {
	surface := s.root.Surfaces[strings.TrimSpace(surfaceID)]
	flow := s.activeOwnerCardFlow(surface)
	record := s.activeTargetPicker(surface)
	if surface == nil || flow == nil || flow.Kind != ownerCardFlowKindTargetPicker || record == nil {
		return nil
	}
	if !targetPickerSelectedThreadMatches(record, threadID) {
		return nil
	}
	record.HistoryLoadingThreadID = ""
	setTargetPickerMessages(record, control.FeishuTargetPickerMessage{
		Level: control.FeishuTargetPickerMessageWarning,
		Text:  firstNonEmpty(strings.TrimSpace(text), "最近 5 轮历史读取失败，确认切换后仍会恢复完整会话上下文。"),
	})
	view, err := s.buildTargetPickerView(surface, record)
	if err != nil {
		return notice(surface, firstNonEmpty(code, "target_picker_unavailable"), err.Error())
	}
	return []eventcontract.Event{s.targetPickerViewEvent(surface, view, false)}
}

func targetPickerSelectedThreadMatches(record *activeTargetPickerRecord, threadID string) bool {
	if record == nil {
		return false
	}
	kind, selectedThreadID := parseTargetPickerSessionValue(record.SelectedSessionValue)
	return kind == control.FeishuTargetPickerSessionThread &&
		strings.TrimSpace(selectedThreadID) != "" &&
		strings.TrimSpace(selectedThreadID) == strings.TrimSpace(threadID)
}

func (s *Service) targetPickerRecentThreadHistoryLines(surface *state.SurfaceConsoleRecord, view *mergedThreadView) []string {
	if surface == nil || view == nil || strings.TrimSpace(view.ThreadID) == "" {
		return nil
	}
	history := s.SurfaceThreadHistory(surface.SurfaceSessionID)
	if history == nil || strings.TrimSpace(history.Thread.ThreadID) != strings.TrimSpace(view.ThreadID) {
		return nil
	}
	currentTurnID := ""
	if view.Inst != nil {
		currentTurnID = strings.TrimSpace(view.Inst.ActiveTurnID)
	}
	return targetPickerRecentThreadHistoryLines(*history, currentTurnID, 5)
}

func targetPickerRecentThreadHistoryLines(history agentproto.ThreadHistoryRecord, currentTurnID string, limit int) []string {
	if limit <= 0 {
		limit = 5
	}
	turns := targetPickerDialogueTurnsFromAgentHistory(history, currentTurnID)
	if len(turns) == 0 {
		return nil
	}
	if len(turns) > limit {
		turns = turns[:limit]
	}
	lines := make([]string, 0, len(turns))
	for i := len(turns) - 1; i >= 0; i-- {
		turn := turns[i]
		prefix := fmt.Sprintf("#%d", turn.Ordinal)
		if turn.IsCurrent {
			prefix = "当前 " + prefix
		}
		full := i == 0
		input := targetPickerDialoguePreview(turn.User, full)
		output := targetPickerDialoguePreview(turn.Assistant, full)
		line := prefix + " " + input
		if output != "" {
			line += " -> " + output
		}
		lines = append(lines, line)
	}
	return lines
}

func targetPickerDialogueTurnsFromAgentHistory(history agentproto.ThreadHistoryRecord, currentTurnID string) []targetPickerDialogueTurn {
	if len(history.Turns) == 0 {
		return nil
	}
	turns := make([]targetPickerDialogueTurn, 0, len(history.Turns))
	for i := len(history.Turns) - 1; i >= 0; i-- {
		turn := history.Turns[i]
		dialogue := targetPickerDialogueTurn{
			Ordinal:   i + 1,
			IsCurrent: strings.TrimSpace(currentTurnID) != "" && strings.TrimSpace(turn.TurnID) == strings.TrimSpace(currentTurnID),
		}
		for _, item := range turn.Items {
			switch strings.TrimSpace(item.Kind) {
			case "user_message":
				if text := strings.TrimSpace(item.Text); text != "" {
					dialogue.User = text
				}
			case "agent_message":
				if text := strings.TrimSpace(item.Text); text != "" {
					dialogue.Assistant = text
				}
			}
		}
		if strings.TrimSpace(dialogue.User) == "" && strings.TrimSpace(dialogue.Assistant) == "" {
			continue
		}
		turns = append(turns, dialogue)
	}
	return turns
}

func targetPickerDialoguePreview(text string, full bool) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return "-"
	}
	if full {
		return text
	}
	return truncateThreadHistoryText(text, 160)
}

func targetPickerThreadContextLines(thread *state.ThreadRecord) []string {
	if thread == nil {
		return nil
	}
	lines := make([]string, 0, 3)
	seen := map[string]struct{}{}
	appendLine := func(prefix, value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		key := strings.TrimSpace(prefix + value)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		lines = append(lines, prefix+value)
	}
	appendLine("最初输入：", threadFirstUserSnippet(thread, 80))
	appendLine("最近输入：", threadLastUserSnippet(thread, 80))
	appendLine("最近回复：", threadLastAssistantSnippet(thread, 80))
	if len(lines) == 0 {
		appendLine("摘要：", previewSnippet(thread.Preview))
	}
	return lines
}

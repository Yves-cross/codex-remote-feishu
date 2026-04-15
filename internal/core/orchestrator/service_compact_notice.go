package orchestrator

import (
	"strings"

	"github.com/kxn/codex-remote-feishu/internal/core/agentproto"
	"github.com/kxn/codex-remote-feishu/internal/core/control"
	"github.com/kxn/codex-remote-feishu/internal/core/state"
)

func compactCompletionNotice() control.Notice {
	return control.Notice{
		Code: "context_compacted",
		Text: "上下文已整理。",
	}
}

func compactCompletionProgressEntryRecord(itemID string) state.ExecCommandProgressEntryRecord {
	itemID = strings.TrimSpace(itemID)
	if itemID == "" {
		itemID = "context_compaction"
	}
	return state.ExecCommandProgressEntryRecord{
		ItemID:  itemID,
		Kind:    "context_compaction",
		Label:   "整理",
		Summary: "上下文已整理。",
		Status:  "completed",
	}
}

func compactCompletionProgressEntry(itemID string) control.ExecCommandProgressEntry {
	entry := compactCompletionProgressEntryRecord(itemID)
	return control.ExecCommandProgressEntry{
		ItemID:  entry.ItemID,
		Kind:    entry.Kind,
		Label:   entry.Label,
		Summary: entry.Summary,
		Status:  entry.Status,
	}
}

func (s *Service) renderCompactNotice(instanceID string, event agentproto.Event) []control.UIEvent {
	inst := s.root.Instances[instanceID]
	notice := compactCompletionNotice()
	surface := s.surfaceForInitiator(instanceID, event)
	if surface == nil {
		if inst != nil && strings.TrimSpace(event.ThreadID) != "" {
			s.storeThreadReplayNotice(inst, event.ThreadID, notice)
		}
		return nil
	}
	if inst != nil && strings.TrimSpace(event.ThreadID) != "" {
		s.clearThreadReplay(inst, event.ThreadID)
	}
	if !s.surfaceAllowsProcessProgress(surface, event.ItemKind) {
		return nil
	}
	progress := s.activeOrEnsureExecCommandProgress(surface, instanceID, event.ThreadID, event.TurnID)
	progress.ItemID = firstNonEmpty(strings.TrimSpace(event.ItemID), progress.ItemID)
	upsertExecCommandProgressEntry(progress, compactCompletionProgressEntryRecord(event.ItemID))
	return s.emitExecCommandProgress(surface, progress, event.ThreadID, event.TurnID, false)
}

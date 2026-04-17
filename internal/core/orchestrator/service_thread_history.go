package orchestrator

import "github.com/kxn/codex-remote-feishu/internal/core/agentproto"

func (s *Service) RecordSurfaceThreadHistory(surfaceID string, history agentproto.ThreadHistoryRecord) {
	if s == nil || s.pickers == nil {
		return
	}
	s.pickers.recordSurfaceThreadHistory(surfaceID, history)
}

func (s *Service) SurfaceThreadHistory(surfaceID string) *agentproto.ThreadHistoryRecord {
	if s == nil || s.pickers == nil {
		return nil
	}
	return s.pickers.surfaceThreadHistory(surfaceID)
}

func cloneThreadHistoryRecord(history agentproto.ThreadHistoryRecord) agentproto.ThreadHistoryRecord {
	cloned := history
	if len(history.Turns) != 0 {
		cloned.Turns = make([]agentproto.ThreadHistoryTurnRecord, len(history.Turns))
		for i, turn := range history.Turns {
			clonedTurn := turn
			if len(turn.Items) != 0 {
				clonedTurn.Items = make([]agentproto.ThreadHistoryItemRecord, len(turn.Items))
				for j, item := range turn.Items {
					clonedItem := item
					if item.ExitCode != nil {
						value := *item.ExitCode
						clonedItem.ExitCode = &value
					}
					clonedTurn.Items[j] = clonedItem
				}
			}
			cloned.Turns[i] = clonedTurn
		}
	}
	return cloned
}

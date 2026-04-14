package orchestrator

import (
	"strings"

	"github.com/kxn/codex-remote-feishu/internal/core/agentproto"
	"github.com/kxn/codex-remote-feishu/internal/core/control"
)

func compactCompletionNotice() control.Notice {
	return control.Notice{
		Code: "context_compacted",
		Text: "上下文已整理。",
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
	return []control.UIEvent{{
		Kind:             control.UIEventNotice,
		GatewayID:        surface.GatewayID,
		SurfaceSessionID: surface.SurfaceSessionID,
		Notice:           &notice,
	}}
}

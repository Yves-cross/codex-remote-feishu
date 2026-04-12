package orchestrator

import (
	"strings"
	"time"

	"github.com/kxn/codex-remote-feishu/internal/core/agentproto"
	"github.com/kxn/codex-remote-feishu/internal/core/control"
	"github.com/kxn/codex-remote-feishu/internal/core/state"
)

func (s *Service) applyThreadTokenUsageUpdate(instanceID string, event agentproto.Event) []control.UIEvent {
	inst := s.root.Instances[instanceID]
	if inst == nil || strings.TrimSpace(event.ThreadID) == "" || event.TokenUsage == nil {
		return nil
	}
	thread := s.ensureThread(inst, event.ThreadID)
	thread.TokenUsage = agentproto.CloneThreadTokenUsage(event.TokenUsage)
	s.recordRemoteTurnTokenUsage(instanceID, event.ThreadID, event.TurnID, event.TokenUsage)
	return nil
}

func (s *Service) recordRemoteTurnTokenUsage(instanceID, threadID, turnID string, usage *agentproto.ThreadTokenUsage) {
	if usage == nil {
		return
	}
	binding := s.lookupRemoteTurn(instanceID, threadID, turnID)
	if binding == nil {
		return
	}
	binding.LastUsage = usage.Last
	binding.HasLastUsage = true
}

func finalTurnSummaryForBinding(now time.Time, binding *remoteTurnBinding, thread *state.ThreadRecord) *control.FinalTurnSummary {
	if binding == nil || binding.StartedAt.IsZero() {
		return nil
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	elapsed := now.Sub(binding.StartedAt)
	if elapsed <= 0 {
		return nil
	}
	summary := &control.FinalTurnSummary{
		Elapsed:   elapsed,
		ThreadCWD: strings.TrimSpace(binding.ThreadCWD),
	}
	if binding.HasLastUsage {
		summary.Usage = &control.FinalTurnUsage{
			InputTokens:           binding.LastUsage.InputTokens,
			CachedInputTokens:     binding.LastUsage.CachedInputTokens,
			OutputTokens:          binding.LastUsage.OutputTokens,
			ReasoningOutputTokens: binding.LastUsage.ReasoningOutputTokens,
			TotalTokens:           binding.LastUsage.TotalTokens,
		}
	}
	if thread != nil && thread.TokenUsage != nil {
		summary.TotalTokensInContext = thread.TokenUsage.Total.TotalTokens
		if thread.TokenUsage.ModelContextWindow != nil {
			value := *thread.TokenUsage.ModelContextWindow
			summary.ModelContextWindow = &value
		}
	}
	return summary
}

package orchestrator

import (
	"strings"
	"time"

	"github.com/kxn/codex-remote-feishu/internal/core/agentproto"
	"github.com/kxn/codex-remote-feishu/internal/core/control"
	"github.com/kxn/codex-remote-feishu/internal/core/state"
)

const execCommandProgressTransientAnimationInterval = 1500 * time.Millisecond

func (s *Service) handleAssistantMessageProgressStart(instanceID string, event agentproto.Event) []control.UIEvent {
	return s.clearTransientExecCommandProgressStatus(instanceID, event.ThreadID, event.TurnID)
}

func (s *Service) handleReasoningSummaryProgressDelta(instanceID string, event agentproto.Event) []control.UIEvent {
	surface := s.turnSurface(instanceID, event.ThreadID, event.TurnID)
	if surface == nil || state.NormalizeSurfaceVerbosity(surface.Verbosity) != state.SurfaceVerbosityVerbose {
		return nil
	}
	progress := s.activeOrEnsureExecCommandProgress(surface, instanceID, event.ThreadID, event.TurnID)
	if progress == nil {
		return nil
	}
	if strings.TrimSpace(event.ItemID) != "" {
		progress.ItemID = strings.TrimSpace(event.ItemID)
	}
	if !upsertExecCommandProgressTransientStatus(progress, event, s.now()) {
		return nil
	}
	return s.emitExecCommandProgress(surface, progress, event.ThreadID, event.TurnID, false)
}

func (s *Service) clearTransientExecCommandProgressStatus(instanceID, threadID, turnID string) []control.UIEvent {
	surface := s.turnSurface(instanceID, threadID, turnID)
	progress := activeExecCommandProgress(surface, instanceID, threadID, turnID)
	if progress == nil || !execCommandProgressHasVisibleTransientStatus(progress) {
		return nil
	}
	clearExecCommandProgressTransientStatus(progress)
	return s.emitExecCommandProgress(surface, progress, threadID, turnID, false)
}

func execCommandProgressTransientStatus(progress *state.ExecCommandProgressRecord) *control.ExecCommandProgressTransientStatus {
	if progress == nil || progress.TransientStatus == nil {
		return nil
	}
	text := strings.TrimSpace(progress.TransientStatus.Text)
	if text == "" {
		return nil
	}
	return &control.ExecCommandProgressTransientStatus{
		Kind: strings.TrimSpace(progress.TransientStatus.Kind),
		Text: formatExecProgressTransientStatus(text, progress.TransientStatus.AnimationStep),
	}
}

func upsertExecCommandProgressTransientStatus(progress *state.ExecCommandProgressRecord, event agentproto.Event, now time.Time) bool {
	if progress == nil || strings.TrimSpace(event.Delta) == "" {
		return false
	}
	record := progress.TransientStatus
	if record == nil {
		record = &state.ExecCommandProgressTransientStatusRecord{Kind: "reasoning"}
		progress.TransientStatus = record
	}
	summaryIndex := lookupIntFromAny(event.Metadata["summaryIndex"])
	if summaryIndex != record.BufferSummaryIndex {
		record.Buffer = ""
		record.BufferSummaryIndex = summaryIndex
	}
	record.Buffer += event.Delta
	title := extractFirstMarkdownBold(record.Buffer)
	if title == "" {
		return false
	}
	display := localizeExecProgressTransientStatus(title)
	if display == "" {
		return false
	}
	if strings.TrimSpace(record.Text) == display && record.VisibleSummaryIndex == summaryIndex {
		return false
	}
	record.RawText = title
	record.Text = display
	record.VisibleSummaryIndex = summaryIndex
	record.AnimationStep = 0
	record.LastAnimatedAt = now
	return true
}

func clearExecCommandProgressTransientStatus(progress *state.ExecCommandProgressRecord) {
	if progress == nil || progress.TransientStatus == nil {
		return
	}
	progress.TransientStatus.Text = ""
	progress.TransientStatus.RawText = ""
	progress.TransientStatus.VisibleSummaryIndex = 0
	progress.TransientStatus.Buffer = ""
	progress.TransientStatus.BufferSummaryIndex = 0
	progress.TransientStatus.AnimationStep = 0
	progress.TransientStatus.LastAnimatedAt = time.Time{}
}

func execCommandProgressHasVisibleTransientStatus(progress *state.ExecCommandProgressRecord) bool {
	return progress != nil &&
		progress.TransientStatus != nil &&
		strings.TrimSpace(progress.TransientStatus.Text) != ""
}

func extractFirstMarkdownBold(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	for i := 0; i+1 < len(value); i++ {
		if value[i] != '*' || value[i+1] != '*' {
			continue
		}
		start := i + 2
		for j := start; j+1 < len(value); j++ {
			if value[j] == '*' && value[j+1] == '*' {
				return strings.TrimSpace(value[start:j])
			}
		}
		return ""
	}
	return ""
}

func localizeExecProgressTransientStatus(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if containsHan(raw) {
		return normalizeExecProgressTransientBaseText(raw)
	}
	normalized := strings.ToLower(strings.Join(strings.Fields(raw), " "))
	switch normalized {
	case "thinking":
		return "思考中"
	case "planning":
		return "规划中"
	case "analyzing":
		return "分析中"
	case "reviewing":
		return "审查中"
	case "checking":
		return "检查中"
	case "exploring":
		return "探索中"
	case "investigating":
		return "排查中"
	}
	switch transientStatusVerb(normalized) {
	case "thinking", "considering", "evaluating", "assessing", "deciding", "using", "continuing":
		return "思考中"
	case "planning":
		return "规划中"
	case "analyzing":
		return "分析中"
	case "reviewing":
		return "审查中"
	case "checking", "verifying", "inspecting", "monitoring":
		return "检查中"
	case "exploring":
		return "探索中"
	case "investigating", "troubleshooting", "debugging":
		return "排查中"
	case "searching", "locating", "identifying":
		return "查找中"
	case "preparing", "finalizing", "creating", "restoring":
		return "准备中"
	case "summarizing", "clarifying", "refining":
		return "整理中"
	case "modifying", "updating", "editing":
		return "修改中"
	default:
		return "思考中"
	}
}

func transientStatusVerb(normalized string) string {
	if normalized == "" {
		return ""
	}
	head := normalized
	if idx := strings.IndexByte(head, ' '); idx >= 0 {
		head = head[:idx]
	}
	head = strings.Trim(head, ".,:;!?()[]{}\"'")
	return head
}

func normalizeExecProgressTransientBaseText(text string) string {
	text = strings.TrimSpace(text)
	text = strings.TrimRight(text, ".")
	text = strings.TrimRight(text, "。")
	text = strings.TrimRight(text, "…")
	text = strings.TrimSpace(text)
	text = strings.TrimPrefix(text, "正在")
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	return text
}

func formatExecProgressTransientStatus(text string, step int) string {
	text = normalizeExecProgressTransientBaseText(text)
	if text == "" {
		return ""
	}
	dotCount := (step % 3) + 1
	return "正在" + text + strings.Repeat(".", dotCount)
}

func containsHan(value string) bool {
	for _, r := range value {
		if r >= 0x4e00 && r <= 0x9fff {
			return true
		}
	}
	return false
}

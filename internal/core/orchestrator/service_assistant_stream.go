package orchestrator

import (
	"strings"
	"time"

	"github.com/kxn/codex-remote-feishu/internal/core/agentproto"
	"github.com/kxn/codex-remote-feishu/internal/core/control"
	"github.com/kxn/codex-remote-feishu/internal/core/eventcontract"
	"github.com/kxn/codex-remote-feishu/internal/core/state"
)

const (
	assistantStreamMinInterval      = 900 * time.Millisecond
	assistantStreamMaxInterval      = 2500 * time.Millisecond
	assistantStreamMinPatchGrowth   = 80
	assistantStreamShortPatchGrowth = 24
	assistantStreamLoadingInterval  = 800 * time.Millisecond
	assistantStreamMaxOpenDuration  = 25 * time.Second
)

func (s *Service) handleAssistantStreamStart(instanceID string, event agentproto.Event) []eventcontract.Event {
	if strings.TrimSpace(event.ItemKind) != "agent_message" {
		return nil
	}
	buf := s.itemBuffers[itemBufferKey(instanceID, event.ThreadID, event.TurnID, event.ItemID)]
	if buf == nil || !assistantStreamPhaseAllowed(buf.Phase) {
		return nil
	}
	surface := s.turnSurface(instanceID, event.ThreadID, event.TurnID)
	if surface == nil {
		return nil
	}
	stream := s.ensureAssistantStream(surface, instanceID, event.ThreadID, event.TurnID, event.ItemID)
	if stream.Closed {
		return nil
	}
	stream.Phase = strings.TrimSpace(buf.Phase)
	if strings.TrimSpace(stream.Text) != "" || strings.TrimSpace(stream.CompletedText) != "" {
		return nil
	}
	stream.Loading = true
	now := s.now()
	stream.LastEmittedAt = now
	stream.LastEmittedText = stream.Text
	return []eventcontract.Event{s.assistantStreamEvent(surface, stream)}
}

func (s *Service) handleAssistantStreamDelta(instanceID string, event agentproto.Event) []eventcontract.Event {
	if strings.TrimSpace(event.ItemKind) != "agent_message" || strings.TrimSpace(event.Delta) == "" {
		return nil
	}
	buf := s.itemBuffers[itemBufferKey(instanceID, event.ThreadID, event.TurnID, event.ItemID)]
	if buf == nil || !assistantStreamPhaseAllowed(buf.Phase) {
		return nil
	}
	surface := s.turnSurface(instanceID, event.ThreadID, event.TurnID)
	if surface == nil {
		return nil
	}
	stream := s.ensureAssistantStream(surface, instanceID, event.ThreadID, event.TurnID, event.ItemID)
	if stream.Closed {
		return nil
	}
	stream.Phase = strings.TrimSpace(buf.Phase)
	stream.Text = joinAssistantStreamText(stream.CompletedText, buf.text())
	stream.Loading = true
	if strings.TrimSpace(stream.Text) == "" {
		return nil
	}
	now := s.now()
	if !shouldEmitAssistantStreamPatch(stream, now) && !assistantStreamWasOnlyLoading(stream) {
		return nil
	}
	stream.LastEmittedAt = now
	stream.LastEmittedText = stream.Text
	return []eventcontract.Event{s.assistantStreamEvent(surface, stream)}
}

func assistantStreamPhaseAllowed(phase string) bool {
	switch strings.TrimSpace(phase) {
	case "final_answer", "commentary":
		return true
	default:
		return false
	}
}

func assistantStreamPhaseCompletesOnItemDone(phase string) bool {
	return strings.TrimSpace(phase) == "commentary"
}

func assistantStreamWasOnlyLoading(stream *state.AssistantStreamRecord) bool {
	if stream == nil {
		return false
	}
	return strings.TrimSpace(stream.LastEmittedText) == ""
}

func joinAssistantStreamText(parts ...string) string {
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return strings.Join(out, "\n\n")
}

func shouldEmitAssistantStreamPatch(stream *state.AssistantStreamRecord, now time.Time) bool {
	if stream == nil {
		return false
	}
	if stream.LastEmittedAt.IsZero() || strings.TrimSpace(stream.LastEmittedText) == "" {
		return true
	}
	elapsed := now.Sub(stream.LastEmittedAt)
	if elapsed < assistantStreamMinInterval {
		return false
	}
	growth := len([]rune(stream.Text)) - len([]rune(stream.LastEmittedText))
	if growth >= assistantStreamMinPatchGrowth {
		return true
	}
	if elapsed >= assistantStreamMaxInterval && growth > 0 {
		return true
	}
	if growth >= assistantStreamShortPatchGrowth && assistantStreamEndsAtReadableBoundary(stream.Text) {
		return true
	}
	return false
}

func (s *Service) tickAssistantStreamLoading(surface *state.SurfaceConsoleRecord, now time.Time) []eventcontract.Event {
	if surface == nil || surface.ActiveAssistantStream == nil {
		return nil
	}
	stream := surface.ActiveAssistantStream
	if stream.Closed || !stream.Loading || strings.TrimSpace(stream.MessageID) == "" || strings.TrimSpace(stream.StreamCardID) == "" {
		return nil
	}
	if strings.TrimSpace(stream.Text) == "" {
		return nil
	}
	if shouldCloseAssistantStreamBeforePlatformTimeout(stream, now) {
		event := s.assistantStreamEventWithDone(surface, stream, true)
		surface.ActiveAssistantStream = nil
		return []eventcontract.Event{event}
	}
	if !stream.LastEmittedAt.IsZero() && now.Sub(stream.LastEmittedAt) < assistantStreamLoadingInterval {
		return nil
	}
	stream.LastEmittedAt = now
	stream.LastEmittedText = stream.Text
	return []eventcontract.Event{s.assistantStreamEvent(surface, stream)}
}

func assistantStreamEndsAtReadableBoundary(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	last := []rune(text)[len([]rune(text))-1]
	switch last {
	case '\n', '.', '!', '?', ':', ';', ',', '。', '！', '？', '：', '；', '，':
		return true
	default:
		return false
	}
}

func (s *Service) ensureAssistantStream(surface *state.SurfaceConsoleRecord, instanceID, threadID, turnID, itemID string) *state.AssistantStreamRecord {
	if surface.ActiveAssistantStream != nil {
		stream := surface.ActiveAssistantStream
		if stream.InstanceID == instanceID && stream.ThreadID == threadID && stream.TurnID == turnID {
			stream.ItemID = itemID
			return stream
		}
	}
	sourceMessageID, sourceMessagePreview := s.replyAnchorForTurn(instanceID, threadID, turnID)
	surface.ActiveAssistantStream = &state.AssistantStreamRecord{
		InstanceID:           instanceID,
		ThreadID:             threadID,
		TurnID:               turnID,
		ItemID:               itemID,
		SourceMessageID:      sourceMessageID,
		SourceMessagePreview: sourceMessagePreview,
		OpenedAt:             s.now(),
	}
	return surface.ActiveAssistantStream
}

func shouldCloseAssistantStreamBeforePlatformTimeout(stream *state.AssistantStreamRecord, now time.Time) bool {
	if stream == nil || stream.OpenedAt.IsZero() || now.Sub(stream.OpenedAt) < assistantStreamMaxOpenDuration {
		return false
	}
	if strings.TrimSpace(stream.MessageID) == "" || strings.TrimSpace(stream.StreamCardID) == "" {
		return false
	}
	text := strings.TrimSpace(stream.Text)
	if text == "" || text != strings.TrimSpace(stream.CompletedText) {
		return false
	}
	return true
}

func (s *Service) assistantStreamEvent(surface *state.SurfaceConsoleRecord, stream *state.AssistantStreamRecord) eventcontract.Event {
	return s.assistantStreamEventWithDone(surface, stream, false)
}

func (s *Service) assistantStreamEventWithDone(surface *state.SurfaceConsoleRecord, stream *state.AssistantStreamRecord, done bool) eventcontract.Event {
	if done {
		stream.Loading = false
		stream.Closed = true
	}
	view := control.AssistantStreamView{
		ThreadID:             stream.ThreadID,
		TurnID:               stream.TurnID,
		ItemID:               stream.ItemID,
		MessageID:            strings.TrimSpace(stream.MessageID),
		StreamCardID:         strings.TrimSpace(stream.StreamCardID),
		SourceMessagePreview: strings.TrimSpace(stream.SourceMessagePreview),
		Text:                 strings.TrimSpace(stream.Text),
		Loading:              stream.Loading && !done,
		Done:                 done,
	}
	return eventcontract.Event{
		Kind:                 eventcontract.KindAssistantStream,
		SurfaceSessionID:     surface.SurfaceSessionID,
		SourceMessageID:      strings.TrimSpace(stream.SourceMessageID),
		SourceMessagePreview: strings.TrimSpace(stream.SourceMessagePreview),
		AssistantStream:      &view,
	}
}

func (s *Service) RecordAssistantStreamMessage(surfaceID, threadID, turnID, itemID, messageID, streamCardID string) {
	surface := s.root.Surfaces[surfaceID]
	if surface == nil || surface.ActiveAssistantStream == nil {
		return
	}
	stream := surface.ActiveAssistantStream
	if stream.ThreadID != threadID || stream.TurnID != turnID || stream.ItemID != itemID {
		return
	}
	stream.MessageID = strings.TrimSpace(messageID)
	if cardID := strings.TrimSpace(streamCardID); cardID != "" {
		stream.StreamCardID = cardID
	}
}

func activeAssistantStream(surface *state.SurfaceConsoleRecord, instanceID, threadID, turnID, itemID string) *state.AssistantStreamRecord {
	if surface == nil || surface.ActiveAssistantStream == nil {
		return nil
	}
	stream := surface.ActiveAssistantStream
	if stream.InstanceID != instanceID || stream.ThreadID != threadID || stream.TurnID != turnID {
		return nil
	}
	if strings.TrimSpace(itemID) != "" && stream.ItemID != itemID {
		return nil
	}
	return stream
}

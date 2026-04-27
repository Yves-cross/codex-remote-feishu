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
	assistantStreamEmitInterval   = 100 * time.Millisecond
	assistantStreamInitialStep    = 1
	assistantStreamCatchupDivisor = 12
	assistantStreamMaxStep        = 12
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
	stream.Phase = strings.TrimSpace(buf.Phase)
	if strings.TrimSpace(stream.Text) != "" || strings.TrimSpace(stream.CompletedText) != "" {
		return nil
	}
	stream.Loading = true
	now := s.now()
	stream.LastEmittedAt = now
	stream.LastEmittedText = stream.VisibleText
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
	stream.Phase = strings.TrimSpace(buf.Phase)
	stream.Text = joinAssistantStreamText(stream.CompletedText, buf.text())
	stream.Loading = true
	if strings.TrimSpace(stream.Text) == "" {
		return nil
	}
	now := s.now()
	if !assistantStreamWasOnlyLoading(stream) && !assistantStreamReadyToEmit(stream, now) {
		return nil
	}
	if !advanceAssistantStreamVisibleText(stream) {
		return nil
	}
	stream.LastEmittedAt = now
	stream.LastEmittedText = stream.VisibleText
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
	return strings.TrimSpace(stream.VisibleText) == "" && strings.TrimSpace(stream.LastEmittedText) == ""
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

func assistantStreamReadyToEmit(stream *state.AssistantStreamRecord, now time.Time) bool {
	if stream == nil {
		return false
	}
	if stream.LastEmittedAt.IsZero() {
		return true
	}
	return now.Sub(stream.LastEmittedAt) >= assistantStreamEmitInterval
}

func (s *Service) tickAssistantStreamLoading(surface *state.SurfaceConsoleRecord, now time.Time) []eventcontract.Event {
	if surface == nil || surface.ActiveAssistantStream == nil {
		return nil
	}
	stream := surface.ActiveAssistantStream
	if !stream.Loading || strings.TrimSpace(stream.MessageID) == "" || strings.TrimSpace(stream.StreamCardID) == "" {
		return nil
	}
	if strings.TrimSpace(stream.Text) == "" {
		return nil
	}
	if strings.TrimSpace(stream.VisibleText) == strings.TrimSpace(stream.Text) {
		return nil
	}
	if !assistantStreamReadyToEmit(stream, now) {
		return nil
	}
	if !advanceAssistantStreamVisibleText(stream) {
		return nil
	}
	stream.LastEmittedAt = now
	stream.LastEmittedText = stream.VisibleText
	return []eventcontract.Event{s.assistantStreamEvent(surface, stream)}
}

func advanceAssistantStreamVisibleText(stream *state.AssistantStreamRecord) bool {
	if stream == nil {
		return false
	}
	target := strings.TrimSpace(stream.Text)
	if target == "" {
		return false
	}
	targetRunes := []rune(target)
	visibleRunes := []rune(strings.TrimSpace(stream.VisibleText))
	if len(visibleRunes) >= len(targetRunes) {
		return false
	}
	step := assistantStreamVisibleStep(len(visibleRunes), len(targetRunes))
	nextLen := len(visibleRunes) + step
	if nextLen > len(targetRunes) {
		nextLen = len(targetRunes)
	}
	stream.VisibleText = string(targetRunes[:nextLen])
	return true
}

func assistantStreamVisibleStep(visibleLen, targetLen int) int {
	remaining := targetLen - visibleLen
	if remaining <= 0 {
		return 0
	}
	if visibleLen == 0 {
		if remaining < assistantStreamInitialStep {
			return remaining
		}
		return assistantStreamInitialStep
	}
	step := (remaining + assistantStreamCatchupDivisor - 1) / assistantStreamCatchupDivisor
	if step < 1 {
		step = 1
	}
	if step > assistantStreamMaxStep {
		step = assistantStreamMaxStep
	}
	return step
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
	}
	return surface.ActiveAssistantStream
}

func (s *Service) assistantStreamEvent(surface *state.SurfaceConsoleRecord, stream *state.AssistantStreamRecord) eventcontract.Event {
	return s.assistantStreamEventWithDone(surface, stream, false)
}

func (s *Service) assistantStreamEventWithDone(surface *state.SurfaceConsoleRecord, stream *state.AssistantStreamRecord, done bool) eventcontract.Event {
	text := strings.TrimSpace(stream.VisibleText)
	if done {
		stream.Loading = false
		text = strings.TrimSpace(firstNonEmpty(stream.Text, stream.VisibleText))
		stream.VisibleText = text
		stream.LastEmittedText = text
	}
	view := control.AssistantStreamView{
		ThreadID:             stream.ThreadID,
		TurnID:               stream.TurnID,
		ItemID:               stream.ItemID,
		MessageID:            strings.TrimSpace(stream.MessageID),
		StreamCardID:         strings.TrimSpace(stream.StreamCardID),
		SourceMessagePreview: strings.TrimSpace(stream.SourceMessagePreview),
		Text:                 text,
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

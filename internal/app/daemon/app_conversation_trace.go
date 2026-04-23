package daemon

import (
	"strings"
	"time"

	"github.com/kxn/codex-remote-feishu/internal/conversationtrace"
	"github.com/kxn/codex-remote-feishu/internal/core/agentproto"
	"github.com/kxn/codex-remote-feishu/internal/core/control"
	"github.com/kxn/codex-remote-feishu/internal/core/eventcontract"
	"github.com/kxn/codex-remote-feishu/internal/core/orchestrator"
	"github.com/kxn/codex-remote-feishu/internal/core/render"
)

type conversationTracer interface {
	Log(conversationtrace.Entry)
	Close() error
}

func (a *App) traceConversation(entry conversationtrace.Entry) {
	if a == nil || a.conversationTrace == nil {
		return
	}
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now().UTC()
	}
	a.conversationTrace.Log(entry)
}

func (a *App) traceUserTextAction(action control.Action) {
	if action.Kind != control.ActionTextMessage {
		return
	}
	text := strings.TrimSpace(action.Text)
	if text == "" {
		return
	}
	a.traceConversation(conversationtrace.Entry{
		Event:            conversationtrace.EventUserMessage,
		Actor:            "user",
		SurfaceSessionID: strings.TrimSpace(action.SurfaceSessionID),
		ChatID:           strings.TrimSpace(action.ChatID),
		MessageID:        strings.TrimSpace(action.MessageID),
		Text:             text,
	})
}

func (a *App) traceSteerCommand(surfaceID, instanceID string, command agentproto.Command) {
	if command.Kind != agentproto.CommandTurnSteer {
		return
	}
	parts := make([]string, 0, len(command.Prompt.Inputs))
	for _, input := range command.Prompt.Inputs {
		if input.Type != agentproto.InputText {
			continue
		}
		text := strings.TrimSpace(input.Text)
		if text != "" {
			parts = append(parts, text)
		}
	}
	surfaceID = strings.TrimSpace(surfaceID)
	sourceMessageID := strings.TrimSpace(command.Origin.MessageID)
	a.traceConversation(conversationtrace.Entry{
		Event:            conversationtrace.EventSteerMessage,
		Actor:            "user",
		SurfaceSessionID: surfaceID,
		ChatID:           strings.TrimSpace(a.service.SurfaceChatID(surfaceID)),
		MessageID:        sourceMessageID,
		InstanceID:       strings.TrimSpace(instanceID),
		ThreadID:         strings.TrimSpace(command.Target.ThreadID),
		TurnID:           strings.TrimSpace(command.Target.TurnID),
		Text:             strings.Join(parts, "\n\n"),
	})
}

func (a *App) traceAssistantBlock(event eventcontract.Event) {
	if event.Kind != eventcontract.EventBlockCommitted || event.Block == nil {
		return
	}
	if event.Block.Kind != render.BlockAssistantMarkdown && event.Block.Kind != render.BlockAssistantCode {
		return
	}
	text := strings.TrimSpace(event.Block.Text)
	if text == "" {
		return
	}
	surfaceID := strings.TrimSpace(event.SurfaceSessionID)
	instanceID := strings.TrimSpace(event.Block.InstanceID)
	if instanceID == "" {
		instanceID = strings.TrimSpace(a.service.AttachedInstanceID(surfaceID))
	}
	a.traceConversation(conversationtrace.Entry{
		Event:            conversationtrace.EventAssistantText,
		Actor:            "assistant",
		SurfaceSessionID: surfaceID,
		ChatID:           strings.TrimSpace(a.service.SurfaceChatID(surfaceID)),
		MessageID:        strings.TrimSpace(event.SourceMessageID),
		InstanceID:       instanceID,
		ThreadID:         strings.TrimSpace(event.Block.ThreadID),
		TurnID:           strings.TrimSpace(event.Block.TurnID),
		Text:             text,
		Final:            event.Block.Final,
	})
}

func (a *App) traceTurnLifecycle(instanceID string, event agentproto.Event) {
	if !shouldTraceTurnLifecycle(event) {
		return
	}
	kind := conversationtrace.EventKind("")
	switch event.Kind {
	case agentproto.EventTurnStarted:
		kind = conversationtrace.EventTurnStarted
	case agentproto.EventTurnCompleted:
		kind = conversationtrace.EventTurnCompleted
	default:
		return
	}
	surfaceID, sourceMessageID := a.lookupTurnTraceBinding(instanceID, event.ThreadID, event.TurnID)
	status := strings.TrimSpace(event.Status)
	if status == "" && event.Kind == agentproto.EventTurnCompleted {
		status = "completed"
	}
	text := strings.TrimSpace(event.ErrorMessage)
	a.traceConversation(conversationtrace.Entry{
		Event:            kind,
		Actor:            "assistant",
		SurfaceSessionID: strings.TrimSpace(surfaceID),
		ChatID:           strings.TrimSpace(a.service.SurfaceChatID(surfaceID)),
		MessageID:        strings.TrimSpace(sourceMessageID),
		InstanceID:       strings.TrimSpace(instanceID),
		ThreadID:         strings.TrimSpace(event.ThreadID),
		TurnID:           strings.TrimSpace(event.TurnID),
		Status:           status,
		Text:             text,
	})
}

func shouldTraceTurnLifecycle(event agentproto.Event) bool {
	if event.Kind != agentproto.EventTurnStarted && event.Kind != agentproto.EventTurnCompleted {
		return false
	}
	if event.TrafficClass == agentproto.TrafficClassInternalHelper {
		return false
	}
	switch event.Initiator.Kind {
	case agentproto.InitiatorInternalHelper, agentproto.InitiatorLocalUI:
		return false
	default:
		return true
	}
}

func (a *App) lookupTurnTraceBinding(instanceID, threadID, turnID string) (surfaceID, sourceMessageID string) {
	instanceID = strings.TrimSpace(instanceID)
	threadID = strings.TrimSpace(threadID)
	turnID = strings.TrimSpace(turnID)
	if instanceID == "" {
		return "", ""
	}
	match := func(statuses []orchestrator.RemoteTurnStatus) (string, string) {
		for _, status := range statuses {
			if strings.TrimSpace(status.InstanceID) != instanceID {
				continue
			}
			if turnID != "" && strings.TrimSpace(status.TurnID) != turnID {
				continue
			}
			if threadID != "" && strings.TrimSpace(status.ThreadID) != threadID {
				continue
			}
			return strings.TrimSpace(status.SurfaceSessionID), strings.TrimSpace(status.SourceMessageID)
		}
		return "", ""
	}
	if surfaceID, sourceMessageID := match(a.service.ActiveRemoteTurns()); surfaceID != "" {
		return surfaceID, sourceMessageID
	}
	return match(a.service.PendingRemoteTurns())
}

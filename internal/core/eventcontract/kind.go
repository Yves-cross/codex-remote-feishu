package eventcontract

type Kind string

// EventKind is a compatibility alias for the previous UI event naming.
// New code should prefer Kind directly.
type EventKind = Kind

const (
	KindUnknown             Kind = ""
	KindSnapshot            Kind = "snapshot.updated"
	KindSelection           Kind = "selection.prompt"
	KindPage                Kind = "page.view"
	KindRequest             Kind = "request.prompt"
	KindPathPicker          Kind = "path.picker"
	KindTargetPicker        Kind = "target.picker"
	KindThreadHistory       Kind = "thread.history"
	KindPendingInput        Kind = "pending.input.state"
	KindNotice              Kind = "notice"
	KindPlanUpdate          Kind = "plan.updated"
	KindBlockCommitted      Kind = "block.committed"
	KindTimelineText        Kind = "timeline.text"
	KindImageOutput         Kind = "image.output"
	KindExecCommandProgress Kind = "exec_command.progress"
	KindAgentCommand        Kind = "agent.command"
	KindDaemonCommand       Kind = "daemon.command"
)

// Legacy UIEvent* aliases kept in eventcontract so cross-layer migrations can
// switch from eventcontract.Event* to eventcontract.Event* mechanically.
const (
	EventSnapshot            EventKind = KindSnapshot
	EventFeishuSelectionView EventKind = KindSelection
	EventFeishuPageView      EventKind = KindPage
	EventFeishuRequestView   EventKind = KindRequest
	EventFeishuPathPicker    EventKind = KindPathPicker
	EventFeishuTargetPicker  EventKind = KindTargetPicker
	EventFeishuThreadHistory EventKind = KindThreadHistory
	EventPendingInput        EventKind = KindPendingInput
	EventNotice              EventKind = KindNotice
	EventPlanUpdated         EventKind = KindPlanUpdate
	EventBlockCommitted      EventKind = KindBlockCommitted
	EventTimelineText        EventKind = KindTimelineText
	EventImageOutput         EventKind = KindImageOutput
	EventExecCommandProgress EventKind = KindExecCommandProgress
	EventAgentCommand        EventKind = KindAgentCommand
	EventDaemonCommand       EventKind = KindDaemonCommand
)

var allKinds = []Kind{
	KindSnapshot,
	KindSelection,
	KindPage,
	KindRequest,
	KindPathPicker,
	KindTargetPicker,
	KindThreadHistory,
	KindPendingInput,
	KindNotice,
	KindPlanUpdate,
	KindBlockCommitted,
	KindTimelineText,
	KindImageOutput,
	KindExecCommandProgress,
	KindAgentCommand,
	KindDaemonCommand,
}

func AllKinds() []Kind {
	out := make([]Kind, len(allKinds))
	copy(out, allKinds)
	return out
}

func IsKnownKind(kind Kind) bool {
	for _, candidate := range allKinds {
		if candidate == kind {
			return true
		}
	}
	return false
}

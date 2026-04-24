package frontstagecontract

import "strings"

type OwnerCardKind string

const (
	OwnerCardRequest      OwnerCardKind = "request"
	OwnerCardPage         OwnerCardKind = "page"
	OwnerCardTargetPicker OwnerCardKind = "target_picker"
	OwnerCardPathPicker   OwnerCardKind = "path_picker"
)

type Phase string

const (
	PhaseEditing         Phase = "editing"
	PhaseWaitingDispatch Phase = "waiting_dispatch"
	PhaseProcessing      Phase = "processing"
	PhaseSucceeded       Phase = "succeeded"
	PhaseFailed          Phase = "failed"
	PhaseCancelled       Phase = "cancelled"
	PhaseExpired         Phase = "expired"
)

type ActionPolicy string

const (
	ActionPolicyInteractive ActionPolicy = "interactive"
	ActionPolicyCancelOnly  ActionPolicy = "cancel_only"
	ActionPolicyReadOnly    ActionPolicy = "read_only"
)

type Frame struct {
	OwnerKind    OwnerCardKind
	Phase        Phase
	ActionPolicy ActionPolicy
}

func NormalizeFrame(frame Frame) Frame {
	frame.OwnerKind = normalizeOwnerCardKind(frame.OwnerKind)
	frame.Phase = NormalizePhase(frame.Phase)
	frame.ActionPolicy = normalizeActionPolicy(frame.Phase, frame.ActionPolicy)
	return frame
}

func NormalizePhase(phase Phase) Phase {
	switch strings.TrimSpace(string(phase)) {
	case string(PhaseWaitingDispatch):
		return PhaseWaitingDispatch
	case string(PhaseProcessing):
		return PhaseProcessing
	case string(PhaseSucceeded):
		return PhaseSucceeded
	case string(PhaseFailed):
		return PhaseFailed
	case string(PhaseCancelled):
		return PhaseCancelled
	case string(PhaseExpired):
		return PhaseExpired
	default:
		return PhaseEditing
	}
}

func DefaultActionPolicy(phase Phase) ActionPolicy {
	switch NormalizePhase(phase) {
	case PhaseEditing:
		return ActionPolicyInteractive
	case PhaseProcessing:
		return ActionPolicyReadOnly
	default:
		return ActionPolicyReadOnly
	}
}

func SealedForPhase(phase Phase) bool {
	switch NormalizePhase(phase) {
	case PhaseEditing, PhaseProcessing:
		return false
	default:
		return true
	}
}

func IsTerminalPhase(phase Phase) bool {
	switch NormalizePhase(phase) {
	case PhaseSucceeded, PhaseFailed, PhaseCancelled, PhaseExpired:
		return true
	default:
		return false
	}
}

func AllowsPrimaryInput(policy ActionPolicy) bool {
	return strings.TrimSpace(string(policy)) == string(ActionPolicyInteractive)
}

func AllowsCancelAction(policy ActionPolicy) bool {
	switch strings.TrimSpace(string(policy)) {
	case string(ActionPolicyInteractive), string(ActionPolicyCancelOnly):
		return true
	default:
		return false
	}
}

func normalizeOwnerCardKind(kind OwnerCardKind) OwnerCardKind {
	switch strings.TrimSpace(string(kind)) {
	case string(OwnerCardRequest):
		return OwnerCardRequest
	case string(OwnerCardPage):
		return OwnerCardPage
	case string(OwnerCardTargetPicker):
		return OwnerCardTargetPicker
	case string(OwnerCardPathPicker):
		return OwnerCardPathPicker
	default:
		return kind
	}
}

func normalizeActionPolicy(phase Phase, policy ActionPolicy) ActionPolicy {
	switch strings.TrimSpace(string(policy)) {
	case string(ActionPolicyInteractive):
		return ActionPolicyInteractive
	case string(ActionPolicyCancelOnly):
		if NormalizePhase(phase) == PhaseProcessing {
			return ActionPolicyCancelOnly
		}
		return DefaultActionPolicy(phase)
	case string(ActionPolicyReadOnly):
		return ActionPolicyReadOnly
	default:
		return DefaultActionPolicy(phase)
	}
}

package control

import "strings"

// ActionRequestResponse carries request-card response semantics so the root
// Action does not keep expanding with request-only fields.
type ActionRequestResponse struct {
	RequestID       string
	RequestType     string
	RequestOptionID string
	Answers         map[string][]string
	RequestRevision int
}

// ActionRequestControl carries request-card local control semantics such as
// canceling the current turn or skipping an optional prompt question.
type ActionRequestControl struct {
	RequestID       string
	RequestType     string
	Control         string
	QuestionID      string
	RequestRevision int
}

// ActionOwnerCardFlow carries owner-card follow-up actions such as upgrade and
// VS Code migration confirmation flows.
type ActionOwnerCardFlow struct {
	FlowID   string
	OptionID string
}

func (a Action) IsCardAction() bool {
	return a.Inbound != nil && strings.TrimSpace(a.Inbound.CardDaemonLifecycleID) != ""
}

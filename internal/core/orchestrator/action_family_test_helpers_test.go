package orchestrator

import "github.com/kxn/codex-remote-feishu/internal/core/control"

func testRequestAction(requestID, requestType, optionID string, answers map[string][]string, revision int) *control.ActionRequestResponse {
	return &control.ActionRequestResponse{
		RequestID:       requestID,
		RequestType:     requestType,
		RequestOptionID: optionID,
		Answers:         answers,
		RequestRevision: revision,
	}
}

func testRequestControl(requestID, requestType, controlName, questionID string, revision int) *control.ActionRequestControl {
	return &control.ActionRequestControl{
		RequestID:       requestID,
		RequestType:     requestType,
		Control:         controlName,
		QuestionID:      questionID,
		RequestRevision: revision,
	}
}

func testOwnerFlow(flowID, optionID string) *control.ActionOwnerCardFlow {
	return &control.ActionOwnerCardFlow{FlowID: flowID, OptionID: optionID}
}

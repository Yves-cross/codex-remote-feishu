package orchestrator

import (
	"github.com/kxn/codex-remote-feishu/internal/core/control"
	"github.com/kxn/codex-remote-feishu/internal/core/eventcontract"
)

var dropNoticeFollowupPolicy = control.FeishuFollowupPolicy{
	DropClasses: []control.FeishuFollowupHandoffClass{
		control.FeishuFollowupHandoffClassNotice,
		control.FeishuFollowupHandoffClassThreadSelection,
	},
}

func filterFollowupEventsByPolicy(events []eventcontract.Event, policy control.FeishuFollowupPolicy) []eventcontract.Event {
	return eventcontract.FilterEventsByFollowupPolicy(events, policy)
}

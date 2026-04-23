package daemon

import (
	"testing"

	"github.com/kxn/codex-remote-feishu/internal/core/control"
	"github.com/kxn/codex-remote-feishu/internal/core/eventcontract"
)

func TestFilterUIEventsByFollowupPolicyDropsThreadSelectionOnly(t *testing.T) {
	events := []eventcontract.Event{
		{
			Kind: eventcontract.EventNotice,
			Notice: &control.Notice{
				Code: "thread_selection_changed",
			},
			ThreadSelection: &control.ThreadSelectionChanged{
				ThreadID: "thread-1",
			},
		},
		{
			Kind: eventcontract.EventNotice,
			Notice: &control.Notice{
				Code: "some_other_notice",
			},
		},
	}

	filtered := filterUIEventsByFollowupPolicy(events, control.FeishuFollowupPolicy{
		DropClasses: []control.FeishuFollowupHandoffClass{
			control.FeishuFollowupHandoffClassThreadSelection,
		},
	})
	if len(filtered) != 1 {
		t.Fatalf("expected only non-thread-selection events to remain, got %#v", filtered)
	}
	if filtered[0].Notice == nil || filtered[0].Notice.Code != "some_other_notice" {
		t.Fatalf("unexpected filtered events: %#v", filtered)
	}
}

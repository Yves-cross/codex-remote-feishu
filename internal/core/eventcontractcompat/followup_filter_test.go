package eventcontractcompat

import (
	"testing"

	"github.com/kxn/codex-remote-feishu/internal/core/control"
	"github.com/kxn/codex-remote-feishu/internal/core/eventcontract"
	"github.com/kxn/codex-remote-feishu/internal/core/render"
)

func TestFilterLegacyUIEventsByFollowupPolicy(t *testing.T) {
	events := []control.UIEvent{
		{
			Kind: control.UIEventNotice,
			Notice: &control.Notice{
				Code: "thread_selection_changed",
			},
			ThreadSelection: &control.ThreadSelectionChanged{
				ThreadID: "thread-1",
			},
		},
		{
			Kind: control.UIEventNotice,
			Notice: &control.Notice{
				Code: "generic_notice",
			},
		},
		{
			Kind:                control.UIEventFeishuSelectionView,
			FeishuSelectionView: &control.FeishuSelectionView{},
		},
	}
	policy := control.FeishuFollowupPolicy{
		DropClasses: []control.FeishuFollowupHandoffClass{
			control.FeishuFollowupHandoffClassNotice,
			control.FeishuFollowupHandoffClassThreadSelection,
		},
	}
	filtered := FilterLegacyUIEventsByFollowupPolicy(events, policy)
	if len(filtered) != 1 {
		t.Fatalf("expected one event to remain, got %#v", filtered)
	}
	if filtered[0].Kind != control.UIEventFeishuSelectionView {
		t.Fatalf("unexpected remaining event: %#v", filtered[0])
	}
}

func TestFilterLegacyUIEventsByFollowupPolicyEmptyPolicyKeepsEvents(t *testing.T) {
	events := []control.UIEvent{
		{Kind: control.UIEventNotice, Notice: &control.Notice{Code: "notice"}},
	}
	filtered := FilterLegacyUIEventsByFollowupPolicy(events, control.FeishuFollowupPolicy{})
	if len(filtered) != 1 || filtered[0].Notice == nil {
		t.Fatalf("expected all events to remain, got %#v", filtered)
	}
}

func TestFilterLegacyUIEventsByFollowupPolicyMatrix(t *testing.T) {
	base := []control.UIEvent{
		{
			Kind:   control.UIEventNotice,
			Notice: &control.Notice{Code: "notice-only"},
		},
		{
			Kind:            control.UIEventNotice,
			Notice:          &control.Notice{Code: "thread-selection"},
			ThreadSelection: &control.ThreadSelectionChanged{ThreadID: "thread-1"},
		},
		{
			Kind:                control.UIEventFeishuSelectionView,
			FeishuSelectionView: &control.FeishuSelectionView{PromptKind: control.SelectionPromptUseThread},
		},
		{
			Kind:       control.UIEventPlanUpdated,
			PlanUpdate: &control.PlanUpdate{ThreadID: "thread-1"},
		},
		{
			Kind: control.UIEventBlockCommitted,
			Block: &render.Block{
				Kind:  render.BlockAssistantMarkdown,
				Text:  "final",
				Final: true,
			},
		},
	}

	cases := []struct {
		name       string
		policy     control.FeishuFollowupPolicy
		wantRemain []eventcontract.HandoffClass
	}{
		{
			name: "drop_notice_and_thread_selection",
			policy: control.FeishuFollowupPolicy{
				DropClasses: []control.FeishuFollowupHandoffClass{
					control.FeishuFollowupHandoffClassNotice,
					control.FeishuFollowupHandoffClassThreadSelection,
				},
			},
			wantRemain: []eventcontract.HandoffClass{
				eventcontract.HandoffClassNavigation,
				eventcontract.HandoffClassProcessDetail,
				eventcontract.HandoffClassTerminalContent,
			},
		},
		{
			name: "drop_navigation_but_keep_navigation_takes_precedence",
			policy: control.FeishuFollowupPolicy{
				DropClasses: []control.FeishuFollowupHandoffClass{
					control.FeishuFollowupHandoffClassNavigation,
				},
				KeepClasses: []control.FeishuFollowupHandoffClass{
					control.FeishuFollowupHandoffClassNavigation,
				},
			},
			wantRemain: []eventcontract.HandoffClass{
				eventcontract.HandoffClassNotice,
				eventcontract.HandoffClassThreadSelection,
				eventcontract.HandoffClassNavigation,
				eventcontract.HandoffClassProcessDetail,
				eventcontract.HandoffClassTerminalContent,
			},
		},
		{
			name: "keep_only_terminal_by_dropping_others",
			policy: control.FeishuFollowupPolicy{
				DropClasses: []control.FeishuFollowupHandoffClass{
					control.FeishuFollowupHandoffClassNotice,
					control.FeishuFollowupHandoffClassThreadSelection,
					control.FeishuFollowupHandoffClassNavigation,
					control.FeishuFollowupHandoffClassProcessDetail,
				},
				KeepClasses: []control.FeishuFollowupHandoffClass{
					control.FeishuFollowupHandoffClassTerminal,
				},
			},
			wantRemain: []eventcontract.HandoffClass{
				eventcontract.HandoffClassTerminalContent,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			filtered := FilterLegacyUIEventsByFollowupPolicy(base, tc.policy)
			got := make([]eventcontract.HandoffClass, 0, len(filtered))
			for _, event := range filtered {
				got = append(got, FromLegacyUIEvent(event).Meta.Semantics.HandoffClass)
			}
			if len(got) != len(tc.wantRemain) {
				t.Fatalf("got %d events (%v), want %d (%v)", len(got), got, len(tc.wantRemain), tc.wantRemain)
			}
			for idx := range tc.wantRemain {
				if got[idx] != tc.wantRemain[idx] {
					t.Fatalf("index %d: got class %q, want %q (full=%v)", idx, got[idx], tc.wantRemain[idx], got)
				}
			}
		})
	}
}

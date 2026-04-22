package eventcontractcompat

import (
	"testing"
	"time"

	"github.com/kxn/codex-remote-feishu/internal/core/agentproto"
	"github.com/kxn/codex-remote-feishu/internal/core/control"
	"github.com/kxn/codex-remote-feishu/internal/core/eventcontract"
	"github.com/kxn/codex-remote-feishu/internal/core/render"
)

func TestLegacyRoundTripCoversAllKinds(t *testing.T) {
	for _, kind := range eventcontract.AllKinds() {
		legacy := sampleLegacyUIEvent(kind)
		converted := FromLegacyUIEvent(legacy)
		if converted.Kind() != kind {
			t.Fatalf("kind %q converted as %q", kind, converted.Kind())
		}
		if converted.GatewayID() != "gateway-1" || converted.SurfaceSessionID() != "surface-1" {
			t.Fatalf("kind %q lost target metadata: %#v", kind, converted.Meta)
		}
		roundTrip := ToLegacyUIEvent(converted)
		if got := KindFromLegacyUIEvent(roundTrip); got != kind {
			t.Fatalf("kind %q roundtrip as %q", kind, got)
		}
	}
}

func TestLegacyKindPrefersPayloadOverMissingLegacyKind(t *testing.T) {
	legacy := control.UIEvent{
		SurfaceSessionID: "surface-1",
		Notice: &control.Notice{
			Code: "hello",
		},
	}
	converted := FromLegacyUIEvent(legacy)
	if converted.Kind() != eventcontract.KindNotice {
		t.Fatalf("converted kind = %q, want notice", converted.Kind())
	}
	if converted.Meta.Semantics.HandoffClass != eventcontract.HandoffClassNotice {
		t.Fatalf("handoff class = %q, want notice", converted.Meta.Semantics.HandoffClass)
	}
}

func TestLegacyNoticeThreadSelectionUsesExplicitHandoffClass(t *testing.T) {
	legacy := control.UIEvent{
		Kind:             control.UIEventNotice,
		SurfaceSessionID: "surface-1",
		Notice: &control.Notice{
			Code: "thread_selection_changed",
		},
		ThreadSelection: &control.ThreadSelectionChanged{ThreadID: "thread-1"},
	}
	converted := FromLegacyUIEvent(legacy)
	if converted.Meta.Semantics.HandoffClass != eventcontract.HandoffClassThreadSelection {
		t.Fatalf("handoff class = %q, want thread selection", converted.Meta.Semantics.HandoffClass)
	}
	if converted.Meta.Semantics.FirstResultDisposition != eventcontract.FirstResultDispositionDrop {
		t.Fatalf("first-result disposition = %q, want drop", converted.Meta.Semantics.FirstResultDisposition)
	}
}

func sampleLegacyUIEvent(kind eventcontract.Kind) control.UIEvent {
	base := control.UIEvent{
		GatewayID:                "gateway-1",
		SurfaceSessionID:         "surface-1",
		SourceMessageID:          "msg-1",
		SourceMessagePreview:     "preview",
		DaemonLifecycleID:        "daemon-1",
		InlineReplaceCurrentCard: true,
	}
	switch kind {
	case eventcontract.KindSnapshot:
		base.Kind = control.UIEventSnapshot
		base.Snapshot = &control.Snapshot{
			SurfaceSessionID: "surface-1",
			Attachment: control.AttachmentSummary{
				SelectedThreadID: "thread-1",
			},
		}
	case eventcontract.KindSelection:
		base.Kind = control.UIEventFeishuSelectionView
		base.FeishuSelectionView = &control.FeishuSelectionView{PromptKind: control.SelectionPromptUseThread}
		base.FeishuSelectionContext = &control.FeishuUISelectionContext{
			DTOOwner:   control.FeishuUIDTOwnerSelection,
			PromptKind: control.SelectionPromptUseThread,
			Title:      "Select",
		}
	case eventcontract.KindPage:
		base.Kind = control.UIEventFeishuPageView
		base.FeishuPageView = &control.FeishuPageView{Title: "Page"}
		base.FeishuPageContext = &control.FeishuUIPageContext{
			DTOOwner: control.FeishuUIDTOwnerPage,
			PageID:   "page-1",
			Title:    "Page",
		}
	case eventcontract.KindRequest:
		base.Kind = control.UIEventFeishuRequestView
		base.FeishuRequestView = &control.FeishuRequestView{Title: "Request"}
		base.FeishuRequestContext = &control.FeishuUIRequestContext{RequestID: "req-1"}
	case eventcontract.KindPathPicker:
		base.Kind = control.UIEventFeishuPathPicker
		base.FeishuPathPickerView = &control.FeishuPathPickerView{Title: "Path"}
		base.FeishuPathPickerContext = &control.FeishuUIPathPickerContext{PickerID: "picker-1"}
	case eventcontract.KindTargetPicker:
		base.Kind = control.UIEventFeishuTargetPicker
		base.FeishuTargetPickerView = &control.FeishuTargetPickerView{Title: "Target"}
		base.FeishuTargetPickerContext = &control.FeishuUITargetPickerContext{PickerID: "picker-2"}
	case eventcontract.KindThreadHistory:
		base.Kind = control.UIEventFeishuThreadHistory
		base.FeishuThreadHistoryView = &control.FeishuThreadHistoryView{Title: "History"}
		base.FeishuThreadHistoryContext = &control.FeishuUIThreadHistoryContext{ThreadID: "thread-1"}
	case eventcontract.KindPendingInput:
		base.Kind = control.UIEventPendingInput
		base.PendingInput = &control.PendingInputState{QueueItemID: "queue-1", TypingOn: true}
	case eventcontract.KindNotice:
		base.Kind = control.UIEventNotice
		base.Notice = &control.Notice{Code: "notice-1", Text: "hello"}
		base.ThreadSelection = &control.ThreadSelectionChanged{ThreadID: "thread-1"}
	case eventcontract.KindPlanUpdate:
		base.Kind = control.UIEventPlanUpdated
		base.PlanUpdate = &control.PlanUpdate{
			ThreadID:    "thread-1",
			Explanation: "plan",
		}
	case eventcontract.KindBlockCommitted:
		base.Kind = control.UIEventBlockCommitted
		base.Block = &render.Block{ID: "block-1", Kind: render.BlockAssistantMarkdown, Text: "final", Final: true}
		base.FileChangeSummary = &control.FileChangeSummary{FileCount: 1}
		base.TurnDiffSnapshot = &control.TurnDiffSnapshot{Diff: "@@ -1 +1 @@\n-old\n+new"}
		base.FinalTurnSummary = &control.FinalTurnSummary{Elapsed: 2 * time.Second}
	case eventcontract.KindTimelineText:
		base.Kind = control.UIEventTimelineText
		base.TimelineText = &control.TimelineText{Text: "timeline"}
	case eventcontract.KindImageOutput:
		base.Kind = control.UIEventImageOutput
		base.ImageOutput = &control.ImageOutput{SavedPath: "/tmp/out.png"}
	case eventcontract.KindExecCommandProgress:
		base.Kind = control.UIEventExecCommandProgress
		base.ExecCommandProgress = &control.ExecCommandProgress{ThreadID: "thread-1", Status: "running"}
	case eventcontract.KindAgentCommand:
		base.Kind = control.UIEventAgentCommand
		base.Command = &agentproto.Command{Kind: agentproto.CommandTurnSteer}
	case eventcontract.KindDaemonCommand:
		base.Kind = control.UIEventDaemonCommand
		base.DaemonCommand = &control.DaemonCommand{Kind: control.DaemonCommandSendIMFile}
	default:
		panic("unsupported kind")
	}
	return base
}

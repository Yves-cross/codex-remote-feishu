package feishu

import (
	"testing"

	"github.com/kxn/codex-remote-feishu/internal/core/control"
	"github.com/kxn/codex-remote-feishu/internal/core/eventcontract"
)

func TestProjectImageOutputAsImageMessage(t *testing.T) {
	projector := NewProjector()
	ops := projector.ProjectEvent("chat-1", eventcontract.Event{
		Kind:             eventcontract.KindImageOutput,
		SurfaceSessionID: "surface-1",
		SourceMessageID:  "om-source-1",
		ImageOutput: &control.ImageOutput{
			ThreadID:  "thread-1",
			TurnID:    "turn-1",
			ItemID:    "img-1",
			SavedPath: "/tmp/generated.png",
		},
	})
	if len(ops) != 1 || ops[0].Kind != OperationSendImage {
		t.Fatalf("unexpected ops: %#v", ops)
	}
	if ops[0].ReplyToMessageID != "" {
		t.Fatalf("expected image output to stay top-level, got %#v", ops[0])
	}
	if ops[0].ImagePath != "/tmp/generated.png" || ops[0].ImageBase64 != "" {
		t.Fatalf("unexpected image output operation payload: %#v", ops[0])
	}
}

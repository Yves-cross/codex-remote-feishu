package feishu

import (
	"testing"

	"github.com/kxn/codex-remote-feishu/internal/core/control"
)

func TestProjectImageOutputAsImageMessage(t *testing.T) {
	projector := NewProjector()
	ops := projector.Project("chat-1", control.UIEvent{
		Kind:             control.UIEventImageOutput,
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
	if ops[0].ReplyToMessageID != "om-source-1" {
		t.Fatalf("expected image output to reply to source message, got %#v", ops[0])
	}
	if ops[0].ImagePath != "/tmp/generated.png" || ops[0].ImageBase64 != "" {
		t.Fatalf("unexpected image output operation payload: %#v", ops[0])
	}
}

package feishu

import (
	"testing"

	"github.com/kxn/codex-remote-feishu/internal/core/control"
)

func renderedButtonCallbackValue(t *testing.T, button map[string]any) map[string]any {
	t.Helper()
	if button["tag"] != "button" {
		t.Fatalf("expected rendered V2 button, got %#v", button)
	}
	if button["value"] != nil {
		t.Fatalf("expected rendered V2 button to move callback payload into behaviors, got %#v", button)
	}
	behaviors, _ := button["behaviors"].([]map[string]any)
	if len(behaviors) != 1 || behaviors[0]["type"] != "callback" {
		t.Fatalf("expected one callback behavior, got %#v", button)
	}
	value, _ := behaviors[0]["value"].(map[string]any)
	return value
}

func renderedColumnButtons(t *testing.T, element map[string]any) []map[string]any {
	t.Helper()
	if element["tag"] != "column_set" {
		t.Fatalf("expected rendered V2 column_set, got %#v", element)
	}
	columns, _ := element["columns"].([]map[string]any)
	buttons := make([]map[string]any, 0, len(columns))
	for _, column := range columns {
		elements, _ := column["elements"].([]map[string]any)
		if len(elements) != 1 || elements[0]["tag"] != "button" {
			t.Fatalf("expected one button per V2 column, got %#v", column)
		}
		buttons = append(buttons, elements[0])
	}
	return buttons
}

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

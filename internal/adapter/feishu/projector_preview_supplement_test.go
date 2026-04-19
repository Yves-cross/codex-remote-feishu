package feishu

import "testing"

func TestProjectPreviewSupplementStaysTopLevel(t *testing.T) {
	projector := NewProjector()
	ops := projector.ProjectPreviewSupplements("app-1", "surface-1", "chat-1", "msg-source-1", []PreviewSupplement{{
		Kind: "card",
		Data: map[string]any{
			"title": "补充信息",
			"body":  "这是补充说明。",
			"theme": cardThemeInfo,
		},
	}})
	if len(ops) != 1 {
		t.Fatalf("expected one preview supplement operation, got %#v", ops)
	}
	op := ops[0]
	if op.Kind != OperationSendCard {
		t.Fatalf("expected preview supplement to render as card, got %#v", op)
	}
	if op.ReplyToMessageID != "" {
		t.Fatalf("expected preview supplement to stay top-level, got %#v", op)
	}
	if op.CardTitle != "补充信息" || op.CardBody != "这是补充说明。" {
		t.Fatalf("unexpected preview supplement payload: %#v", op)
	}
}

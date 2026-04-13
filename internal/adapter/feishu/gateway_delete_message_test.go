package feishu

import (
	"context"
	"testing"

	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

func TestApplyDeleteMessage(t *testing.T) {
	gateway := NewLiveGateway(LiveGatewayConfig{GatewayID: "app-1"})
	var deletedMessageID string
	gateway.deleteMessageFn = func(_ context.Context, messageID string) (*larkim.DeleteMessageResp, error) {
		deletedMessageID = messageID
		return &larkim.DeleteMessageResp{CodeError: larkcore.CodeError{Code: 0}}, nil
	}
	gateway.recordSurfaceMessage("om-card-1", "surface-1")

	err := gateway.Apply(t.Context(), []Operation{{
		Kind:             OperationDeleteMessage,
		GatewayID:        "app-1",
		SurfaceSessionID: "surface-1",
		MessageID:        "om-card-1",
	}})
	if err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	if deletedMessageID != "om-card-1" {
		t.Fatalf("unexpected deleted message id: %q", deletedMessageID)
	}
	if _, ok := gateway.messages["om-card-1"]; ok {
		t.Fatalf("expected recalled message mapping to be removed, got %#v", gateway.messages)
	}
}

func TestIgnoredMissingMessageDeleteError(t *testing.T) {
	tests := []struct {
		name string
		msg  string
		want bool
	}{
		{name: "english missing message", msg: "message not found", want: true},
		{name: "english recalled message", msg: "target message has been recalled", want: true},
		{name: "chinese deleted message", msg: "消息已删除", want: true},
		{name: "empty", msg: "", want: false},
	}
	for _, tt := range tests {
		if got := ignoredMissingMessageDeleteError(0, tt.msg); got != tt.want {
			t.Fatalf("%s: got %v, want %v", tt.name, got, tt.want)
		}
	}
}

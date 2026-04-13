package feishu

import (
	"testing"

	"github.com/kxn/codex-remote-feishu/internal/core/control"
	larkcallback "github.com/larksuite/oapi-sdk-go/v3/event/dispatcher/callback"
)

func TestParseCardActionTriggerEventBuildsCreateWorkspaceAction(t *testing.T) {
	gateway := NewLiveGateway(LiveGatewayConfig{GatewayID: "app-1"})
	gateway.recordSurfaceMessage("om-card-workspaces", "feishu:app-1:user:user-1")
	userID := "user-1"
	event := &larkcallback.CardActionTriggerEvent{
		Event: &larkcallback.CardActionTriggerRequest{
			Operator: &larkcallback.Operator{UserID: &userID},
			Action: &larkcallback.CallBackAction{
				Value: map[string]interface{}{
					"kind": "create_workspace",
				},
			},
			Context: &larkcallback.Context{
				OpenChatID:    "oc_1",
				OpenMessageID: "om-card-workspaces",
			},
		},
	}

	got, ok := gateway.parseCardActionTriggerEvent(event)
	if !ok {
		t.Fatal("expected create_workspace action to parse")
	}
	if got.Kind != control.ActionCreateWorkspace {
		t.Fatalf("unexpected action kind: %#v", got)
	}
	if got.GatewayID != "app-1" || got.SurfaceSessionID == "" || got.ChatID == "" || got.ActorUserID == "" || got.MessageID == "" {
		t.Fatalf("expected parsed routing metadata, got %#v", got)
	}
}

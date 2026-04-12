package daemon

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/kxn/codex-remote-feishu/internal/core/agentproto"
	"github.com/kxn/codex-remote-feishu/internal/core/control"
)

func TestDaemonRejectsOldRemovedCardAndShowsConcreteLegacyCommand(t *testing.T) {
	gateway := &recordingGateway{}
	startedAt := time.Date(2026, 4, 8, 12, 0, 0, 0, time.UTC)
	app := New(":0", ":0", gateway, agentproto.ServerIdentity{PID: 42, StartedAt: startedAt})

	seedAttachedSurfaceForInboundTests(app)

	before := len(gateway.operations)
	app.HandleAction(context.Background(), control.Action{
		Kind:             control.ActionRemovedCommand,
		SurfaceSessionID: "feishu:chat:1",
		ChatID:           "chat-1",
		ActorUserID:      "user-1",
		Text:             "resume_headless_thread",
		Inbound: &control.ActionInboundMeta{
			CardDaemonLifecycleID: "older-life",
		},
	})

	delta := gateway.operations[before:]
	assertSingleRejectedNotice(t, delta, "旧卡片已过期", "重新发送对应命令获取新卡片")
	if !strings.Contains(delta[0].CardBody, "/newinstance") {
		t.Fatalf("expected expired removed-card notice to mention /newinstance, got %#v", delta)
	}
}

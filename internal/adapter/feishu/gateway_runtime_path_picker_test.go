package feishu

import (
	"context"
	"testing"
	"time"

	"github.com/kxn/codex-remote-feishu/internal/core/control"
)

func TestHandleCardActionTriggerKeepsPathPickerConfirmAsync(t *testing.T) {
	action := control.Action{
		Kind: control.ActionPathPickerConfirm,
		Inbound: &control.ActionInboundMeta{
			CardDaemonLifecycleID: "life-1",
		},
	}
	started := make(chan struct{})
	release := make(chan struct{})
	done := make(chan struct{})
	handler := func(context.Context, control.Action) *ActionResult {
		close(started)
		<-release
		close(done)
		return &ActionResult{
			ReplaceCurrentCard: &Operation{
				Kind:         OperationSendCard,
				CardTitle:    "路径选择",
				CardBody:     "已确认",
				CardThemeKey: cardThemeInfo,
			},
		}
	}

	begin := time.Now()
	resp, err := handleCardActionTrigger(context.Background(), action, handler)
	if err != nil {
		t.Fatalf("handleCardActionTrigger returned error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected empty callback response")
	}
	if resp.Card != nil {
		t.Fatalf("expected path picker confirm callback not to replace synchronously, got %#v", resp)
	}
	if elapsed := time.Since(begin); elapsed > 100*time.Millisecond {
		t.Fatalf("expected async callback ack, took %s", elapsed)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("expected background handler to start")
	}
	close(release)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("expected background handler to finish")
	}
}

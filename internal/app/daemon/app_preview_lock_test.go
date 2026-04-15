package daemon

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/kxn/codex-remote-feishu/internal/adapter/feishu"
	"github.com/kxn/codex-remote-feishu/internal/core/agentproto"
	"github.com/kxn/codex-remote-feishu/internal/core/control"
	"github.com/kxn/codex-remote-feishu/internal/core/render"
	"github.com/kxn/codex-remote-feishu/internal/core/state"
)

type reentrantAppLockPreviewer struct {
	app *App

	mu       sync.Mutex
	requests []feishu.FinalBlockPreviewRequest
}

func (s *reentrantAppLockPreviewer) RewriteFinalBlock(_ context.Context, req feishu.FinalBlockPreviewRequest) (feishu.FinalBlockPreviewResult, error) {
	s.mu.Lock()
	s.requests = append(s.requests, req)
	s.mu.Unlock()

	locked := make(chan struct{})
	go func() {
		s.app.mu.Lock()
		s.app.mu.Unlock()
		close(locked)
	}()

	select {
	case <-locked:
		return feishu.FinalBlockPreviewResult{Block: req.Block}, nil
	case <-time.After(250 * time.Millisecond):
		return feishu.FinalBlockPreviewResult{Block: req.Block}, errors.New("previewer could not reacquire app lock")
	}
}

func (s *reentrantAppLockPreviewer) snapshot() []feishu.FinalBlockPreviewRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]feishu.FinalBlockPreviewRequest(nil), s.requests...)
}

func TestHandleUIEventsReleasesAppLockDuringFinalPreviewRewrite(t *testing.T) {
	gateway := &recordingGateway{}
	app := New(":0", ":0", gateway, agentproto.ServerIdentity{})
	previewer := &reentrantAppLockPreviewer{app: app}
	app.SetFinalBlockPreviewer(previewer)

	app.service.MaterializeSurface("feishu:chat:1", "app-1", "chat-1", "ou_user")
	app.service.UpsertInstance(&state.InstanceRecord{
		InstanceID:    "inst-1",
		DisplayName:   "droid",
		WorkspaceRoot: "/data/dl/droid",
		WorkspaceKey:  "/data/dl/droid",
		ShortName:     "droid",
		Online:        true,
		Threads: map[string]*state.ThreadRecord{
			"thread-1": {
				ThreadID: "thread-1",
				CWD:      "/data/dl/droid",
				Loaded:   true,
			},
		},
	})

	done := make(chan struct{})
	go func() {
		app.mu.Lock()
		defer app.mu.Unlock()
		app.handleUIEvents(context.Background(), []control.UIEvent{{
			Kind:             control.UIEventBlockCommitted,
			SurfaceSessionID: "feishu:chat:1",
			SourceMessageID:  "msg-1",
			Block: &render.Block{
				Kind:       render.BlockAssistantMarkdown,
				InstanceID: "inst-1",
				ThreadID:   "thread-1",
				TurnID:     "turn-1",
				ItemID:     "item-1",
				Text:       "最终结果",
				Final:      true,
			},
		}})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("handleUIEvents timed out while previewer reentered app lock")
	}

	if requests := previewer.snapshot(); len(requests) != 1 {
		t.Fatalf("expected one preview rewrite request, got %#v", requests)
	}
	if len(gateway.operations) == 0 {
		t.Fatalf("expected final reply to be delivered after preview rewrite")
	}
}

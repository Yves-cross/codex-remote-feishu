package wrapper

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/kxn/codex-remote-feishu/internal/adapter/codex"
)

func TestRestoreChildSessionContextClearsPendingStateOnCancel(t *testing.T) {
	app := &App{
		translator: codex.NewTranslator("inst-1"),
	}
	if _, err := app.translator.ObserveClient([]byte(`{"method":"thread/resume","params":{"threadId":"thread-1","cwd":"/tmp/project"}}`)); err != nil {
		t.Fatalf("seed thread/resume: %v", err)
	}

	writeCh := make(chan []byte, 1)
	commandResponses := newCommandResponseTracker()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- app.restoreChildSessionContext(ctx, writeCh, commandResponses)
	}()

	select {
	case frame := <-writeCh:
		if !strings.Contains(string(frame), `"method":"thread/resume"`) {
			t.Fatalf("expected restore frame, got %q", string(frame))
		}
		var payload map[string]any
		if err := json.Unmarshal(frame, &payload); err != nil {
			t.Fatalf("unmarshal restore frame: %v", err)
		}
		requestID, _ := payload["id"].(string)
		if strings.TrimSpace(requestID) == "" {
			t.Fatalf("expected restore request id in %q", string(frame))
		}
		defer func() {
			result, err := app.translator.ObserveServer([]byte(`{"id":"` + requestID + `","result":{}}`))
			if err != nil {
				t.Fatalf("observe late restore response: %v", err)
			}
			if result.Suppress {
				t.Fatalf("expected canceled restore response not to be suppressed, got %#v", result)
			}
		}()
		cancel()
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for restore frame")
	}

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected restore cancellation error")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for restore cancellation")
	}

	if len(commandResponses.pending) != 0 {
		t.Fatalf("expected command response tracker to be cleared, got %#v", commandResponses.pending)
	}
}

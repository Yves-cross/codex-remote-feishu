package wrapper

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/kxn/codex-remote-feishu/internal/adapter/codex"
)

type blockingReader struct {
	done chan struct{}
	err  error
}

func newBlockingReader(err error) *blockingReader {
	return &blockingReader{
		done: make(chan struct{}),
		err:  err,
	}
}

func (r *blockingReader) Read(_ []byte) (int, error) {
	<-r.done
	return 0, r.err
}

func (r *blockingReader) Close() {
	select {
	case <-r.done:
	default:
		close(r.done)
	}
}

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

func TestStdoutLoopIgnoresClosedPipeAfterSessionCancel(t *testing.T) {
	reader := newBlockingReader(errors.New("read |0: file already closed"))
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	done := make(chan struct{})

	go func() {
		stdoutLoop(ctx, reader, io.Discard, make(chan []byte, 1), codex.NewTranslator("inst-1"), nil, newCommandResponseTracker(), errCh, nil, nil, nil)
		close(done)
	}()

	cancel()
	reader.Close()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for stdoutLoop to exit")
	}

	select {
	case err := <-errCh:
		t.Fatalf("expected canceled session to suppress closed-pipe error, got %v", err)
	default:
	}
}

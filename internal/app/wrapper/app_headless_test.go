package wrapper

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"
)

func TestBootstrapHeadlessCodexCompletesInitializeHandshake(t *testing.T) {
	app := New(Config{
		Source:  "headless",
		Version: "test",
	})

	bufferedLine := mustJSONLine(t, map[string]any{
		"method": "thread/started",
		"params": map[string]any{
			"thread": map[string]any{
				"id": "thread-buffered",
			},
		},
	})
	initializeResponse := mustJSONLine(t, map[string]any{
		"id": relayBootstrapInitializeID,
		"result": map[string]any{
			"userAgent": "mockcodex/0.0.1",
		},
	})

	var childStdin bytes.Buffer
	replayedStdout, err := app.bootstrapHeadlessCodex(&childStdin, strings.NewReader(bufferedLine+initializeResponse), nil, nil)
	if err != nil {
		t.Fatalf("bootstrap headless codex: %v", err)
	}

	frames := decodeJSONLines(t, childStdin.String())
	if len(frames) != 2 {
		t.Fatalf("expected 2 bootstrap frames, got %d: %s", len(frames), childStdin.String())
	}
	if got := lookupStringFromMap(frames[0], "method"); got != "initialize" {
		t.Fatalf("expected first frame to be initialize, got %q", got)
	}
	if got := lookupStringFromMap(frames[0], "id"); got != relayBootstrapInitializeID {
		t.Fatalf("expected initialize id %q, got %q", relayBootstrapInitializeID, got)
	}
	if got := lookupStringFromMap(frames[1], "method"); got != "initialized" {
		t.Fatalf("expected second frame to be initialized, got %q", got)
	}

	remaining, err := io.ReadAll(replayedStdout)
	if err != nil {
		t.Fatalf("read replayed stdout: %v", err)
	}
	if string(remaining) != bufferedLine {
		t.Fatalf("expected buffered stdout to be replayed, got %q", string(remaining))
	}
}

func TestBootstrapHeadlessCodexFailsWhenInitializeRejected(t *testing.T) {
	app := New(Config{
		Source:  "headless",
		Version: "test",
	})

	var childStdin bytes.Buffer
	_, err := app.bootstrapHeadlessCodex(&childStdin, strings.NewReader(mustJSONLine(t, map[string]any{
		"id": relayBootstrapInitializeID,
		"error": map[string]any{
			"message": "Not initialized",
		},
	})), nil, nil)
	if err == nil {
		t.Fatal("expected bootstrap to fail when initialize is rejected")
	}
	if !strings.Contains(err.Error(), "Not initialized") {
		t.Fatalf("expected initialize rejection in error, got %v", err)
	}
}

func decodeJSONLines(t *testing.T, raw string) []map[string]any {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(raw), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil
	}
	frames := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var frame map[string]any
		if err := json.Unmarshal([]byte(line), &frame); err != nil {
			t.Fatalf("unmarshal json line %q: %v", line, err)
		}
		frames = append(frames, frame)
	}
	return frames
}

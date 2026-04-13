//go:build devtrace

package conversationtrace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDevTraceBuildWritesNDJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "trace.ndjson")
	logger, err := Open(path)
	if err != nil {
		t.Fatalf("open dev logger: %v", err)
	}
	if logger == nil {
		t.Fatal("expected devtrace logger to be enabled")
	}
	t.Cleanup(func() {
		_ = logger.Close()
	})
	if !Enabled() {
		t.Fatalf("expected Enabled() to be true in devtrace build")
	}
	logger.Log(Entry{Event: EventUserMessage, Text: "hello"})
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read trace file: %v", err)
	}
	content := strings.TrimSpace(string(raw))
	if !strings.Contains(content, "\"event\":\"user_message\"") || !strings.Contains(content, "\"text\":\"hello\"") {
		t.Fatalf("unexpected trace content: %s", content)
	}
}

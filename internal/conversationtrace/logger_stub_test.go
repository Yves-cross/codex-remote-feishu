//go:build !devtrace

package conversationtrace

import (
	"path/filepath"
	"testing"
)

func TestStubBuildKeepsConversationTraceDisabled(t *testing.T) {
	logger, err := Open(filepath.Join(t.TempDir(), "trace.ndjson"))
	if err != nil {
		t.Fatalf("open stub logger: %v", err)
	}
	if logger != nil {
		t.Fatalf("expected no logger in non-devtrace build, got %#v", logger)
	}
	if Enabled() {
		t.Fatalf("expected Enabled() to be false in non-devtrace build")
	}
}
